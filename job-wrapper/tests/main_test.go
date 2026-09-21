// Package tests runs the compiled wrapper end to end: on the host against the legacy job-script.sh
// contract, and (with JOB_WRAPPER_DOCKER=1) inside Docker containers against real cgroups.
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	serviceItemsFile  = ".pl/expected_items"
	serviceMarkerFile = ".pl/completed"
	serviceReportFile = ".pl/usage.json"
	tmpDirRel         = ".pl/tmp"
)

var (
	binDir   string // built binaries live here
	hostBin  string // job-wrapper for the host OS
	moduleRt string // module root
)

func TestMain(m *testing.M) {
	var err error
	moduleRt, err = filepath.Abs("..")
	if err != nil {
		panic(err)
	}
	binDir, err = os.MkdirTemp("", "job-wrapper-bin-")
	if err != nil {
		panic(err)
	}
	hostBin = filepath.Join(binDir, "job-wrapper-host")
	if runtime.GOOS == "windows" {
		hostBin += ".exe"
	}
	err = goBuild(hostBin, "./cmd/job-wrapper", "", "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "building job-wrapper:", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(binDir)
	os.Exit(code)
}

func goBuild(out, pkg, goos, goarch string) error {
	cmd := exec.CommandContext(context.Background(), "go", "build", "-ldflags", "-X main.version=test", "-o", out, pkg)
	cmd.Dir = moduleRt
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if goos != "" {
		cmd.Env = append(cmd.Env, "GOOS="+goos, "GOARCH="+goarch)
	}
	outb, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, outb)
	}
	return nil
}

type result struct {
	stdout, stderr string
	exitCode       int
}

// runWrapper executes the host binary with a clean environment (PATH and HOME only) plus env.
func runWrapper(t *testing.T, env map[string]string, args ...string) result {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), hostBin, args...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running job-wrapper: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return result{stdout: out.String(), stderr: errb.String(), exitCode: code}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("reading %s: %v", p, err)
	}
	return string(b)
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func mkdir(t *testing.T, p string) string {
	t.Helper()
	err := os.MkdirAll(p, 0o750)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeFile(t *testing.T, p, content string, mode os.FileMode) {
	t.Helper()
	mkdir(t, filepath.Dir(p))
	err := os.WriteFile(p, []byte(content), mode)
	if err != nil {
		t.Fatal(err)
	}
}

// writeExpectedItemsFile writes the list the way the runner does: one relative path per line, the
// file itself last so it survives its own prune.
func writeExpectedItemsFile(t *testing.T, workdir string, items ...string) string {
	t.Helper()
	p := filepath.Join(workdir, serviceItemsFile)
	writeFile(t, p, strings.Join(items, "\n")+"\n"+serviceItemsFile+"\n", 0o600)
	return p
}

func assertContains(t *testing.T, what, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("%s must contain %q, got:\n%s", what, sub, s)
	}
}

func assertNotContains(t *testing.T, what, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Errorf("%s must not contain %q, got:\n%s", what, sub, s)
	}
}
