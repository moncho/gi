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

// Stop while the first provider request is in flight. Queued steering must
// survive the aborted turn and run in a cleanup-owned continuation, not leak
// into the aborted request or be silently discarded.
func TestNativeAbortRetainsSteeringAcrossRequestBoundaries(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	const sessionID = "abort-steering-order"
	if _, err := s.CreateSession(ctx, sessionID, sessionID, map[string]any{"model": "mock-tool", "status": "idle"}); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
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
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "answer"}}}}, nil
	})
	e := New(s)
	defer e.Close()
	first, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "first request", Model: "mock-tool"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("first provider request did not start")
	}
	for _, text := range []string{"steer one", "steer two"} {
		steered, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: text, Model: "mock-tool"})
		if err != nil || steered.Queued || steered.TurnID != first.TurnID {
			t.Fatalf("steering admission %s: %#v %v", text, steered, err)
		}
	}
	if err := e.StopWebActiveTurn(ctx, sessionID, first.TurnID); err != nil {
		t.Fatal(err)
	}
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
		turn, err := s.GetTurn(ctx, first.TurnID)
		if err != nil || turn.Status != "cancelled" {
			return false
		}
		_, _, err = s.GetSessionActiveTurn(ctx, sessionID)
		return err == sql.ErrNoRows
	}, "aborted turn and continuation cleanup")
	if count.Load() != 3 {
		t.Fatalf("provider request count=%d want 3", count.Load())
	}
	if queued, err := s.SteeringQueueLength(ctx, sessionID); err != nil || queued != 0 {
		t.Fatalf("pending steering after completion=%d: %v", queued, err)
	}
	stored, err := s.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	var users []string
	var continuation string
	for _, message := range stored {
		if message.Role == "user" {
			users = append(users, message.Content)
			if message.Content == "steer one" || message.Content == "steer two" {
				turnID, _ := message.Payload["turn_id"].(string)
				if turnID == "" || turnID == first.TurnID {
					t.Fatalf("steering %q persisted on aborted turn %q", message.Content, turnID)
				}
				if continuation != "" && continuation != turnID {
					t.Fatalf("steering messages have separate continuation turns: %q / %q", continuation, turnID)
				}
				continuation = turnID
			}
		}
	}
	if !reflect.DeepEqual(users, []string{"first request", "steer one", "steer two"}) || continuation == "" {
		t.Fatalf("persisted steering=%q continuation=%q", users, continuation)
	}
	turn, err := s.GetTurn(ctx, continuation)
	if err != nil || turn.Status != "completed" {
		t.Fatalf("continuation=%#v: %v", turn, err)
	}
}
