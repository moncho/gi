package tools

import (
	"context"
	"errors"
	"testing"

	goai "github.com/rcarmo/go-ai"
)

// A failing command keeps its output and ends with Pi's exit status line,
// in both shell paths (buffered and streamed).
func TestShellExitKeepsOutput(t *testing.T) {
	for _, stream := range []bool{false, true} {
		var onOutput func(string) error
		if stream {
			onOutput = func(string) error { return nil }
		}
		_, err := ExecuteShellOutput(context.Background(), t.TempDir(), goai.ToolCall{Arguments: map[string]any{"command": "echo out-line; echo err-line >&2; exit 3"}}, onOutput)
		var re *ResultError
		if !errors.As(err, &re) || re.Text != "out-line\nerr-line\n\nCommand exited with code 3" {
			t.Fatalf("stream=%v: %#v", stream, err)
		}
		_, err = ExecuteShellOutput(context.Background(), t.TempDir(), goai.ToolCall{Arguments: map[string]any{"command": "exit 1"}}, onOutput)
		if !errors.As(err, &re) || re.Text != "(no output)\n\nCommand exited with code 1" {
			t.Fatalf("stream=%v empty: %#v", stream, err)
		}
	}
	if _, err := ExecuteShellOutput(context.Background(), t.TempDir(), goai.ToolCall{Arguments: map[string]any{"command": "kill -TERM $$"}}, nil); err == nil || err.Error() != "(no output)\n\nCommand exited with code 143" {
		t.Fatalf("signal: %v", err)
	}
}
