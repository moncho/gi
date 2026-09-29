package turn

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/inference"
	goai "github.com/rcarmo/go-ai"
)

// A direct assistant reply remains visible before steering queued while the
// provider is answering. The next request sees both in their original order.
func TestNativeDirectReplySteeringPreservesVisibleAssistant(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	const sessionID = "direct-reply-steering"
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
			close(firstStarted)
			select {
			case <-releaseFirst:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "first visible answer"}}}}, nil
		case 2:
			secondRequest <- append([]goai.Message(nil), conv.Messages...)
			return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "second answer"}}}}, nil
		default:
			return nil, context.Canceled
		}
	})
	e := New(s)
	defer e.Close()
	turn, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "first request", Model: "mock-tool"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("provider request did not start")
	}
	steered, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "steer after direct reply", Model: "mock-tool"})
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
		if message.Role == goai.RoleAssistant && text == "first visible answer" || message.Role == goai.RoleUser && text == "steer after direct reply" {
			sequence = append(sequence, text)
		}
	}
	want := []string{"first visible answer", "steer after direct reply"}
	if strings.Join(sequence, "|") != strings.Join(want, "|") {
		t.Fatalf("second request order: %q want %q", sequence, want)
	}
	waitForCondition(t, 5*time.Second, func() bool {
		record, err := s.GetTurn(ctx, turn.TurnID)
		return err == nil && record.Status == "completed"
	}, "completed direct-reply turn")
	saved, err := s.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	var stored []string
	for _, message := range saved {
		if message.Role == "assistant" && message.Content == "first visible answer" || message.Role == "user" && message.Content == "steer after direct reply" {
			stored = append(stored, message.Content)
		}
	}
	if strings.Join(stored, "|") != strings.Join(want, "|") || requests.Load() != 2 {
		t.Fatalf("stored direct reply/steering: %q requests=%d", stored, requests.Load())
	}
}

func TestNativeDirectReplySteeringFailsClosedOnAssistantWrite(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	const sessionID = "direct-reply-write-failure"
	if _, err := s.CreateSession(ctx, sessionID, sessionID, map[string]any{"model": "mock-tool", "status": "idle"}); err != nil {
		t.Fatal(err)
	}
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var requests atomic.Int32
	continuationRequest := make(chan []goai.Message, 1)
	withStreamWithToolsStub(t, func(ctx context.Context, _ string, conv *goai.Context, _ func(map[string]any)) (*inference.StreamResult, error) {
		if requests.Add(1) == 1 {
			close(firstStarted)
		} else {
			continuationRequest <- append([]goai.Message(nil), conv.Messages...)
		}
		select {
		case <-releaseFirst:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "answer not stored"}}}}, nil
	})
	e := New(s)
	defer e.Close()
	turn, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "first request", Model: "mock-tool"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("provider request did not start")
	}
	steered, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "steer after direct reply", Model: "mock-tool"})
	if err != nil || steered.Queued || steered.TurnID != turn.TurnID {
		t.Fatalf("active steering admission: %#v %v", steered, err)
	}
	if _, err := s.DB().Exec(`create trigger reject_assistant_response before insert on messages when new.role='assistant' begin select raise(abort,'fixture assistant write failure'); end`); err != nil {
		t.Fatal(err)
	}
	close(releaseFirst)
	waitForCondition(t, 5*time.Second, func() bool {
		record, err := s.GetTurn(ctx, turn.TurnID)
		return err == nil && record.Status == "failed"
	}, "assistant persistence failure")
	var next []goai.Message
	select {
	case next = <-continuationRequest:
	case <-time.After(5 * time.Second):
		t.Fatal("queued steering continuation did not start")
	}
	for _, message := range next {
		if message.Role == goai.RoleAssistant && goai.GetTextContent(&message) == "answer not stored" {
			t.Fatal("failed assistant write leaked into continuation request")
		}
	}
	turns, err := s.ListTurns(ctx, sessionID)
	if err != nil || len(turns) != 2 || turns[0].ID != turn.TurnID || turns[1].ID == turn.TurnID {
		t.Fatalf("steering continuation ownership: %#v, %v", turns, err)
	}
	messages, err := s.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		if message.Role == "assistant" && message.Content == "answer not stored" {
			t.Fatalf("phantom answer after failure: %#v", messages)
		}
	}
	if requests.Load() != 2 {
		t.Fatalf("expected one failed run and one owned continuation: %d requests", requests.Load())
	}
}
