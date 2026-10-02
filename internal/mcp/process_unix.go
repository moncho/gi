//go:build unix

package mcp

import (
	"os/exec"
	"syscall"
)

// setProcessGroup starts a stdio server in its own process group, so stopping
// it also stops children of wrappers such as npx or uvx (Pi).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// signalProcessGroup sends SIGTERM (or SIGKILL when kill) to the group.
func signalProcessGroup(cmd *exec.Cmd, kill bool) {
	if cmd.Process == nil {
		return
	}
	sig := syscall.SIGTERM
	if kill {
		sig = syscall.SIGKILL
	}
	if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil {
		_ = cmd.Process.Signal(sig)
	}
}

// processGroupAlive reports whether any process remains in the group.
func processGroupAlive(cmd *exec.Cmd) bool {
	return cmd.Process != nil && syscall.Kill(-cmd.Process.Pid, 0) == nil
}
