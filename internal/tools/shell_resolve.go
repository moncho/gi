package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ShellConfig is one shell the shell tool can run a command with, as
// Piclaw 3.2.5's tracked-bash ShellConfig.
type ShellConfig struct {
	Shell  string
	Args   []string
	Family string // "posix", "powershell" or "cmd"
}

var (
	posixArgs      = []string{"-c"}
	powershellArgs = []string{"-NoProfile", "-Command"}
	cmdArgs        = []string{"/c"}
)

// ShellCandidates is Piclaw's resolveShellCandidates: a configured shellPath
// alone (an error when it does not exist); otherwise, on POSIX hosts, $SHELL
// when it names an existing file, /bin/bash, then bash from PATH; on Windows
// $SHELL, PowerShell 7, Windows PowerShell, pwsh.exe, powershell.exe,
// %ComSpec% and cmd.exe.
func ShellCandidates(shellPath, goos string, getenv func(string) string, exists func(string) bool) ([]ShellConfig, error) {
	if shellPath = strings.TrimSpace(shellPath); shellPath != "" {
		if !exists(shellPath) {
			return nil, fmt.Errorf("Custom shell path not found: %s", shellPath)
		}
		return []ShellConfig{{shellPath, posixArgs, "posix"}}, nil
	}
	var out []ShellConfig
	push := func(c ShellConfig) {
		if strings.TrimSpace(c.Shell) == "" {
			return
		}
		for _, e := range out {
			if strings.EqualFold(e.Shell, c.Shell) {
				return
			}
		}
		out = append(out, c)
	}
	if sh := getenv("SHELL"); sh != "" && exists(sh) {
		push(ShellConfig{sh, posixArgs, "posix"})
	}
	if goos == "windows" {
		for _, p := range []string{`C:\Program Files\PowerShell\7\pwsh.exe`, `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`} {
			if exists(p) {
				push(ShellConfig{p, powershellArgs, "powershell"})
			}
		}
		push(ShellConfig{"pwsh.exe", powershellArgs, "powershell"})
		push(ShellConfig{"powershell.exe", powershellArgs, "powershell"})
		if cs := strings.TrimSpace(getenv("ComSpec")); cs != "" {
			push(ShellConfig{cs, cmdArgs, "cmd"})
		}
		push(ShellConfig{"cmd.exe", cmdArgs, "cmd"})
		return out, nil
	}
	if exists("/bin/bash") {
		push(ShellConfig{"/bin/bash", posixArgs, "posix"})
	}
	push(ShellConfig{"bash", posixArgs, "posix"})
	return out, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ResolveShell is the first candidate that can be started (Piclaw skips a
// candidate whose spawn fails with ENOENT).
func ResolveShell(shellPath string) (ShellConfig, error) {
	candidates, err := ShellCandidates(shellPath, runtime.GOOS, os.Getenv, fileExists)
	if err != nil {
		return ShellConfig{}, err
	}
	var tried []string
	for _, c := range candidates {
		tried = append(tried, c.Shell)
		if path, err := exec.LookPath(c.Shell); err == nil {
			c.Shell = path
			return c, nil
		}
	}
	attempted := "(none)"
	if len(tried) > 0 {
		attempted = strings.Join(tried, ", ")
	}
	return ShellConfig{}, fmt.Errorf("No supported shell found. Tried: %s", attempted)
}

// ShellCommand is command run by the detected shell.
func ShellCommand(ctx context.Context, shellPath, command string) (*exec.Cmd, error) {
	shell, err := ResolveShell(shellPath)
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, shell.Shell, append(append([]string(nil), shell.Args...), command)...), nil
}
