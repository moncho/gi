package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/rtk"
	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func ScriptWithPayload(engine, name string, payload map[string]any, script string) string {
	if strings.TrimSpace(script) == "" {
		return script
	}
	b, _ := json.Marshal(payload)
	if engine == "joker" || engine == "" && strings.HasPrefix(strings.TrimSpace(script), "(") {
		return fmt.Sprintf("(def *gi-%s* (walk/keywordize-keys (json/read-string %q)))\n%s", name, string(b), script)
	}
	return fmt.Sprintf("gi.%s = %s; gi.%sPayload = gi.%s; gi.toolArgs = (gi.tool && gi.tool.arguments) || {};\n%s", name, string(b), name, name, script)
}

func ExecuteWrite(ctx context.Context, cfg config.RuntimeConfig, s *store.Store, call goai.ToolCall) (string, error) {
	path, _ := call.Arguments["path"].(string)
	content, _ := call.Arguments["content"].(string)
	if path == "" {
		return "", fmt.Errorf("write: path is required")
	}
	if err := WriteFile(ctx, cfg, s, path, content); err != nil {
		return "", err
	}
	return "written", nil
}

func ExecuteRTK(ctx context.Context, workspaceRoot string, call goai.ToolCall) (string, error) {
	command, _ := call.Arguments["command"].(string)
	if command == "" {
		return "", fmt.Errorf("rtk: command is required")
	}
	filterOnly, _ := call.Arguments["filter_only"].(bool)
	output, _ := call.Arguments["output"].(string)
	var err error
	if !filterOnly {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Dir = workspaceRoot
		out, runErr := cmd.CombinedOutput()
		output = string(out)
		err = runErr
	}
	res := rtk.Filter(command, output)
	b, _ := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return string(b), shellExitError(ctx, string(b), err)
	}
	return string(b), nil
}

func ExecuteShell(ctx context.Context, workspaceRoot string, call goai.ToolCall) (string, error) {
	return ExecuteShellOutput(ctx, workspaceRoot, call, nil)
}

func ExecuteShellOutput(ctx context.Context, workspaceRoot string, call goai.ToolCall, onOutput func(string) error) (string, error) {
	return ExecuteShellPrepared(ctx, workspaceRoot, call, onOutput, nil)
}

// PreparedShell is a shell command as it runs: the command text (keychain
// placeholders resolved), its full environment (nil: the process's) and the
// configured shell path ("" to detect one).
type PreparedShell struct {
	Command   string
	Env       []string
	ShellPath string
}

// ShellPreparer prepares a shell command before it runs.
type ShellPreparer func(ctx context.Context, command string) (PreparedShell, error)

// ExecuteShellPrepared runs the shell tool's command after prepare (nil for
// none) in the detected shell.
func ExecuteShellPrepared(ctx context.Context, workspaceRoot string, call goai.ToolCall, onOutput func(string) error, prepare ShellPreparer) (string, error) {
	command, _ := call.Arguments["command"].(string)
	if command == "" {
		return "", fmt.Errorf("shell: command is required")
	}
	prepared := PreparedShell{Command: command}
	if prepare != nil {
		var err error
		if prepared, err = prepare(ctx, command); err != nil {
			return "", fmt.Errorf("shell: %w", err)
		}
	}
	cmd, err := ShellCommand(ctx, prepared.ShellPath, prepared.Command)
	if err != nil {
		return "", fmt.Errorf("shell: %w", err)
	}
	cmd.Dir = workspaceRoot
	if prepared.Env != nil {
		cmd.Env = prepared.Env
	}
	if onOutput == nil {
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), shellExitError(ctx, string(out), err)
		}
		return string(out), nil
	}
	configureShellProcess(cmd)
	cmd.Cancel = func() error { killShellProcess(cmd); return nil }
	output := &toolOutputWriter{notify: onOutput, cancel: func() { killShellProcess(cmd) }}
	cmd.Stdout, cmd.Stderr = output, output
	err = cmd.Run()
	if output.err != nil {
		return output.text.String(), output.err
	}
	if err != nil {
		return output.text.String(), shellExitError(ctx, output.text.String(), err)
	}
	return output.text.String(), nil
}

// exec may copy stdout/stderr concurrently; serialize cumulative snapshots.
type toolOutputWriter struct {
	mu     sync.Mutex
	text   strings.Builder
	notify func(string) error
	cancel func()
	err    error
}

func (w *toolOutputWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return len(p), nil
	} // keep draining until the killed process exits
	w.text.Write(p)
	w.err = w.notify(w.text.String())
	if w.err != nil && w.cancel != nil {
		w.cancel()
	}
	return len(p), nil
}
