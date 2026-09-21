package wrapper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/milaboratory/small-binaries/job-wrapper/internal/cgroup"
	"github.com/milaboratory/small-binaries/job-wrapper/internal/execid"
	"github.com/milaboratory/small-binaries/job-wrapper/internal/prune"
	"github.com/milaboratory/small-binaries/job-wrapper/internal/report"
	"github.com/milaboratory/small-binaries/job-wrapper/internal/shellwords"
)

// Name is what the report calls this program.
const Name = "job-wrapper"

// legacyPrefix marks the messages job-script.sh used to write; they are kept byte for byte.
// ownPrefix marks messages that are new with the wrapper.
const (
	legacyPrefix = prune.LogPrefix
	ownPrefix    = "[job-wrapper]"
)

// Exit codes the wrapper produces on its own, following sh conventions.
const (
	exitStartFailure = 126
	exitNotFound     = 127
	exitOOMKilled    = 137
)

// Command modes, see execid.Exec.Mode.
const (
	modeArgv  = "argv"
	modeShell = "shell"
)

type waitResult struct {
	exitCode int
	signal   string
	err      error
}

// run is the state of one job execution.
type run struct {
	cfg     Config
	realOut io.Writer
	realErr io.Writer
	streams *streams
	argv    []string
	sampler *report.Sampler
	pub     *publisher
	limits  cgroup.Limits
	rep     *report.Report
}

// Run executes the job described by cfg and returns the exit code the wrapper must exit with.
func Run(cfg Config) int {
	r := &run{cfg: cfg, realOut: cfg.Stdout, realErr: cfg.Stderr}
	if r.realOut == nil {
		r.realOut = os.Stdout
	}
	if r.realErr == nil {
		r.realErr = os.Stderr
	}

	applyPathEnv(cfg)
	r.prepareWorkdir()

	st, err := openStreams(cfg, r.realOut, r.realErr)
	if err != nil {
		r.warnf("failed to set up output redirection: %v", err)
		return 1
	}
	r.streams = st

	r.resolveCommand()
	r.setupMetrics()

	startedAt := time.Now()
	r.rep.StartedAt = startedAt.UnixMilli()
	r.sampler.Start(startedAt)
	r.pub.flush()

	res := r.startAndSupervise()
	finishedAt := time.Now()
	r.sampler.Finish(finishedAt)

	// Completion marker: written for every exit code, absent only when the wrapper was killed.
	if cfg.MarkerPath != "" {
		err = writeMarker(cfg.MarkerPath, res.exitCode)
		if err != nil {
			r.warnf("failed to write completion marker %s: %v", cfg.MarkerPath, err)
		}
	}

	// Final report before draining the streams: a grandchild holding the pipe must not delay it.
	r.pub.finish(res, finishedAt, startedAt)
	r.reportOutcome(res)

	// Wait for the tees.
	r.streams.drain()

	return res.exitCode
}

// warnf writes a wrapper-own diagnostic to the real stderr.
func (r *run) warnf(format string, a ...any) {
	_, _ = fmt.Fprintf(r.realErr, ownPrefix+" "+format+"\n", a...)
}

// prepareWorkdir prunes leftovers of an earlier attempt and hands the command an empty temporary
// directory, the way job-script.sh did.
func (r *run) prepareWorkdir() {
	cfg := r.cfg
	if cfg.ExpectedItemsFile != "" && cfg.Workdir != "" && isDir(cfg.Workdir) {
		items, ok := loadItems(cfg.ExpectedItemsFile)
		if ok {
			err := prune.Run(cfg.Workdir, items, TmpDirRel, func(msg string) {
				_, _ = fmt.Fprintf(r.realErr, "%s %s\n", legacyPrefix, msg)
			})
			if err != nil {
				r.warnf("workdir prune failed: %v", err)
			}
		}
	}
	if cfg.Workdir != "" && isDir(filepath.Join(cfg.Workdir, TmpDirRel)) {
		prune.EmptyDir(cfg.Workdir, TmpDirRel)
	}
}

// resolveCommand settles the argv the report identifies the command by.
func (r *run) resolveCommand() {
	r.argv = r.cfg.Argv
	mode := modeArgv
	if len(r.argv) == 0 {
		mode = modeShell
		words, err := shellwords.Split(r.cfg.ShellCommand)
		if err == nil && len(words) > 0 {
			r.argv = words
		} else {
			r.argv = []string{"sh", r.cfg.ShellCommand}
		}
	}
	ex, _ := execid.Derive(r.argv)
	ex.Mode = mode
	r.rep = &report.Report{
		Version: report.Version,
		Wrapper: report.Wrapper{Name: Name, Version: r.cfg.Version},
		State:   report.StateRunning,
		ExecID:  ex.ID(),
		Exec:    &ex,
	}
	if r.cfg.ExecIDOverride != "" {
		r.rep.ExecID = r.cfg.ExecIDOverride
	}
}

// setupMetrics discovers the cgroup and builds the sampler and the publisher.
func (r *run) setupMetrics() {
	var src cgroup.Source
	if r.cfg.ReportPath != "" {
		found, err := cgroup.Discover(cgroup.DiscoverOptions{Dir: r.cfg.CgroupDir})
		if err != nil {
			r.rep.Cgroup = cgroup.Info{Error: err.Error()}
		} else {
			src = found
			r.rep.Cgroup = src.Info()
			r.limits = src.Limits()
			r.rep.Granted = r.limits
		}
	}
	r.sampler = report.NewSampler(src, r.cfg.SampleInterval, r.cfg.MaxSamples)
	r.pub = newPublisher(r.cfg.ReportPath, r.rep, r.sampler, r.warnf)
}

// startAndSupervise starts the command and runs the sampling loop until it exits. A command that
// cannot start is reported the way sh reports it: 127 when not found, 126 otherwise.
func (r *run) startAndSupervise() waitResult {
	// The command's lifetime is the wrapper's: signals are forwarded explicitly, nothing cancels it.
	ctx := context.Background()
	var cmd *exec.Cmd
	if len(r.cfg.Argv) > 0 {
		//nolint:gosec // running the job's command is the point
		cmd = exec.CommandContext(ctx, r.cfg.Argv[0], r.cfg.Argv[1:]...)
	} else {
		//nolint:gosec // the legacy contract: `sh -c "$PL_JOB_CMD_AND_ARGS"`
		cmd = exec.CommandContext(ctx, "sh", "-c", r.cfg.ShellCommand)
	}
	cmd.Stdout = r.streams.childStdout
	cmd.Stderr = r.streams.childStderr
	cmd.Stdin = os.Stdin
	setProcAttr(cmd)

	err := cmd.Start()
	r.streams.started()
	if err != nil {
		res := waitResult{exitCode: exitStartFailure, err: err}
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			res.exitCode = exitNotFound
		}
		_, _ = fmt.Fprintf(r.streams.wrapperStderr, "%s %s: %v\n", legacyPrefix, r.argv[0], err)
		return res
	}
	return r.supervise(cmd)
}

// supervise runs the sampling and flushing loop until the command exits, forwarding SIGTERM and
// SIGINT to it on the way.
func (r *run) supervise(cmd *exec.Cmd) waitResult {
	done := make(chan waitResult, 1)
	go func() { done <- waitChild(cmd) }()

	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	sampleTick := time.NewTicker(r.cfg.SampleInterval)
	defer sampleTick.Stop()
	flushEvery := r.cfg.FlushInterval
	if flushEvery <= 0 {
		flushEvery = DefaultFlushInterval
	}
	flushTick := time.NewTicker(flushEvery)
	defer flushTick.Stop()

	highWaterFlushed := false
	for {
		select {
		case res := <-done:
			return res

		case now := <-sampleTick.C:
			mem := r.sampler.Tick(now)
			// Close to the limit the kernel may take the whole cgroup down, wrapper included; get the
			// peak on disk now rather than at the next scheduled flush. Once per run, then back to
			// the regular cadence.
			if mem != nil && !highWaterFlushed && r.nearMemoryLimit(mem.Current) {
				highWaterFlushed = true
				r.pub.flush()
				flushTick.Reset(flushEvery)
			}

		case <-flushTick.C:
			r.pub.flush()

		case sig := <-sigs:
			r.rep.TerminationSignal = signalName(sig)
			forwardSignal(cmd, sig)
			r.pub.flush()
		}
	}
}

// nearMemoryLimit reports whether current is at 95 % of the granted memory or above.
func (r *run) nearMemoryLimit(current uint64) bool {
	return r.limits.MemoryBytes > 0 && current >= r.limits.MemoryBytes/100*95
}

// reportOutcome writes the exit messages to the command's stderr sink, as the script did after
// its own stderr had been redirected.
func (r *run) reportOutcome(res waitResult) {
	if res.exitCode == 0 {
		return
	}
	w := r.streams.wrapperStderr
	_, _ = fmt.Fprintf(w, "%s Process exited with code %d\n", legacyPrefix, res.exitCode)
	if res.exitCode == exitOOMKilled {
		_, _ = fmt.Fprintf(
			w,
			"%s The process was killed (likely out of memory). Consider running this job with more memory.\n",
			legacyPrefix,
		)
	}
	if r.rep.OOMKilled {
		_, _ = fmt.Fprintf(
			w,
			"%s The kernel OOM killer took %d process(es) from this job's cgroup (peak RAM %d bytes, limit %d bytes).\n",
			ownPrefix,
			r.rep.MemoryEvents.OOMKill+r.rep.MemoryEvents.OOMGroupKill,
			r.rep.RAM.Peak,
			r.limits.MemoryBytes,
		)
	}
	if res.err != nil {
		_, _ = fmt.Fprintf(w, "%s %v\n", ownPrefix, res.err)
	}
}

// publisher owns the report file.
type publisher struct {
	path    string
	rep     *report.Report
	sampler *report.Sampler
	warn    func(string, ...any)
	warned  bool
}

func newPublisher(path string, rep *report.Report, sampler *report.Sampler, warn func(string, ...any)) *publisher {
	return &publisher{path: path, rep: rep, sampler: sampler, warn: warn}
}

func (p *publisher) flush() {
	if p.path == "" {
		return
	}
	p.sampler.Fill(p.rep)
	p.rep.UpdatedAt = time.Now().UnixMilli()
	err := report.WriteAtomic(p.path, p.rep)
	if err != nil && !p.warned {
		p.warned = true
		p.warn("failed to write usage report %s: %v", p.path, err)
	}
}

func (p *publisher) finish(res waitResult, finishedAt, startedAt time.Time) {
	if p.path == "" {
		return
	}
	code := res.exitCode
	fin := finishedAt.UnixMilli()
	dur := finishedAt.Sub(startedAt).Seconds()
	p.rep.State = report.StateFinished
	p.rep.ExitCode = &code
	p.rep.Signal = res.signal
	p.rep.FinishedAt = &fin
	p.rep.Duration = &dur
	p.flush()
}

// applyPathEnv reproduces the PATH / LD_LIBRARY_PATH handling of job-script.sh on the wrapper's
// own environment, which the command inherits and which `sh` is looked up through.
func applyPathEnv(cfg Config) {
	sep := string(os.PathListSeparator)
	prepend := func(name, value string) {
		if value == "" {
			return
		}
		cur := os.Getenv(name)
		if cur == "" {
			_ = os.Setenv(name, value)
			return
		}
		_ = os.Setenv(name, value+sep+cur)
	}
	prepend("PATH", cfg.GPUBinPath)
	prepend("LD_LIBRARY_PATH", cfg.GPULibPath)
	prepend("PATH", cfg.JobPath)
}

// loadItems reads the expected-items list. A missing or empty file means no pruning: pruning
// against nothing would wipe the working directory.
func loadItems(path string) (prune.Items, bool) {
	f, err := os.Open(path)
	if err != nil {
		return prune.Items{}, false
	}
	defer func() { _ = f.Close() }()
	items, err := prune.ParseItems(f)
	if err != nil || items.Len() == 0 {
		return prune.Items{}, false
	}
	return items, true
}

// writeMarker writes the exit code where the runner expects it: 0755 / 0644 so the runner can read
// it under another uid.
func writeMarker(path string, exitCode int) error {
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(exitCode)+"\n"), 0o644)
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func itoa(i int) string { return strconv.Itoa(i) }
