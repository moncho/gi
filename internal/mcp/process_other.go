//go:build !unix

package mcp

import "os/exec"

func setProcessGroup(*exec.Cmd) {}

func signalProcessGroup(cmd *exec.Cmd, _ bool) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func processGroupAlive(*exec.Cmd) bool { return false }
