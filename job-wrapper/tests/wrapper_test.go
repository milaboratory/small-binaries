package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/milaboratory/small-binaries/job-wrapper/internal/report"
)

const markers = `printf "STDOUT_MARKER\n"; printf "STDERR_MARKER\n" >&2`

func skipOnWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the legacy contract needs a POSIX sh")
	}
}

// --- Redirections, success and failure ---

func TestSuccessNoRedirects(t *testing.T) {
	skipOnWindows(t)
	r := runWrapper(t, map[string]string{"PL_JOB_CMD_AND_ARGS": markers})
	if r.exitCode != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exitCode, r.stderr)
	}
	assertContains(t, "stdout", r.stdout, "STDOUT_MARKER")
	assertNotContains(t, "stdout", r.stdout, "STDERR_MARKER")
	assertContains(t, "stderr", r.stderr, "STDERR_MARKER")
	assertNotContains(t, "stderr", r.stderr, "STDOUT_MARKER")
}

func TestRedirects(t *testing.T) {
	skipOnWindows(t)
	for _, tc := range []struct {
		name     string
		exit     int
		stdout   bool
		stderr   bool
		sameFile bool
	}{
		{"stdout only", 0, true, false, false},
		{"stderr only", 0, false, true, false},
		{"both same file", 0, true, true, true},
		{"both different files", 0, true, true, false},
		{"failure stdout only", 42, true, false, false},
		{"failure stderr only", 42, false, true, false},
		{"failure both same file", 42, true, true, true},
		{"failure both different files", 42, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			env := map[string]string{"PL_JOB_CMD_AND_ARGS": markers + "; exit " + itoa(tc.exit)}
			outPath, errPath := filepath.Join(dir, "stdout_file"), filepath.Join(dir, "stderr_file")
			if tc.sameFile {
				outPath = filepath.Join(dir, "combined")
				errPath = outPath
			}
			if tc.stdout {
				env["PL_JOB_STDOUT_PATH"] = outPath
			}
			if tc.stderr {
				env["PL_JOB_STDERR_PATH"] = errPath
			}

			r := runWrapper(t, env)
			if r.exitCode != tc.exit {
				t.Fatalf("exit %d, want %d; stderr: %s", r.exitCode, tc.exit, r.stderr)
			}

			if tc.sameFile {
				// Both streams flow through one sink onto the real stderr, as `exec 1>&2` did.
				if r.stdout != "" {
					t.Errorf("stdout must be empty, got %q", r.stdout)
				}
				assertContains(t, "stderr", r.stderr, "STDOUT_MARKER")
				assertContains(t, "stderr", r.stderr, "STDERR_MARKER")
				content := readFile(t, outPath)
				assertContains(t, "combined file", content, "STDOUT_MARKER")
				assertContains(t, "combined file", content, "STDERR_MARKER")
				if tc.exit != 0 {
					assertContains(t, "combined file", content, "Process exited with code 42")
				}
				return
			}

			assertContains(t, "stdout", r.stdout, "STDOUT_MARKER")
			assertNotContains(t, "stdout", r.stdout, "STDERR_MARKER")
			assertContains(t, "stderr", r.stderr, "STDERR_MARKER")
			assertNotContains(t, "stderr", r.stderr, "STDOUT_MARKER")
			if tc.stdout {
				content := readFile(t, outPath)
				assertContains(t, "stdout file", content, "STDOUT_MARKER")
				assertNotContains(t, "stdout file", content, "STDERR_MARKER")
			}
			if tc.stderr {
				content := readFile(t, errPath)
				assertContains(t, "stderr file", content, "STDERR_MARKER")
				assertNotContains(t, "stderr file", content, "STDOUT_MARKER")
				if tc.exit != 0 {
					assertContains(t, "stderr file", content, "Process exited with code 42")
				}
			}
		})
	}
}

func TestRedirectAppends(t *testing.T) {
	skipOnWindows(t)
	p := filepath.Join(t.TempDir(), "out")
	writeFile(t, p, "EARLIER\n", 0o600)
	r := runWrapper(t, map[string]string{"PL_JOB_CMD_AND_ARGS": "echo LATER", "PL_JOB_STDOUT_PATH": p})
	if r.exitCode != 0 {
		t.Fatal(r.stderr)
	}
	if got := readFile(t, p); got != "EARLIER\nLATER\n" {
		t.Errorf("tee -a semantics: %q", got)
	}
}

func TestRedirectToUnwritablePathFails(t *testing.T) {
	skipOnWindows(t)
	r := runWrapper(t, map[string]string{
		"PL_JOB_CMD_AND_ARGS": "echo never",
		"PL_JOB_STDOUT_PATH":  filepath.Join(t.TempDir(), "missing-dir", "out"),
	})
	if r.exitCode == 0 {
		t.Fatal("an unopenable log file must fail the wrapper, not hang it")
	}
	assertContains(t, "stderr", r.stderr, "output redirection")
	assertNotContains(t, "stdout", r.stdout, "never")
}

// --- PATH handling ---

func TestJobPathPrepend(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "my-custom-cmd"), "#!/bin/sh\necho CUSTOM_PATH_WORKS\n", 0o500)
	r := runWrapper(t, map[string]string{"PL_JOB_CMD_AND_ARGS": "my-custom-cmd", "PL_JOB_PATH": dir})
	if r.exitCode != 0 {
		t.Fatal(r.stderr)
	}
	assertContains(t, "stdout", r.stdout, "CUSTOM_PATH_WORKS")
}

func TestGpuBinPathPrependDoesNotWipeImagePath(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "fake-nvidia-smi"), "#!/bin/sh\necho GPU_BIN_WORKS\n", 0o500)
	r := runWrapper(t, map[string]string{
		"PL_JOB_CMD_AND_ARGS": "fake-nvidia-smi && echo IMAGE_PATH_PRESERVED",
		"PL_GPU_BIN_PATH":     dir,
	})
	if r.exitCode != 0 {
		t.Fatal(r.stderr)
	}
	assertContains(t, "stdout", r.stdout, "GPU_BIN_WORKS")
	assertContains(t, "stdout", r.stdout, "IMAGE_PATH_PRESERVED")
}

func TestGpuLibPath(t *testing.T) {
	skipOnWindows(t)
	r := runWrapper(t, map[string]string{
		"PL_JOB_CMD_AND_ARGS": `printf '%s\n' "$LD_LIBRARY_PATH"`,
		"PL_GPU_LIB_PATH":     "/usr/local/nvidia/lib64:/usr/local/nvidia/lib",
	})
	if strings.TrimSpace(r.stdout) != "/usr/local/nvidia/lib64:/usr/local/nvidia/lib" {
		t.Errorf("LD_LIBRARY_PATH = %q", r.stdout)
	}
	r = runWrapper(t, map[string]string{
		"PL_JOB_CMD_AND_ARGS": `printf '%s\n' "$LD_LIBRARY_PATH"`,
		"PL_GPU_LIB_PATH":     "/usr/local/nvidia/lib64",
		"LD_LIBRARY_PATH":     "/opt/conda/lib",
	})
	if strings.TrimSpace(r.stdout) != "/usr/local/nvidia/lib64:/opt/conda/lib" {
		t.Errorf("LD_LIBRARY_PATH = %q", r.stdout)
	}
}

func TestJobPathWinsOverGpuBinPath(t *testing.T) {
	skipOnWindows(t)
	jobDir, gpuDir := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(jobDir, "which-wins"), "#!/bin/sh\necho JOB_PATH_WINS\n", 0o500)
	writeFile(t, filepath.Join(gpuDir, "which-wins"), "#!/bin/sh\necho GPU_PATH_WINS\n", 0o500)
	r := runWrapper(t, map[string]string{
		"PL_JOB_CMD_AND_ARGS": "which-wins",
		"PL_JOB_PATH":         jobDir,
		"PL_GPU_BIN_PATH":     gpuDir,
	})
	assertContains(t, "stdout", r.stdout, "JOB_PATH_WINS")
	assertNotContains(t, "stdout", r.stdout, "GPU_PATH_WINS")
}

// --- Prune and the temporary directory, through the binary ---

func TestPruneThroughBinary(t *testing.T) {
	skipOnWindows(t)
	workdir := mkdir(t, filepath.Join(t.TempDir(), "workdir"))
	writeFile(t, filepath.Join(workdir, "input.txt"), "keep", 0o600)
	writeFile(t, filepath.Join(workdir, "output", "stale.bin"), "stale", 0o600)
	writeFile(t, filepath.Join(workdir, "pframe_1", "data.bin"), "stale", 0o600)
	writeFile(t, filepath.Join(workdir, serviceMarkerFile), "0\n", 0o600)
	writeFile(t, filepath.Join(workdir, tmpDirRel, "leftover.tmp"), "stale", 0o600)
	items := writeExpectedItemsFile(t, workdir, "input.txt", "output/")

	r := runWrapper(t, map[string]string{
		"PL_JOB_CMD_AND_ARGS":        "ls -A " + filepath.Join(workdir, tmpDirRel) + " | wc -l",
		"PL_JOB_WORKDIR":             workdir,
		"PL_JOB_EXPECTED_ITEMS_FILE": items,
	})
	if r.exitCode != 0 {
		t.Fatal(r.stderr)
	}
	assertContains(t, "stderr", r.stderr, "[job-script] Pruning unexpected file: output/stale.bin")
	assertContains(t, "stderr", r.stderr, "[job-script] Pruning unexpected file: pframe_1/data.bin")
	assertContains(t, "stderr", r.stderr, "[job-script] Pruning empty directory: pframe_1")
	assertContains(t, "stderr", r.stderr, "[job-script] Pruning unexpected file: .pl/completed")
	assertNotContains(t, "stderr", r.stderr, "leftover.tmp")
	if !exists(filepath.Join(workdir, "input.txt")) || !exists(filepath.Join(workdir, "output")) || !exists(items) {
		t.Error("expected items must survive")
	}
	if exists(filepath.Join(workdir, "pframe_1")) || exists(filepath.Join(workdir, serviceMarkerFile)) {
		t.Error("stale output and stale marker must go")
	}
	if strings.TrimSpace(r.stdout) != "0" {
		t.Errorf("the command must start on an empty temporary directory, saw %q entries", strings.TrimSpace(r.stdout))
	}
	if !exists(filepath.Join(workdir, tmpDirRel)) {
		t.Error("the temporary directory itself stays: it is a mount point on Kubernetes")
	}
}

func TestPruneSkippedWhenListMissingEmptyOrInline(t *testing.T) {
	skipOnWindows(t)
	for name, env := range map[string]map[string]string{
		"unset":   {},
		"missing": {"PL_JOB_EXPECTED_ITEMS_FILE": "does_not_exist"},
		"empty":   {"PL_JOB_EXPECTED_ITEMS_FILE": ""},
		"inline":  {"PL_JOB_EXPECTED_ITEMS": "expected.txt"},
	} {
		t.Run(name, func(t *testing.T) {
			workdir := mkdir(t, filepath.Join(t.TempDir(), "workdir"))
			writeFile(t, filepath.Join(workdir, "leftover.bin"), "stale", 0o600)
			env["PL_JOB_CMD_AND_ARGS"] = "true"
			env["PL_JOB_WORKDIR"] = workdir
			if p, ok := env["PL_JOB_EXPECTED_ITEMS_FILE"]; ok && p != "" {
				env["PL_JOB_EXPECTED_ITEMS_FILE"] = filepath.Join(workdir, p)
			}
			if name == "empty" {
				p := filepath.Join(workdir, serviceItemsFile)
				writeFile(t, p, "", 0o600)
				env["PL_JOB_EXPECTED_ITEMS_FILE"] = p
			}
			r := runWrapper(t, env)
			if r.exitCode != 0 {
				t.Fatal(r.stderr)
			}
			if !exists(filepath.Join(workdir, "leftover.bin")) {
				t.Error("nothing may be pruned without a usable list")
			}
			assertNotContains(t, "stderr", r.stderr, "Pruning")
		})
	}
}

func TestTmpDirEmptiedWhenPruneIsSkipped(t *testing.T) {
	skipOnWindows(t)
	workdir := mkdir(t, filepath.Join(t.TempDir(), "workdir"))
	tmp := filepath.Join(workdir, tmpDirRel)
	writeFile(t, filepath.Join(tmp, "leftover.tmp"), "stale", 0o600)
	r := runWrapper(t, map[string]string{"PL_JOB_CMD_AND_ARGS": "ls -A " + tmp + " | wc -l", "PL_JOB_WORKDIR": workdir})
	if strings.TrimSpace(r.stdout) != "0" || !exists(tmp) {
		t.Errorf("tmp must be emptied but kept: %q", r.stdout)
	}
}

// --- Completion marker ---

func TestCompletionMarker(t *testing.T) {
	skipOnWindows(t)
	for _, code := range []int{0, 42, 137} {
		marker := filepath.Join(t.TempDir(), serviceMarkerFile) // parent .pl does not exist yet
		r := runWrapper(t, map[string]string{
			"PL_JOB_CMD_AND_ARGS":           "exit " + itoa(code),
			"PL_JOB_COMPLETION_MARKER_PATH": marker,
		})
		if r.exitCode != code {
			t.Errorf("exit %d, want %d", r.exitCode, code)
		}
		if got := strings.TrimSpace(readFile(t, marker)); got != itoa(code) {
			t.Errorf("marker = %q, want %d", got, code)
		}
		if code == 137 {
			assertContains(t, "stderr", r.stderr, "out of memory")
		}
	}
	r := runWrapper(t, map[string]string{"PL_JOB_CMD_AND_ARGS": "exit 0"})
	if r.exitCode != 0 || r.stderr != "" {
		t.Errorf("no marker, no report, quiet stderr: exit %d stderr %q", r.exitCode, r.stderr)
	}
}

func TestCompletionMarkerNotWrittenWhenKilled(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, serviceMarkerFile)
	sentinel := filepath.Join(dir, ".started")
	pidFile := filepath.Join(dir, ".pid")

	cmd := exec.CommandContext(t.Context(), hostBin)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"),
		"PL_JOB_CMD_AND_ARGS=echo $$ > " + pidFile + " && touch " + sentinel + " && sleep 60",
		"PL_JOB_COMPLETION_MARKER_PATH=" + marker}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, func() bool { return exists(sentinel) })
	// SIGKILL to the whole tree, the OOM reaper's shape.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	pid, err := strconv.Atoi(strings.TrimSpace(readFile(t, pidFile)))
	if err == nil {
		_ = syscall.Kill(-pid, syscall.SIGKILL) // the command's own process group
	}
	_ = cmd.Wait()
	if exists(marker) {
		t.Error("a killed wrapper must not leave a marker: its absence is the OOM signal")
	}
}

// --- Signals ---

func TestSigtermIsForwardedToTheCommand(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	sentinel := filepath.Join(dir, ".started")
	reportPath := filepath.Join(dir, "usage.json")

	cmd := exec.CommandContext(t.Context(), hostBin, "--report", reportPath, "--flush-interval", "1s")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"),
		`PL_JOB_CMD_AND_ARGS=trap 'exit 143' TERM; touch ` + sentinel + `; sleep 30 & wait $!`}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, func() bool { return exists(sentinel) })
	time.Sleep(200 * time.Millisecond)
	err = cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("the wrapper must exit soon after the command handles the forwarded SIGTERM")
	}
	if code := cmd.ProcessState.ExitCode(); code != 143 {
		t.Errorf("exit %d, want the trap's 143; stderr: %s", code, stderr.String())
	}
	rep, err := report.Read(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.TerminationSignal != "SIGTERM" || rep.State != report.StateFinished ||
		rep.ExitCode == nil || *rep.ExitCode != 143 {
		t.Errorf("report: signal=%q state=%q exit=%v", rep.TerminationSignal, rep.State, rep.ExitCode)
	}
}

// --- argv mode ---

func TestArgvMode(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "usage.json")
	outPath := filepath.Join(dir, "stdout")
	r := runWrapper(t, map[string]string{"PL_JOB_STDOUT_PATH": outPath},
		"--report", reportPath, "--", "sh", "-c", `printf 'arg with $pace\n'; exit 7`)
	if r.exitCode != 7 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}
	if got := readFile(t, outPath); got != "arg with $pace\n" {
		t.Errorf("argv must reach the command verbatim, no shell in between: %q", got)
	}
	rep, err := report.Read(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Exec == nil || rep.Exec.Command != "sh" || rep.Exec.Mode != "argv" || rep.Exec.ArgCount != 2 {
		t.Errorf("exec: %+v", rep.Exec)
	}
	if strings.Contains(rep.ExecID, "pace") {
		t.Errorf("argument values must not leak into execId: %q", rep.ExecID)
	}
}

func TestArgvNotFound(t *testing.T) {
	skipOnWindows(t)
	marker := filepath.Join(t.TempDir(), serviceMarkerFile)
	r := runWrapper(t, map[string]string{"PL_JOB_COMPLETION_MARKER_PATH": marker}, "--", "definitely-not-a-command-xyz")
	if r.exitCode != 127 {
		t.Errorf("exit %d, want 127 like sh", r.exitCode)
	}
	assertContains(t, "stderr", r.stderr, "definitely-not-a-command-xyz")
	if got := strings.TrimSpace(readFile(t, marker)); got != "127" {
		t.Errorf("marker = %q", got)
	}
}

// --- Report on the host ---

func TestReportWrittenOnHost(t *testing.T) {
	skipOnWindows(t)
	workdir := mkdir(t, filepath.Join(t.TempDir(), "workdir"))
	r := runWrapper(t, map[string]string{
		"PL_JOB_CMD_AND_ARGS": `'/usr/bin/env' 'sleep' '0.3'`,
		"PL_JOB_WORKDIR":      workdir,
		"PL_JOB_EXEC_ID":      "",
	})
	if r.exitCode != 0 {
		t.Fatal(r.stderr)
	}
	reportPath := filepath.Join(workdir, serviceReportFile)
	rep, err := report.Read(reportPath)
	if err != nil {
		t.Fatalf("default report location %s: %v", reportPath, err)
	}
	if exists(reportPath + ".tmp") {
		t.Error("temporary file must not survive")
	}
	if rep.Version != report.Version || rep.Wrapper.Name != "job-wrapper" || rep.Wrapper.Version != "test" {
		t.Errorf("header: %+v", rep.Wrapper)
	}
	if rep.State != report.StateFinished || rep.ExitCode == nil || *rep.ExitCode != 0 {
		t.Errorf("state=%q exit=%v", rep.State, rep.ExitCode)
	}
	if rep.Duration == nil || *rep.Duration < 0.25 || *rep.Duration > 5 {
		t.Errorf("duration = %v", rep.Duration)
	}
	if rep.FinishedAt == nil || *rep.FinishedAt < rep.StartedAt || rep.UpdatedAt < rep.StartedAt {
		t.Errorf("timestamps: started=%d finished=%v updated=%d", rep.StartedAt, rep.FinishedAt, rep.UpdatedAt)
	}
	if rep.Exec == nil || rep.Exec.Command != "env" || rep.Exec.Mode != "shell" ||
		rep.Exec.ArgCount != 2 || rep.ExecID != "env:"+rep.Exec.ArgsHash {
		t.Errorf("exec identity from the quoted shell line: %+v id=%q", rep.Exec, rep.ExecID)
	}
	if runtime.GOOS != "linux" && rep.Cgroup.Available {
		t.Errorf("no cgroups off Linux: %+v", rep.Cgroup)
	}
	if rep.RAM.Series.Values == nil || rep.CPU.Series.Values == nil {
		t.Error("series must be present (possibly empty), never null")
	}
}

func TestReportDisabledAndOverridden(t *testing.T) {
	skipOnWindows(t)
	workdir := mkdir(t, filepath.Join(t.TempDir(), "workdir"))
	r := runWrapper(t, map[string]string{
		"PL_JOB_CMD_AND_ARGS":      "true",
		"PL_JOB_WORKDIR":           workdir,
		"PL_JOB_USAGE_REPORT_PATH": "none",
	})
	if r.exitCode != 0 || exists(filepath.Join(workdir, serviceReportFile)) {
		t.Error("PL_JOB_USAGE_REPORT_PATH=none must disable the report")
	}
	r = runWrapper(t, map[string]string{"PL_JOB_CMD_AND_ARGS": "true", "PL_JOB_WORKDIR": workdir}, "--report", "none")
	if r.exitCode != 0 || exists(filepath.Join(workdir, serviceReportFile)) {
		t.Error("--report none must disable the report")
	}
	custom := filepath.Join(t.TempDir(), "elsewhere", "u.json")
	r = runWrapper(t, map[string]string{
		"PL_JOB_CMD_AND_ARGS":      "true",
		"PL_JOB_WORKDIR":           workdir,
		"PL_JOB_USAGE_REPORT_PATH": custom,
		"PL_JOB_EXEC_ID":           "upstream-id",
	})
	if r.exitCode != 0 || !exists(custom) || exists(filepath.Join(workdir, serviceReportFile)) {
		t.Error("the env path must win over the default")
	}
	if rep, _ := report.Read(custom); rep == nil || rep.ExecID != "upstream-id" {
		t.Error("PL_JOB_EXEC_ID must replace the derived execId")
	}
}

func TestNoCommandIsAUsageError(t *testing.T) {
	r := runWrapper(t, nil)
	if r.exitCode != 2 {
		t.Errorf("exit %d", r.exitCode)
	}
	assertContains(t, "stderr", r.stderr, "Usage")
	r = runWrapper(t, nil, "--version")
	if r.exitCode != 0 || !strings.Contains(r.stdout, "job-wrapper test") {
		t.Errorf("--version: %d %q", r.exitCode, r.stdout)
	}
}

func waitFor(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func itoa(i int) string { return strconv.Itoa(i) }
