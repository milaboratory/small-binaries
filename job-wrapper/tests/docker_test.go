package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/milaboratory/small-binaries/job-wrapper/internal/report"
)

// The Docker suite runs the Linux wrapper as PID 1 of a busybox container with a memory and CPU
// limit, which is the shape of a Kubernetes job pod, and reads the report back from a bind-mounted
// working directory. Enabled with JOB_WRAPPER_DOCKER=1 (test.sh sets it); skipped otherwise.

const dockerImage = "busybox:1.36"

const (
	mib      = 1 << 20
	memLimit = 512 * mib
)

func dockerEnabled(t *testing.T) {
	t.Helper()
	if os.Getenv("JOB_WRAPPER_DOCKER") == "" {
		t.Skip("set JOB_WRAPPER_DOCKER=1 to run the Docker suite")
	}
}

var (
	linuxOnce sync.Once
	linuxDir  string
	errLinux  error
)

// linuxBinaries cross-compiles job-wrapper and memhog for the Docker daemon's architecture.
func linuxBinaries(t *testing.T) string {
	t.Helper()
	linuxOnce.Do(func() {
		out, err := exec.CommandContext(t.Context(), "docker", "info", "--format", "{{.Architecture}}").Output()
		if err != nil {
			errLinux = fmt.Errorf("docker info: %w", err)
			return
		}
		arch := strings.TrimSpace(string(out))
		switch arch {
		case "x86_64", "amd64":
			arch = "amd64"
		case "aarch64", "arm64":
			arch = "arm64"
		default:
			errLinux = fmt.Errorf("unsupported docker architecture %q", arch)
			return
		}
		linuxDir = filepath.Join(binDir, "linux-"+arch)
		errLinux = os.MkdirAll(linuxDir, 0o755)
		if errLinux != nil {
			return
		}
		for name, pkg := range map[string]string{"job-wrapper": "./cmd/job-wrapper", "memhog": "./tests/memhog"} {
			p := filepath.Join(linuxDir, name)
			err := goBuild(p, pkg, "linux", arch)
			if err != nil {
				errLinux = fmt.Errorf("building %s: %w", name, err)
				return
			}
			_ = os.Chmod(p, 0o755)
		}
		_ = exec.CommandContext(t.Context(), "docker", "pull", "-q", dockerImage).Run()
	})
	if errLinux != nil {
		t.Fatal(errLinux)
	}
	return linuxDir
}

// workDir is a host directory the container (uid 1010) can write to.
func workDir(t *testing.T) string {
	t.Helper()
	// Not t.TempDir(): files the container writes as uid 1010 can defeat its RemoveAll on Linux and
	// fail the test in cleanup; this directory has a container-side fallback below.
	dir, err := os.MkdirTemp(os.Getenv("JOB_WRAPPER_TEST_TMPDIR"), "jw-work-") //nolint:usetesting // see above
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(dir, 0o777)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := os.RemoveAll(dir)
		if err == nil {
			return
		}
		// Files created by uid 1010 inside a directory the host user cannot write: let a container
		// clean up. t.Context() is already cancelled in Cleanup, so this gets its own context.
		cleanup := exec.CommandContext(
			context.Background(),
			"docker", "run", "--rm", "-v", dir+":/work", dockerImage, "sh", "-c", "rm -rf /work/.pl /work/*",
		)
		_ = cleanup.Run()
		_ = os.RemoveAll(dir)
	})
	return dir
}

type container struct {
	name string
	work string
}

// dockerArgs builds the common `docker run` argument list.
func dockerArgs(bins, work string, extra []string, env map[string]string, wrapperArgs ...string) []string {
	args := []string{"run", "--user", "1010:1010",
		"-v", bins + ":/opt/jw:ro", "-v", work + ":/work", "-w", "/work",
		"-e", "PL_JOB_WORKDIR=/work"}
	args = append(args, extra...)
	for k, v := range env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, dockerImage, "/opt/jw/job-wrapper")
	return append(args, wrapperArgs...)
}

func dockerRun(t *testing.T, bins, work string, extra []string, env map[string]string, wrapperArgs ...string) result {
	t.Helper()
	args := dockerArgs(bins, work, append([]string{"--rm"}, extra...), env, wrapperArgs...)
	cmd := exec.CommandContext(t.Context(), "docker", args...)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("docker run: %v", err)
		}
		code = exitErr.ExitCode()
	}
	t.Logf("docker run exit=%d\n--- stdout\n%s--- stderr\n%s", code, out.String(), errb.String())
	return result{stdout: out.String(), stderr: errb.String(), exitCode: code}
}

func dockerStart(
	t *testing.T,
	bins, work string,
	extra []string,
	env map[string]string,
	wrapperArgs ...string,
) container {
	t.Helper()
	name := fmt.Sprintf("jw-test-%d-%d", time.Now().UnixNano(), rand.Intn(1<<16))
	args := dockerArgs(bins, work, append([]string{"-d", "--name", name}, extra...), env, wrapperArgs...)
	out, err := exec.CommandContext(t.Context(), "docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker run -d: %v: %s", err, out)
	}
	t.Cleanup(func() {
		// t.Context() is already cancelled in Cleanup.
		_ = exec.CommandContext(context.Background(), "docker", "rm", "-f", name).Run()
	})
	return container{name: name, work: work}
}

func (c container) wait(t *testing.T) int {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "docker", "wait", c.name).Output()
	if err != nil {
		t.Fatalf("docker wait: %v", err)
	}
	code, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("docker wait output %q: %v", out, err)
	}
	logs, _ := exec.CommandContext(t.Context(), "docker", "logs", c.name).CombinedOutput()
	t.Logf("container %s exit=%d logs:\n%s", c.name, code, logs)
	return code
}

func readReport(t *testing.T, work string) *report.Report {
	t.Helper()
	rep, err := report.Read(filepath.Join(work, serviceReportFile))
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		t.Fatalf("marshalling report: %v", err)
	}
	t.Logf("report:\n%s", b)
	return rep
}

func tryReadReport(work string) (*report.Report, bool) {
	rep, err := report.Read(filepath.Join(work, serviceReportFile))
	if err != nil {
		return nil, false
	}
	return rep, true
}

// --- Tests ---

func TestDockerCollectsCPUAndRAM(t *testing.T) {
	dockerEnabled(t)
	bins := linuxBinaries(t)
	work := workDir(t)

	r := dockerRun(t, bins, work,
		[]string{"--memory", "512m", "--memory-swap", "512m", "--cpus", "1"},
		map[string]string{"PL_JOB_COMPLETION_MARKER_PATH": "/work/" + serviceMarkerFile},
		"--flush-interval", "1s", "--", "/opt/jw/memhog", "--mb", "128", "--hold", "3s", "--spin")
	if r.exitCode != 0 {
		t.Fatalf("exit %d", r.exitCode)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(work, serviceMarkerFile))); got != "0" {
		t.Errorf("marker = %q", got)
	}

	rep := readReport(t, work)
	if rep.State != report.StateFinished || rep.ExitCode == nil || *rep.ExitCode != 0 || rep.Signal != "" {
		t.Errorf("outcome: state=%s exit=%v signal=%q", rep.State, rep.ExitCode, rep.Signal)
	}
	if !rep.Cgroup.Available || (rep.Cgroup.Version != 1 && rep.Cgroup.Version != 2) {
		t.Fatalf("cgroup must be available in the container: %+v", rep.Cgroup)
	}
	if rep.Granted.MemoryBytes != memLimit {
		t.Errorf("granted.ram = %d, want %d (the --memory limit)", rep.Granted.MemoryBytes, memLimit)
	}
	if rep.Granted.CPUMillicores != 1000 {
		t.Errorf("granted.cpu = %d, want 1000 (the --cpus limit)", rep.Granted.CPUMillicores)
	}
	if rep.RAM.Peak < 128*mib || rep.RAM.Peak > memLimit {
		t.Errorf("ram.peak = %d, want between the 128 MiB the workload touched and the 512 MiB limit", rep.RAM.Peak)
	}
	if rep.RAM.PeakSource == "" {
		t.Error("ram.peakSource must say where the peak came from")
	}
	if len(rep.RAM.Series.Values) < 2 || rep.RAM.Series.IntervalSeconds != 1 {
		t.Errorf("ram.series: %d values at %vs", len(rep.RAM.Series.Values), rep.RAM.Series.IntervalSeconds)
	}
	maxWS := uint64(0)
	for _, v := range rep.RAM.Series.Values {
		maxWS = max(maxWS, v)
	}
	if maxWS < 100*mib {
		t.Errorf("the working-set series must show the 128 MiB allocation, max was %d", maxWS)
	}
	if rep.CPU.Peak < 500 {
		t.Errorf("cpu.peak = %d millicores, a spinning core under a 1-CPU quota must show well above 500", rep.CPU.Peak)
	}
	if rep.CPU.UsageSeconds < 1.5 {
		t.Errorf("cpu.usageSeconds = %v for a 3 s spin", rep.CPU.UsageSeconds)
	}
	if len(rep.CPU.Series.Values) < 1 {
		t.Error("cpu.series must have values")
	}
	if rep.Duration == nil || *rep.Duration < 2.9 || *rep.Duration > 20 {
		t.Errorf("duration = %v", rep.Duration)
	}
	if rep.Points.Start == nil || rep.Points.End == nil ||
		rep.Points.End.CPUUsageSeconds <= rep.Points.Start.CPUUsageSeconds {
		t.Errorf("points: %+v", rep.Points)
	}
	if rep.OOMKilled || rep.MemoryEvents.OOMKill != 0 {
		t.Errorf("no OOM expected: %+v", rep.MemoryEvents)
	}
	if rep.DiskIO == nil {
		t.Error("diskIo must be present: /proc/<pid>/io of the command is readable by its own uid")
	}
	if rep.Exec == nil || rep.Exec.Command != "memhog" || rep.Exec.Mode != "argv" || rep.ExecID == "" {
		t.Errorf("exec: %+v id=%q", rep.Exec, rep.ExecID)
	}
	if rep.Sampling.Observed < 3 || rep.Sampling.ReadErrors != 0 {
		t.Errorf("sampling: %+v", rep.Sampling)
	}
	if rep.Wrapper.Version != "test" {
		t.Errorf("wrapper version stamping: %+v", rep.Wrapper)
	}
}

func TestDockerOOMKillIsDetected(t *testing.T) {
	dockerEnabled(t)
	bins := linuxBinaries(t)
	work := workDir(t)

	r := dockerRun(t, bins, work,
		[]string{"--memory", "64m", "--memory-swap", "64m"},
		map[string]string{"PL_JOB_COMPLETION_MARKER_PATH": "/work/" + serviceMarkerFile},
		"--flush-interval", "1s", "--", "/opt/jw/memhog", "--mb", "256", "--hold", "2s")
	if r.exitCode != 137 {
		t.Fatalf("exit %d, want 137 for a SIGKILLed command", r.exitCode)
	}
	assertContains(t, "stderr", r.stderr, "Process exited with code 137")
	assertContains(t, "stderr", r.stderr, "out of memory")
	assertContains(t, "stderr", r.stderr, "OOM killer")
	if got := strings.TrimSpace(readFile(t, filepath.Join(work, serviceMarkerFile))); got != "137" {
		t.Errorf("marker = %q: the wrapper survives its child's OOM kill and records the code", got)
	}

	rep := readReport(t, work)
	if rep.State != report.StateFinished || rep.ExitCode == nil || *rep.ExitCode != 137 {
		t.Errorf("outcome: state=%s exit=%v", rep.State, rep.ExitCode)
	}
	if rep.Signal != "SIGKILL" {
		t.Errorf("signal = %q, want SIGKILL", rep.Signal)
	}
	if !rep.OOMKilled || rep.MemoryEvents.OOMKill < 1 {
		t.Errorf(
			"the kernel's oom_kill counter is the definitive signal: killed=%v events=%+v",
			rep.OOMKilled,
			rep.MemoryEvents,
		)
	}
	if rep.Granted.MemoryBytes != 64*mib {
		t.Errorf("granted.ram = %d", rep.Granted.MemoryBytes)
	}
	// The kernel's high-water mark may sit a page or so above memory.max at the kill.
	if rep.RAM.Peak < 56*mib || rep.RAM.Peak > 65*mib {
		t.Errorf("ram.peak = %d, want about the 64 MiB limit", rep.RAM.Peak)
	}
}

func TestDockerReportIsPublishedWhileRunning(t *testing.T) {
	dockerEnabled(t)
	bins := linuxBinaries(t)
	work := workDir(t)

	c := dockerStart(t, bins, work, []string{"--memory", "256m", "--memory-swap", "256m"}, nil,
		"--flush-interval", "1s", "--", "/opt/jw/memhog", "--mb", "32", "--hold", "6s")

	// The runner does not have to wait for the command: a report shows up early and keeps moving.
	var first *report.Report
	waitFor(t, 8*time.Second, func() bool {
		rep, ok := tryReadReport(work)
		if !ok || rep.State != report.StateRunning || rep.Sampling.Observed < 2 {
			return false
		}
		first = rep
		return true
	})
	if first.ExitCode != nil || first.FinishedAt != nil || first.Duration != nil {
		t.Errorf("a running report carries no outcome yet: %+v", first)
	}
	if first.StartedAt == 0 || first.UpdatedAt < first.StartedAt || first.Points.Start == nil {
		t.Errorf(
			"running report header: started=%d updated=%d start=%+v",
			first.StartedAt,
			first.UpdatedAt,
			first.Points.Start,
		)
	}
	waitFor(t, 8*time.Second, func() bool {
		rep, ok := tryReadReport(work)
		return ok && rep.UpdatedAt > first.UpdatedAt && rep.Sampling.Observed > first.Sampling.Observed
	})
	if exists(filepath.Join(work, serviceReportFile+".tmp")) {
		t.Log("a .tmp file was observed mid-write; that is fine, the rename is what readers rely on")
	}

	if code := c.wait(t); code != 0 {
		t.Fatalf("container exit %d", code)
	}
	rep := readReport(t, work)
	if rep.State != report.StateFinished || rep.ExitCode == nil || *rep.ExitCode != 0 || rep.Points.End == nil {
		t.Errorf("final report: state=%s exit=%v end=%+v", rep.State, rep.ExitCode, rep.Points.End)
	}
	if rep.Duration == nil || *rep.Duration < 5.9 {
		t.Errorf("duration = %v for a 6 s hold", rep.Duration)
	}
}

func TestDockerLegacyShellContractAsPID1(t *testing.T) {
	dockerEnabled(t)
	bins := linuxBinaries(t)
	work := workDir(t)

	writeFile(t, filepath.Join(work, "input.txt"), "keep", 0o644)
	writeFile(t, filepath.Join(work, "leftover.bin"), "stale", 0o644)
	writeFile(t, filepath.Join(work, serviceMarkerFile), "0\n", 0o644)
	writeFile(t, filepath.Join(work, serviceItemsFile), "input.txt\n"+serviceItemsFile+"\n", 0o644)
	_ = os.Chmod(filepath.Join(work, ".pl"), 0o777)

	r := dockerRun(t, bins, work, []string{"--memory", "128m", "--memory-swap", "128m"}, map[string]string{
		"PL_JOB_CMD_AND_ARGS":           `'sh' '-c' 'echo OUT_MARKER; echo ERR_MARKER >&2; echo pid=$$ ppid=$PPID; exit 3'`,
		"PL_JOB_STDOUT_PATH":            "/work/stdout.log",
		"PL_JOB_STDERR_PATH":            "/work/stderr.log",
		"PL_JOB_COMPLETION_MARKER_PATH": "/work/" + serviceMarkerFile,
		"PL_JOB_EXPECTED_ITEMS_FILE":    "/work/" + serviceItemsFile,
	})
	if r.exitCode != 3 {
		t.Fatalf("exit %d", r.exitCode)
	}
	assertContains(t, "stdout", r.stdout, "OUT_MARKER")
	assertContains(t, "stdout", r.stdout, "ppid=1")
	assertContains(t, "stderr", r.stderr, "ERR_MARKER")
	assertContains(t, "stderr", r.stderr, "[job-script] Pruning unexpected file: leftover.bin")
	assertContains(t, "stderr", r.stderr, "[job-script] Pruning unexpected file: .pl/completed")
	assertContains(t, "stderr", r.stderr, "[job-script] Process exited with code 3")
	assertContains(t, "stdout file", readFile(t, filepath.Join(work, "stdout.log")), "OUT_MARKER")
	errLog := readFile(t, filepath.Join(work, "stderr.log"))
	assertContains(t, "stderr file", errLog, "ERR_MARKER")
	assertContains(t, "stderr file", errLog, "Process exited with code 3")
	if got := strings.TrimSpace(readFile(t, filepath.Join(work, serviceMarkerFile))); got != "3" {
		t.Errorf("marker = %q", got)
	}
	if exists(filepath.Join(work, "leftover.bin")) || !exists(filepath.Join(work, "input.txt")) {
		t.Error("prune outcome wrong")
	}

	rep := readReport(t, work)
	if rep.Exec == nil || rep.Exec.Command != "sh" || rep.Exec.Mode != "shell" || rep.Exec.ArgCount != 2 {
		t.Errorf("exec derived from the quoted shell line: %+v", rep.Exec)
	}
	if rep.ExitCode == nil || *rep.ExitCode != 3 || !rep.Cgroup.Available || rep.Granted.MemoryBytes != 128*mib {
		t.Errorf("report: exit=%v cgroup=%+v granted=%+v", rep.ExitCode, rep.Cgroup, rep.Granted)
	}
}

func TestDockerReapsOrphansAsPID1(t *testing.T) {
	dockerEnabled(t)
	bins := linuxBinaries(t)
	work := workDir(t)

	c := dockerStart(t, bins, work, nil, nil, "--flush-interval", "1s", "--", "/opt/jw/memhog", "--orphan", "--hold", "5s")
	waitFor(t, 8*time.Second, func() bool {
		rep, ok := tryReadReport(work)
		return ok && rep.State == report.StateRunning
	})
	time.Sleep(1500 * time.Millisecond) // the orphan's `sleep 0.2` has exited by now

	// Every process state in the container; a naive PID 1 leaves the orphan as Z.
	out, err := exec.CommandContext(t.Context(), "docker", "exec", c.name, "sh", "-c",
		`for p in /proc/[0-9]*; do [ -r "$p/stat" ] && awk '{print $1, $3}' "$p/stat"; done`).CombinedOutput()
	if err != nil {
		t.Fatalf("docker exec: %v: %s", err, out)
	}
	t.Logf("process states:\n%s", out)
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if strings.HasSuffix(line, " Z") {
			t.Errorf("zombie left behind: %q", line)
		}
	}
	if code := c.wait(t); code != 0 {
		t.Fatalf("container exit %d", code)
	}
}
