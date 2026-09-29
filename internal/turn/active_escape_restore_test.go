package turn

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/inference"
	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

// A foreign writer uses a separate SQLite connection. The runner owns only
// its in-process lock; the transaction must fence all competing DB writes.
// Completion can be paused after the provider has answered but before the
// worker finalizes. A cancellation request must not be overwritten by a
// late successful finalization.
func TestActiveEscapeRestoreWhileCompletionPending(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	const sid = "escape-completion-race"
	if _, err := s.CreateSession(ctx, sid, sid, map[string]any{"model": "mock-tool", "status": "idle"}); err != nil {
		t.Fatal(err)
	}
	withStreamWithToolsStub(t, func(context.Context, string, *goai.Context, func(map[string]any)) (*inference.StreamResult, error) {
		return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "answer"}}}}, nil
	})
	e := New(s)
	defer e.Close()
	entered, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	if _, err := e.RegisterHook(HookTurnEnd, "escape-race", func(context.Context, HookRequest) (HookResponse, error) {
		close(entered)
		<-release
		return HookResponse{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	active, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Prompt: "held", Model: "mock-tool"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("completion hook did not start")
	}
	queued, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Prompt: "queued text", Intent: "queue", Model: "mock-tool"})
	if err != nil || !queued.Queued {
		t.Fatalf("queue=%#v: %v", queued, err)
	}
	draft, err := s.SaveTUITextDraft(ctx, sid, 0, store.TUITextSnapshot{Text: "unsent", Cursor: 6})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.CancelActiveTurn(ctx, sid, active.TurnID); err != nil {
		t.Fatal(err)
	}
	if persisted, err := s.LoadTUITextDraft(ctx, sid); err != nil || persisted != draft {
		t.Fatalf("cancellation changed unsent text=%#v: %v", persisted, err)
	}
	if pending, err := s.GetTurn(ctx, queued.TurnID); err != nil || pending.Status != "queued" {
		t.Fatalf("cancellation changed queued delivery=%#v: %v", pending, err)
	}
	close(release)
	waitForCondition(t, 5*time.Second, func() bool {
		_, _, err := s.GetSessionActiveTurn(ctx, sid)
		return err == sql.ErrNoRows
	}, "completion cleanup")
	turn, err := s.GetTurn(ctx, active.TurnID)
	if err != nil || turn.Status != "cancelled" {
		t.Fatalf("committed abort overwritten by completion: %#v: %v", turn, err)
	}
}

func TestActiveEscapeRestoreFencesAndCancelsBeforeCleanup(t *testing.T) {
	for _, reason := range []string{"text", "draft-writer", "active-steering", "foreign-claim"} {
		t.Run(reason, func(t *testing.T) {
			ctx := context.Background()
			path := t.TempDir() + "/gi.db"
			s, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			other, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			const sid = "escape"
			if _, err := s.CreateSession(ctx, sid, sid, map[string]any{"model": "mock-tool", "status": "idle"}); err != nil {
				t.Fatal(err)
			}
			started, sawCancel, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var closeRelease sync.Once
			defer closeRelease.Do(func() { close(release) })
			var first sync.Once
			withStreamWithToolsStub(t, func(ctx context.Context, _ string, _ *goai.Context, _ func(map[string]any)) (*inference.StreamResult, error) {
				once := false
				first.Do(func() { once = true })
				if !once {
					return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "done"}}}}, nil
				}
				close(started)
				<-ctx.Done()
				close(sawCancel)
				<-release
				return nil, ctx.Err()
			})
			e := New(s)
			defer e.Close()
			active, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Prompt: "held", Model: "mock-tool"})
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("provider not started")
			}
			queued, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Prompt: "follow-up", Intent: "queue", Model: "mock-tool"})
			if err != nil || !queued.Queued {
				t.Fatalf("follow-up=%#v: %v", queued, err)
			}
			draft, err := other.SaveTUITextDraft(ctx, sid, 0, store.TUITextSnapshot{Text: "newer", Cursor: 5})
			if err != nil {
				t.Fatal(err)
			}
			switch reason {
			case "draft-writer":
				if _, err := other.SaveTUITextDraft(ctx, sid, draft.Revision, store.TUITextSnapshot{Text: "new writer", Cursor: 10}); err != nil {
					t.Fatal(err)
				}
			case "active-steering":
				if _, err := other.EnqueueSteering(ctx, sid, "", "user", "steer", nil, nil, "one-at-a-time"); err != nil {
					t.Fatal(err)
				}
			case "foreign-claim":
				if err := other.ReleaseSessionActiveTurn(ctx, sid, active.TurnID); err != nil {
					t.Fatal(err)
				}
				ok, err := other.ClaimSessionActiveTurn(ctx, sid, active.TurnID, "other", "foreign")
				if err != nil || !ok {
					t.Fatalf("foreign claim=%t: %v", ok, err)
				}
			}
			restored, ids, err := e.AbortActiveAndRestoreTUITextDraft(ctx, sid, active.TurnID, draft)
			if reason == "text" {
				if err != nil || len(ids) != 1 || ids[0] != queued.TurnID || restored.Text != "follow-up\n\nnewer" {
					t.Fatalf("restore=%#v ids=%q: %v", restored, ids, err)
				}
				select {
				case <-sawCancel:
				case <-time.After(5 * time.Second):
					t.Fatal("provider not cancelled")
				}
				claim, _, err := other.GetSessionActiveTurn(ctx, sid)
				if err != nil || claim != active.TurnID {
					t.Fatalf("claim dropped before cleanup: %q %v", claim, err)
				}
				row, err := other.GetTurn(ctx, queued.TurnID)
				if err != nil || row.Status != "cancelled" {
					t.Fatalf("follow-up not removed: %#v %v", row, err)
				}
			} else {
				if err == nil || len(ids) != 0 {
					t.Fatalf("conflict signalled unsafe success: %#v ids=%q err=%v", restored, ids, err)
				}
				if reason != "draft-writer" && !errors.Is(err, store.ErrQueueConflict) {
					t.Fatalf("unexpected conflict: %v", err)
				}
				select {
				case <-sawCancel:
					t.Fatal("provider cancelled on failed restore")
				default:
				}
				row, err := other.GetTurn(ctx, active.TurnID)
				if err != nil || row.Status != "running" {
					t.Fatalf("active changed=%#v %v", row, err)
				}
				row, err = other.GetTurn(ctx, queued.TurnID)
				if err != nil || row.Status != "queued" {
					t.Fatalf("follow-up changed=%#v %v", row, err)
				}
				if reason == "foreign-claim" {
					// Restore the engine's claim so ordinary cancellation can clean up.
					if err := other.ReleaseSessionActiveTurn(ctx, sid, "foreign"); err != nil {
						t.Fatal(err)
					}
					ok, err := other.ClaimSessionActiveTurn(ctx, sid, active.TurnID, "runner", active.TurnID)
					if err != nil || !ok {
						t.Fatalf("reclaim=%t %v", ok, err)
					}
				}
				if err := e.CancelActiveTurn(ctx, sid, active.TurnID); err != nil {
					t.Fatal(err)
				}
			}
			closeRelease.Do(func() { close(release) })
			waitForCondition(t, 5*time.Second, func() bool {
				row, err := other.GetTurn(ctx, queued.TurnID)
				_, _, claimErr := other.GetSessionActiveTurn(ctx, sid)
				want := "completed"
				if reason == "text" {
					want = "cancelled"
				}
				return err == nil && row.Status == want && claimErr == sql.ErrNoRows
			}, "cleanup queue handoff")
			messages, err := other.ListMessages(ctx, sid)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, m := range messages {
				if m.Role == "user" && m.Content == "follow-up" {
					count++
				}
			}
			want := 1
			if reason == "text" {
				want = 0
			}
			if count != want {
				t.Fatalf("delivery count=%d want %d", count, want)
			}
		})
	}
}
