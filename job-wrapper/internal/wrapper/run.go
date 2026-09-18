package wrapper

import (
	"errors"
	"fmt"
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
)

type waitResult struct {
	exitCode int
	signal   string
	err      error
}

// Run executes the job described by cfg and returns the exit code the wrapper must exit with.
func Run(cfg Config) int {
	realOut, realErr := cfg.Stdout, cfg.Stderr
	if realOut == nil {
		realOut = os.Stdout
	}
	if realErr == nil {
		realErr = os.Stderr
	}
	warn := func(format string, a ...any) {
		fmt.Fprintf(realErr, ownPrefix+" "+format+"\n", a...)
	}

	// --- Environment: GPU paths first, then the runenv PATH so it wins over them. ---
	applyPathEnv(cfg)

	// --- Prune leftovers of an earlier attempt, then hand the command an empty temporary dir. ---
	if cfg.ExpectedItemsFile != "" && cfg.Workdir != "" && isDir(cfg.Workdir) {
		if items, ok := loadItems(cfg.ExpectedItemsFile); ok {
			err := prune.Run(cfg.Workdir, items, TmpDirRel, func(msg string) {
				fmt.Fprintf(realErr, "%s %s\n", legacyPrefix, msg)
			})
			if err != nil {
				warn("workdir prune failed: %v", err)
			}
		}
	}
	if cfg.Workdir != "" && isDir(filepath.Join(cfg.Workdir, TmpDirRel)) {
		prune.EmptyDir(cfg.Workdir, TmpDirRel)
	}

	// --- Streams. ---
	st, err := openStreams(cfg, realOut, realErr)
	if err != nil {
		warn("failed to set up output redirection: %v", err)
		return 1
	}

	// --- The command and its identity. ---
	argv := cfg.Argv
	mode := "argv"
	if len(argv) == 0 {
		mode = "shell"
		if words, err := shellwords.Split(cfg.ShellCommand); err == nil && len(words) > 0 {
			argv = words
		} else {
			argv = []string{"sh", cfg.ShellCommand}
		}
	}
	ex, _ := execid.Derive(argv)
	ex.Mode = mode

	// --- Metrics. ---
	var src cgroup.Source
	info := cgroup.Info{}
	if cfg.ReportPath != "" {
		src, err = cgroup.Discover(cgroup.DiscoverOptions{Dir: cfg.CgroupDir})
		if err != nil {
			info.Error = err.Error()
			src = nil
		} else {
			info = src.Info()
		}
	}
	sampler := report.NewSampler(src, cfg.SampleInterval, cfg.MaxSamples)
	rep := &report.Report{
		Version: report.Version,
		Wrapper: report.Wrapper{Name: Name, Version: cfg.Version},
		State:   report.StateRunning,
		ExecID:  ex.ID(),
		Exec:    &ex,
		Cgroup:  info,
	}
	if cfg.ExecIDOverride != "" {
		rep.ExecID = cfg.ExecIDOverride
	}
	var limits cgroup.Limits
	if src != nil {
		limits = src.Limits()
		rep.Granted = limits
	}
	pub := newPublisher(cfg.ReportPath, rep, sampler, warn)

	startedAt := time.Now()
	rep.StartedAt = startedAt.UnixMilli()
	sampler.Start(startedAt)
	pub.flush()

	// --- Start. ---
	var cmd *exec.Cmd
	if len(cfg.Argv) > 0 {
		cmd = exec.Command(cfg.Argv[0], cfg.Argv[1:]...)
	} else {
		cmd = exec.Command("sh", "-c", cfg.ShellCommand)
	}
	cmd.Stdout = st.childStdout
	cmd.Stderr = st.childStderr
	cmd.Stdin = os.Stdin
	setProcAttr(cmd)

	var res waitResult
	if err := cmd.Start(); err != nil {
		st.started()
		res = waitResult{exitCode: exitStartFailure, err: err}
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			res.exitCode = exitNotFound
		}
		fmt.Fprintf(st.wrapperStderr, "%s %s: %v\n", legacyPrefix, argv[0], err)
	} else {
		st.started()
		res = supervise(cmd, cfg, sampler, pub, limits, rep)
	}
	finishedAt := time.Now()
	sampler.Finish(finishedAt)

	// --- Completion marker: written for every exit code, absent only when the wrapper was killed. ---
	if cfg.MarkerPath != "" {
		if err := writeMarker(cfg.MarkerPath, res.exitCode); err != nil {
			warn("failed to write completion marker %s: %v", cfg.MarkerPath, err)
		}
	}

	// --- Final report, before draining the streams: a grandchild holding the pipe must not delay it. ---
	pub.finish(res, finishedAt, startedAt)

	// --- Report the outcome to the (possibly redirected) stderr. ---
	if res.exitCode != 0 {
		fmt.Fprintf(st.wrapperStderr, "%s Process exited with code %d\n", legacyPrefix, res.exitCode)
		if res.exitCode == 137 {
			fmt.Fprintf(st.wrapperStderr, "%s The process was killed (likely out of memory). Consider running this job with more memory.\n", legacyPrefix)
		}
		if rep.OOMKilled {
			fmt.Fprintf(st.wrapperStderr, "%s The kernel OOM killer took %d process(es) from this job's cgroup (peak RAM %d bytes, limit %d bytes).\n",
				ownPrefix, rep.MemoryEvents.OOMKill+rep.MemoryEvents.OOMGroupKill, rep.RAM.Peak, limits.MemoryBytes)
		}
		if res.err != nil {
			fmt.Fprintf(st.wrapperStderr, "%s %v\n", ownPrefix, res.err)
		}
	}

	// --- Wait for the tees. ---
	st.drain()

	return res.exitCode
}

// supervise runs the sampling and flushing loop until the command exits, forwarding SIGTERM and
// SIGINT to it on the way.
func supervise(cmd *exec.Cmd, cfg Config, sampler *report.Sampler, pub *publisher, limits cgroup.Limits, rep *report.Report) waitResult {
	done := make(chan waitResult, 1)
	go func() { done <- waitChild(cmd) }()

	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	sampleTick := time.NewTicker(cfg.SampleInterval)
	defer sampleTick.Stop()
	flushEvery := cfg.FlushInterval
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
			mem := sampler.Tick(now)
			// Close to the limit the kernel may take the whole cgroup down, wrapper included; get the
			// peak on disk now rather than at the next scheduled flush. Once per run, then back to
			// the regular cadence.
			if mem != nil && !highWaterFlushed && limits.MemoryBytes > 0 && mem.Current >= limits.MemoryBytes/100*95 {
				highWaterFlushed = true
				pub.flush()
				flushTick.Reset(flushEvery)
			}

		case <-flushTick.C:
			pub.flush()

		case sig := <-sigs:
			rep.TerminationSignal = signalName(sig)
			forwardSignal(cmd, sig)
			pub.flush()
		}
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
	if err := report.WriteAtomic(p.path, p.rep); err != nil && !p.warned {
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
	defer f.Close()
	items, err := prune.ParseItems(f)
	if err != nil || items.Len() == 0 {
		return prune.Items{}, false
	}
	return items, true
}

func writeMarker(path string, exitCode int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(exitCode)+"\n"), 0o644)
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func itoa(i int) string { return strconv.Itoa(i) }
