//go:build windows

package wrapper

import (
	"errors"
	"os"
	"os/exec"
)

func setProcAttr(cmd *exec.Cmd) {}

func forwardSignal(cmd *exec.Cmd, sig os.Signal) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func waitChild(cmd *exec.Cmd) waitResult {
	err := cmd.Wait()
	if err == nil {
		return waitResult{exitCode: 0}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return waitResult{exitCode: exitErr.ExitCode()}
	}
	return waitResult{exitCode: 1, err: err}
}

func signalName(sig os.Signal) string { return sig.String() }
