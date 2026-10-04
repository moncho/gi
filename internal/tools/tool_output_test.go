package tools

import (
	"context"
	"errors"
	goai "github.com/rcarmo/go-ai"
	"os"
	"strings"
	"testing"
	"time"
)

func TestShellToolOutputStreamsAndReturnsUnchanged(t *testing.T) {
	var snapshots []string
	out, err := ExecuteShellOutput(context.Background(), t.TempDir(), goai.ToolCall{Arguments: map[string]any{"command": "printf 'first\\n'; sleep 0.05; printf 'second\\n' >&2"}}, func(text string) error { snapshots = append(snapshots, text); return nil })
	if err != nil || out != "first\nsecond\n" || len(snapshots) < 2 || snapshots[0] != "first\n" || snapshots[len(snapshots)-1] != out {
		t.Fatal(out, snapshots, err)
	}
}
func TestShellToolOutputPersistenceFailurePropagates(t *testing.T) {
	failure := errors.New("persist preview failed")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := ExecuteShellOutput(ctx, t.TempDir(), goai.ToolCall{Arguments: map[string]any{"command": "printf output; while :; do printf 'more output\\n'; done"}}, func(string) error { return failure })
	if ctx.Err() != nil {
		t.Fatal("reporter failure did not terminate the writer promptly")
	}
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
func TestShellToolOutputFailureRetainsStderr(t *testing.T) {
	var last string
	out, err := ExecuteShellOutput(context.Background(), t.TempDir(), goai.ToolCall{Arguments: map[string]any{"command": "printf 'failed details' >&2; exit 7"}}, func(text string) error { last = text; return nil })
	if err == nil || !strings.Contains(out, "failed details") || last != out {
		t.Fatal(out, last, err)
	}
}

// A preparer (the keychain) rewrites the command and adds environment; its
// error fails the call before anything runs.
func TestExecuteShellPrepared(t *testing.T) {
	prepare := func(_ context.Context, command string) (PreparedShell, error) {
		return PreparedShell{Command: strings.ReplaceAll(command, "PLACEHOLDER", "resolved"), Env: append(os.Environ(), "GI_TEST_SECRET=injected")}, nil
	}
	out, err := ExecuteShellPrepared(context.Background(), t.TempDir(), goai.ToolCall{Arguments: map[string]any{"command": `printf '%s %s' PLACEHOLDER "$GI_TEST_SECRET"`}}, nil, prepare)
	if err != nil || out != "resolved injected" {
		t.Fatalf("%q %v", out, err)
	}
	dir := t.TempDir()
	failing := func(context.Context, string) (PreparedShell, error) {
		return PreparedShell{}, errors.New("entry not found")
	}
	if _, err := ExecuteShellPrepared(context.Background(), dir, goai.ToolCall{Arguments: map[string]any{"command": "touch ran"}}, nil, failing); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir + "/ran"); err == nil {
		t.Fatal("command ran after the preparer failed")
	}
}
