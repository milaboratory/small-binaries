//go:build unix

package wrapper

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// setProcAttr puts the command in its own process group so a forwarded signal reaches the whole
// tree it spawns, not just the shell in front of it.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// forwardSignal delivers sig to the command's process group.
func forwardSignal(cmd *exec.Cmd, sig os.Signal) {
	if cmd.Process == nil {
		return
	}
	if s, ok := sig.(syscall.Signal); ok {
		err := syscall.Kill(-cmd.Process.Pid, s)
		if err == nil {
			return
		}
	}
	_ = cmd.Process.Signal(sig)
}

// waitChild waits for the command while reaping every other child that gets reparented to the
// wrapper. Inside a container the wrapper is PID 1, so orphaned grandchildren land on it; without
// this loop each of them would stay a zombie for the rest of the run.
func waitChild(cmd *exec.Cmd) waitResult {
	pid := cmd.Process.Pid
	for {
		var ws syscall.WaitStatus
		wpid, err := syscall.Wait4(-1, &ws, 0, nil)
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			// ECHILD: the command is gone and nobody told us its status.
			return waitResult{exitCode: 1, err: err}
		}
		if wpid != pid {
			continue // an orphan, reaped
		}
		_ = cmd.Process.Release()
		if ws.Signaled() {
			sig := ws.Signal()
			return waitResult{exitCode: 128 + int(sig), signal: signalName(sig)}
		}
		return waitResult{exitCode: ws.ExitStatus()}
	}
}

var signalNames = map[syscall.Signal]string{
	syscall.SIGHUP:  "SIGHUP",
	syscall.SIGINT:  "SIGINT",
	syscall.SIGQUIT: "SIGQUIT",
	syscall.SIGILL:  "SIGILL",
	syscall.SIGABRT: "SIGABRT",
	syscall.SIGBUS:  "SIGBUS",
	syscall.SIGFPE:  "SIGFPE",
	syscall.SIGKILL: "SIGKILL",
	syscall.SIGSEGV: "SIGSEGV",
	syscall.SIGPIPE: "SIGPIPE",
	syscall.SIGALRM: "SIGALRM",
	syscall.SIGTERM: "SIGTERM",
	syscall.SIGXCPU: "SIGXCPU",
	syscall.SIGXFSZ: "SIGXFSZ",
}

func signalName(sig os.Signal) string {
	if s, ok := sig.(syscall.Signal); ok {
		if name, ok := signalNames[s]; ok {
			return name
		}
		return "SIG" + itoa(int(s))
	}
	return sig.String()
}
