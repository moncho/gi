package store

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestRestoreQueuedTUITextDraftAtomic(t *testing.T) {
	ctx := context.Background()
	s, err := Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	const sid = "restore-queue"
	if _, err := s.CreateSession(ctx, sid, sid, nil); err != nil {
		t.Fatal(err)
	}
	draft, err := s.SaveTUITextDraft(ctx, sid, 0, TUITextSnapshot{Text: "newer 中文🙂", Cursor: 3})
	if err != nil {
		t.Fatal(err)
	}
	for i, text := range []string{"steer one", "follow two"} {
		if _, err := s.CreateTurnWithStatus(ctx, string(rune('a'+i)), sid, "queued", text, nil); err != nil {
			t.Fatal(err)
		}
	}
	restored, rows, err := s.RestoreQueuedTUITextDraft(ctx, sid, draft)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rows, []string{"a", "b"}) || restored.Text != "steer one\n\nfollow two\n\nnewer 中文🙂" || restored.Cursor != len([]rune("steer one\n\nfollow two\n\n"))+3 {
		t.Fatalf("restore rows=%q draft=%#v", rows, restored)
	}
	if _, _, err := s.RestoreQueuedTUITextDraft(ctx, sid, draft); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("repeat restore: %v", err)
	}
	queued, err := s.ListQueuedTurns(ctx, sid)
	if err != nil || len(queued) != 0 {
		t.Fatalf("queued=%v: %v", queued, err)
	}
	for _, id := range rows {
		turn, err := s.GetTurn(ctx, id)
		if err != nil || turn.Status != "cancelled" || turn.Phase != "aborted" || turn.FinishedAt == "" {
			t.Fatalf("turn %s=%#v: %v", id, turn, err)
		}
		events, err := s.ListTurnEvents(ctx, id)
		if err != nil || len(events) != 1 || events[0].Type != "turn.cancelled" || events[0].Payload["reason"] != "queue_restore" {
			t.Fatalf("turn %s events=%#v: %v", id, events, err)
		}
	}
	persisted, err := s.LoadTUITextDraft(ctx, sid)
	if err != nil || persisted != restored {
		t.Fatalf("restored journal=%#v want %#v: %v", persisted, restored, err)
	}
	session, err := s.GetSession(ctx, sid)
	if err != nil || session.State["queue_count"] != float64(0) {
		t.Fatalf("queue count after restore=%#v: %v", session, err)
	}
}

func TestRestoreQueuedTUITextDraftFailsClosed(t *testing.T) {
	for _, reason := range []string{"media", "manual-compaction", "pending-media", "claim", "active-steering", "writer", "save", "second-turn-update", "second-turn-event"} {
		t.Run(reason, func(t *testing.T) {
			ctx := context.Background()
			s, err := Open("file::memory:?cache=shared")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			const sid = "restore-queue"
			if _, err := s.CreateSession(ctx, sid, sid, nil); err != nil {
				t.Fatal(err)
			}
			draft, err := s.SaveTUITextDraft(ctx, sid, 0, TUITextSnapshot{Text: "unsent", Cursor: 3})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.CreateTurnWithStatus(ctx, "one", sid, "queued", "one", nil); err != nil {
				t.Fatal(err)
			}
			meta := map[string]any{}
			if reason == "media" {
				meta["media"] = []any{map[string]any{"media_id": "attachment"}}
			}
			if reason == "manual-compaction" {
				meta["operation"] = "manual_compaction"
			}
			if _, err := s.CreateTurnWithStatus(ctx, "two", sid, "queued", "two", meta); err != nil {
				t.Fatal(err)
			}
			switch reason {
			case "pending-media":
				item, err := s.CreateMedia(ctx, sid, "note.txt", "text/plain", []byte("original"), nil)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.StageTUIMedia(ctx, sid, MediaRef{ID: MediaRefID(item.ID), MediaID: item.ID, SessionID: sid}); err != nil {
					t.Fatal(err)
				}
			case "claim":
				if _, err := s.ClaimSessionActiveTurn(ctx, sid, "two", "fixture", "claim"); err != nil {
					t.Fatal(err)
				}
			case "active-steering":
				if _, err := s.EnqueueSteering(ctx, sid, "", "user", "pending steer", nil, nil, "one-at-a-time"); err != nil {
					t.Fatal(err)
				}
			case "writer":
				if _, err := s.SaveTUITextDraft(ctx, sid, draft.Revision, TUITextSnapshot{Text: "new writer", Cursor: 10}); err != nil {
					t.Fatal(err)
				}
			case "save":
				if _, err := s.DB().Exec(`create trigger reject_restore before update on kv_store when old.namespace='tui_text_draft_v1' begin select raise(abort,'injected save failure'); end`); err != nil {
					t.Fatal(err)
				}
			case "second-turn-update":
				if _, err := s.DB().Exec(`create trigger reject_second_restore before update on turns when old.id='two' and new.status='cancelled' begin select raise(abort,'injected cancellation failure'); end`); err != nil {
					t.Fatal(err)
				}
			case "second-turn-event":
				if _, err := s.DB().Exec(`create trigger reject_second_event before insert on turn_events when new.turn_id='two' and new.event_type='turn.cancelled' begin select raise(abort,'injected event failure'); end`); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := s.RestoreQueuedTUITextDraft(ctx, sid, draft); err == nil {
				t.Fatal("restored unsafe queue")
			}
			queued, err := s.ListQueuedTurns(ctx, sid)
			if err != nil || len(queued) != 2 || queued[0].ID != "one" || queued[1].ID != "two" {
				t.Fatalf("queue changed: %v %v", queued, err)
			}
			session, err := s.GetSession(ctx, sid)
			if err != nil || session.State["queue_count"] != float64(2) {
				t.Fatalf("queue count changed: %#v %v", session, err)
			}
			for _, id := range []string{"one", "two"} {
				events, err := s.ListTurnEvents(ctx, id)
				if err != nil || len(events) != 0 {
					t.Fatalf("failed restore wrote event for %s: %#v %v", id, events, err)
				}
			}
			got, err := s.LoadTUITextDraft(ctx, sid)
			if err != nil || (reason != "writer" && got.Text != "unsent") || (reason == "writer" && got.Text != "new writer") {
				t.Fatalf("draft changed: %#v %v", got, err)
			}
		})
	}
}
