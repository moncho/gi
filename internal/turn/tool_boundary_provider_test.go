package turn

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/inference"
	"github.com/rcarmo/gi/internal/tools"
	goai "github.com/rcarmo/go-ai"
)

// The stub replaces only the provider stream. Admission, the two tool calls,
// durable results, steering injection and the next request use the Go engine.
func TestNativeProviderToolBatchSteeringRequestOrder(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	const sessionID = "provider-tool-boundary"
	if _, err := s.CreateSession(ctx, sessionID, sessionID, map[string]any{"model": "mock-tool", "status": "idle"}); err != nil {
		t.Fatal(err)
	}
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondRequest := make(chan []goai.Message, 1)
	var requests atomic.Int32
	withStreamWithToolsStub(t, func(ctx context.Context, _ string, conv *goai.Context, _ func(map[string]any)) (*inference.StreamResult, error) {
		switch requests.Add(1) {
		case 1:
			return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonToolUse, Content: []goai.ContentBlock{
				{Type: "toolCall", ID: "first", Name: "first_boundary", Arguments: map[string]any{}},
				{Type: "toolCall", ID: "second", Name: "second_boundary", Arguments: map[string]any{}},
			}}}, nil
		case 2:
			secondRequest <- append([]goai.Message(nil), conv.Messages...)
			return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "done"}}}}, nil
		default:
			return nil, context.Canceled
		}
	})
	e := New(s)
	defer e.Close()
	for _, name := range []string{"first_boundary", "second_boundary"} {
		name := name
		if err := e.RegisterTool(tools.RegisteredTool{Name: name, Description: name, Executor: func(ctx context.Context, _ tools.ToolRuntime, _ goai.ToolCall) (string, error) {
			if name == "first_boundary" {
				close(firstStarted)
				select {
				case <-releaseFirst:
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}
			return "result " + name, nil
		}}); err != nil {
			t.Fatal(err)
		}
	}
	turn, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "first request", Model: "mock-tool"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstStarted:
	case <-time.After(5 * time.Second):
		record, _ := s.GetTurn(ctx, turn.TurnID)
		events, _ := s.ListTurnEvents(ctx, turn.TurnID)
		messages, _ := s.ListMessages(ctx, sessionID)
		t.Fatalf("first tool did not start; turn=%#v requests=%d events=%#v messages=%#v", record, requests.Load(), events, messages)
	}
	steered, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "steer at tool boundary", Model: "mock-tool"})
	if err != nil || steered.Queued || steered.TurnID != turn.TurnID {
		t.Fatalf("active steering admission: %#v %v", steered, err)
	}
	close(releaseFirst)
	var messages []goai.Message
	select {
	case messages = <-secondRequest:
	case <-time.After(5 * time.Second):
		t.Fatal("second provider request missing")
	}
	var sequence []string
	for _, message := range messages {
		text := goai.GetTextContent(&message)
		if message.Role == goai.RoleToolResult && strings.HasPrefix(text, "result ") || message.Role == goai.RoleUser && text == "steer at tool boundary" {
			sequence = append(sequence, text)
		}
	}
	want := []string{"result first_boundary", "result second_boundary", "steer at tool boundary"}
	if len(sequence) != len(want) || strings.Join(sequence, "|") != strings.Join(want, "|") {
		t.Fatalf("second model request order: %q want %q", sequence, want)
	}
	waitForCondition(t, 5*time.Second, func() bool {
		record, err := s.GetTurn(ctx, turn.TurnID)
		return err == nil && record.Status == "completed"
	}, "completed native provider turn")
	if requests.Load() != 2 {
		t.Fatalf("unexpected provider requests: %d", requests.Load())
	}
	saved, err := s.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	var storedSequence []string
	for _, message := range saved {
		if message.Role == "tool_result" && strings.HasPrefix(message.Content, "result ") || message.Role == "user" && message.Content == "steer at tool boundary" {
			storedSequence = append(storedSequence, message.Content)
		}
	}
	if strings.Join(storedSequence, "|") != strings.Join(want, "|") {
		t.Fatalf("stored tool and steering order: %q want %q", storedSequence, want)
	}
	events, err := s.ListTurnEvents(ctx, turn.TurnID)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Type == "tool.skipped" {
			t.Fatalf("announced tool skipped: %#v", event)
		}
	}
}
