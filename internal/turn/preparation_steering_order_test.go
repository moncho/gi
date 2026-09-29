package turn

import (
	"context"
	"database/sql"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/inference"
	goai "github.com/rcarmo/go-ai"
)

// A message already selected at the previous response boundary must reach the
// second request even if another arrives while its context is being prepared.
func TestNativeSteeringDuringNextRequestPreparation(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	const sessionID = "steering-during-preparation"
	if _, err := s.CreateSession(ctx, sessionID, sessionID, map[string]any{"model": "mock-tool", "status": "idle"}); err != nil {
		t.Fatal(err)
	}
	firstStarted, releaseFirst := make(chan struct{}), make(chan struct{})
	preparing, releasePreparation := make(chan struct{}), make(chan struct{})
	requests := make(chan []string, 3)
	var count atomic.Int32
	withStreamWithToolsStub(t, func(ctx context.Context, _ string, conv *goai.Context, _ func(map[string]any)) (*inference.StreamResult, error) {
		index := int(count.Add(1))
		var users []string
		for _, message := range conv.Messages {
			if message.Role == goai.RoleUser {
				users = append(users, goai.GetTextContent(&message))
			}
		}
		requests <- users
		if index == 1 {
			close(firstStarted)
			select {
			case <-releaseFirst:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "answer"}}}}, nil
	})
	e := New(s)
	defer e.Close()
	if _, err := e.RegisterHook(HookContext, "preparation-gate", func(ctx context.Context, req HookRequest) (HookResponse, error) {
		if req.Iteration == 2 {
			close(preparing)
			select {
			case <-releasePreparation:
			case <-ctx.Done():
				return HookResponse{}, ctx.Err()
			}
		}
		return HookResponse{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	turn, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "first request", Model: "mock-tool"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("first provider request did not start")
	}
	first, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "steer one", Model: "mock-tool"})
	if err != nil || first.Queued || first.TurnID != turn.TurnID {
		t.Fatalf("first steering admission: %#v %v", first, err)
	}
	close(releaseFirst)
	select {
	case <-preparing:
	case <-time.After(5 * time.Second):
		t.Fatal("second iteration preparation did not start")
	}
	second, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "steer two", Model: "mock-tool"})
	if err != nil || second.Queued || second.TurnID != turn.TurnID {
		t.Fatalf("second steering admission: %#v %v", second, err)
	}
	close(releasePreparation)
	for i, expected := range [][]string{{"first request"}, {"first request", "steer one"}, {"first request", "steer one", "steer two"}} {
		select {
		case observed := <-requests:
			if !reflect.DeepEqual(observed, expected) {
				t.Fatalf("request %d users=%q want %q", i+1, observed, expected)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("request %d missing", i+1)
		}
	}
	waitForCondition(t, 5*time.Second, func() bool {
		record, err := s.GetTurn(ctx, turn.TurnID)
		if err != nil || record.Status != "completed" {
			return false
		}
		_, _, err = s.GetSessionActiveTurn(ctx, sessionID)
		return err == sql.ErrNoRows
	}, "prepared steering turn completion and release")
	if count.Load() != 3 {
		t.Fatalf("provider request count=%d want 3", count.Load())
	}
	if queued, err := s.SteeringQueueLength(ctx, sessionID); err != nil || queued != 0 {
		t.Fatalf("pending steering after completion=%d: %v", queued, err)
	}
	saved, err := s.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	var sequence []string
	for _, message := range saved {
		if message.Role == "user" {
			sequence = append(sequence, message.Content)
			if message.Content != "first request" && message.Payload["turn_id"] != turn.TurnID {
				t.Fatalf("steering %q persisted against turn %v, want %s", message.Content, message.Payload["turn_id"], turn.TurnID)
			}
		} else if message.Role == "assistant" && message.Content == "answer" {
			sequence = append(sequence, "answer")
		}
	}
	want := []string{"first request", "answer", "steer one", "answer", "steer two", "answer"}
	if !reflect.DeepEqual(sequence, want) {
		t.Fatalf("stored message order=%q want %q", sequence, want)
	}
}
