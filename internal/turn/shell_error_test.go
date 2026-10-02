package turn

import (
	"context"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/inference"
	goai "github.com/rcarmo/go-ai"
)

// A failing shell command reaches the model like Pi's bash tool result: its
// output, then "Command exited with code N", as an error result.
func TestShellFailureResultKeepsOutput(t *testing.T) {
	s := openTestStore(t)
	e := New(s)
	t.Cleanup(func() { e.Close(); s.Close() })
	ctx := context.Background()
	seen := make(chan string, 1)
	calls := 0
	withStreamWithToolsStub(t, func(_ context.Context, _ string, conv *goai.Context, _ func(map[string]any)) (*inference.StreamResult, error) {
		calls++
		if calls == 1 {
			return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonToolUse,
				Content: []goai.ContentBlock{{Type: "toolCall", ID: "tc_sh", Name: "shell", Arguments: map[string]any{"command": "echo out-line; exit 2"}}}}}, nil
		}
		for _, m := range conv.Messages {
			if m.Role == goai.RoleToolResult && m.ToolCallID == "tc_sh" && len(m.Content) > 0 {
				seen <- m.Content[0].Text
			}
		}
		return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "done"}}}}, nil
	})
	sess, err := s.CreateSession(ctx, "session_shell_fail", "Shell", map[string]any{"status": "idle", "model": "mock-shell"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.SubmitPrompt(ctx, RunInput{SessionID: sess.ID, Prompt: "run it", Model: "mock-shell"}); err != nil {
		t.Fatal(err)
	}
	want := "out-line\n\nCommand exited with code 2"
	select {
	case got := <-seen:
		if got != want {
			t.Fatalf("model saw %q, want %q", got, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("tool result never reached the model")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		msgs, _ := s.ListMessages(ctx, sess.ID)
		for _, m := range msgs {
			if m.Role == "tool_result" && m.Content == want && m.Payload["is_error"] == true {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("stored tool result: %+v", msgs)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
