package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestAbortActiveTurnAndRestoreQueuedTUITextDraft(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail bool
	}{
		{"text", false}, {"no-queue", false}, {"foreign-claim", true},
		{"draft-writer", true}, {"queued-media", true}, {"active-media", true},
		{"steering", true}, {"dequeued-steering", true}, {"pending-media", true}, {"event-failure", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			path := t.TempDir() + "/escape.db"
			s, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			other, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			const sid = "escape"
			if _, err := s.CreateSession(ctx, sid, sid, nil); err != nil {
				t.Fatal(err)
			}
			metadata := map[string]any{}
			if tc.name == "active-media" {
				metadata["media"] = []any{"media:7"}
			}
			if _, err := s.CreateTurnWithStatus(ctx, "active", sid, "running", "held", metadata); err != nil {
				t.Fatal(err)
			}
			token := "active"
			if tc.name == "foreign-claim" {
				token = "other-worker"
			}
			claimed, err := s.ClaimSessionActiveTurn(ctx, sid, "active", "fixture", token)
			if err != nil || !claimed {
				t.Fatalf("claim=%t: %v", claimed, err)
			}
			if tc.name != "no-queue" {
				metadata = map[string]any{}
				if tc.name == "queued-media" {
					metadata["media"] = []any{"media:8"}
				}
				if _, err := s.CreateTurnWithStatus(ctx, "one", sid, "queued", "first", metadata); err != nil {
					t.Fatal(err)
				}
				if _, err := s.CreateTurnWithStatus(ctx, "two", sid, "queued", "second", nil); err != nil {
					t.Fatal(err)
				}
			}
			draft, err := s.SaveTUITextDraft(ctx, sid, 0, TUITextSnapshot{Text: "中文🙂 draft", Cursor: 3})
			if err != nil {
				t.Fatal(err)
			}
			switch tc.name {
			case "draft-writer":
				if _, err := other.SaveTUITextDraft(ctx, sid, draft.Revision, TUITextSnapshot{Text: "other writer", Cursor: 6}); err != nil {
					t.Fatal(err)
				}
			case "steering":
				if _, err := other.EnqueueSteering(ctx, sid, "", "user", "steer", nil, nil, "one-at-a-time"); err != nil {
					t.Fatal(err)
				}
			case "dequeued-steering":
				if _, err := other.EnqueueSteering(ctx, sid, "active", "user", "already injected", nil, nil, "one-at-a-time"); err != nil {
					t.Fatal(err)
				}
				if _, err := other.DB().ExecContext(ctx, `update steering_queue set status='dequeued' where session_id=? and turn_id='active'`, sid); err != nil {
					t.Fatal(err)
				}
			case "pending-media":
				media, err := other.CreateMedia(ctx, sid, "a.txt", "text/plain", []byte("bytes"), nil)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := other.StageTUIMedia(ctx, sid, MediaRef{ID: MediaRefID(media.ID), MediaID: media.ID, SessionID: sid}); err != nil {
					t.Fatal(err)
				}
			case "event-failure":
				if _, err := s.DB().Exec(`create trigger fail_abort before insert on turn_events when new.turn_id='active' and new.event_type='turn.cancelling' begin select raise(abort,'injected'); end`); err != nil {
					t.Fatal(err)
				}
			}
			updated, ids, err := s.AbortActiveTurnAndRestoreQueuedTUITextDraft(ctx, sid, "active", "active", draft)
			if tc.fail {
				if err == nil || len(ids) != 0 {
					t.Fatalf("unsafe restore=%#v ids=%q err=%v", updated, ids, err)
				}
				if tc.name != "event-failure" && !errors.Is(err, ErrQueueConflict) && !errors.Is(err, ErrTUIDraftConflict) && !errors.Is(err, ErrTUIDraftHeld) {
					t.Fatalf("unexpected conflict: %v", err)
				}
			} else {
				wantIDs := []string{"one", "two"}
				wantText := "first\n\nsecond\n\n中文🙂 draft"
				wantCursor := len([]rune("first\n\nsecond\n\n")) + 3
				if tc.name == "no-queue" {
					wantIDs = nil
					wantText = draft.Text
					wantCursor = draft.Cursor
				}
				if err != nil || !reflect.DeepEqual(ids, wantIDs) || updated.Text != wantText || updated.Cursor != wantCursor {
					t.Fatalf("restore=%#v ids=%q err=%v", updated, ids, err)
				}
			}
			active, err := other.GetTurn(ctx, "active")
			if err != nil || active.Status != map[bool]string{true: "running", false: "cancelling"}[tc.fail] {
				t.Fatalf("active=%#v: %v", active, err)
			}
			turnEvents, err := other.ListTurnEvents(ctx, "active")
			if err != nil || len(turnEvents) != map[bool]int{true: 0, false: 1}[tc.fail] {
				t.Fatalf("active events=%#v: %v", turnEvents, err)
			}
			for _, id := range []string{"one", "two"} {
				if tc.name == "no-queue" {
					break
				}
				row, err := other.GetTurn(ctx, id)
				if err != nil || row.Status != map[bool]string{true: "queued", false: "cancelled"}[tc.fail] {
					t.Fatalf("queued %s=%#v: %v", id, row, err)
				}
			}
			stored, err := other.LoadTUITextDraft(ctx, sid)
			if err != nil {
				t.Fatal(err)
			}
			if tc.fail && tc.name != "draft-writer" && stored != draft {
				t.Fatalf("failed restore changed draft=%#v", stored)
			}
			if !tc.fail && stored != updated {
				t.Fatalf("draft not durable=%#v", stored)
			}
			claim, _, err := other.GetSessionActiveTurn(ctx, sid)
			if err != nil || claim != "active" {
				t.Fatalf("claim changed=%q: %v", claim, err)
			}
		})
	}
}
