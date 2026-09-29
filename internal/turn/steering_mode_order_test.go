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

func TestNativeSteeringModeRequestOrder(t *testing.T) {
	for _, mode := range []string{"one-at-a-time", "all"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			s := openTestStore(t)
			defer s.Close()
			const sessionID = "steering-modes"
			if _, err := s.CreateSession(ctx, sessionID, sessionID, map[string]any{"model": "mock-tool", "status": "idle"}); err != nil {
				t.Fatal(err)
			}
			firstStarted, releaseFirst := make(chan struct{}), make(chan struct{})
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
			turn, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: "first request", Model: "mock-tool"})
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-firstStarted:
			case <-time.After(5 * time.Second):
				t.Fatal("first provider request did not start")
			}
			for _, text := range []string{"steer one", "steer two"} {
				result, err := e.SubmitPrompt(ctx, RunInput{SessionID: sessionID, Prompt: text, Model: "mock-tool", Metadata: map[string]any{"steering_mode": mode}})
				if err != nil || result.Queued || result.TurnID != turn.TurnID {
					t.Fatalf("steering admission %s: %#v %v", text, result, err)
				}
			}
			close(releaseFirst)
			want := [][]string{{"first request"}, {"first request", "steer one"}, {"first request", "steer one", "steer two"}}
			if mode == "all" {
				want = [][]string{{"first request"}, {"first request", "steer one", "steer two"}}
			}
			for i, expected := range want {
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
				return err == nil && record.Status == "completed"
			}, "steering mode turn completion")
			if count.Load() != int32(len(want)) {
				t.Fatalf("provider request count=%d want %d", count.Load(), len(want))
			}
			waitForCondition(t, 5*time.Second, func() bool {
				_, _, err := s.GetSessionActiveTurn(ctx, sessionID)
				return err == sql.ErrNoRows
			}, "steering mode active claim release")
			if queued, err := s.SteeringQueueLength(ctx, sessionID); err != nil || queued != 0 {
				t.Fatalf("pending steering after completion=%d: %v", queued, err)
			}
			if queued, err := s.ListQueuedTurns(ctx, sessionID); err != nil || len(queued) != 0 {
				t.Fatalf("unexpected queued turns after steering: %v: %v", queued, err)
			}
			saved, err := s.ListMessages(ctx, sessionID)
			if err != nil {
				t.Fatal(err)
			}
			var stored []string
			for _, message := range saved {
				if message.Role == "user" {
					stored = append(stored, message.Content)
					if message.Content != "first request" && message.Payload["turn_id"] != turn.TurnID {
						t.Fatalf("steering %q persisted against turn %v, want %s", message.Content, message.Payload["turn_id"], turn.TurnID)
					}
				} else if message.Role == "assistant" && message.Content == "answer" {
					stored = append(stored, "answer")
				}
			}
			wantStored := []string{"first request", "answer", "steer one", "steer two", "answer"}
			if mode == "one-at-a-time" {
				wantStored = []string{"first request", "answer", "steer one", "answer", "steer two", "answer"}
			}
			if !reflect.DeepEqual(stored, wantStored) {
				t.Fatalf("stored message order=%q want %q", stored, wantStored)
			}
		})
	}
}
