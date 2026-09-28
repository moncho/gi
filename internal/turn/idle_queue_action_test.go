package turn

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/store"
)

func TestIdleQueueSteerSelectedTurnRetainsMetadata(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	e := New(s)
	defer e.Close()
	s.CreateSession(ctx, "a", "a", nil)
	for _, id := range []string{"first", "selected", "last"} {
		s.CreateTurnWithStatus(ctx, id, "a", "queued", id, map[string]any{"model": "bootstrap", "thinking_level": "medium", "media": []any{map[string]any{"id": 7, "session_id": "a"}}})
	}
	for _, id := range []string{"first", "last"} {
		if err := s.UpdateTurnStatusAndPhase(ctx, id, "queued", "steer_returned"); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.SteerQueuedTurn(ctx, "a", "selected", ""); err != nil {
		t.Fatal(err)
	}
	if err := e.SteerQueuedTurn(ctx, "a", "selected", ""); !errors.Is(err, store.ErrQueueConflict) {
		t.Fatal("duplicate", err)
	}
	waitForCondition(t, 3*time.Second, func() bool { _, _, err := s.GetSessionActiveTurn(ctx, "a"); return errors.Is(err, sql.ErrNoRows) }, "selected cleanup")
	selected, err := s.GetTurn(ctx, "selected")
	if err != nil || selected.Status != "completed" || selected.Metadata["thinking_level"] != "medium" {
		t.Fatal(selected, err)
	}
	for _, id := range []string{"first", "last"} {
		turn, _ := s.GetTurn(ctx, id)
		if turn.Status != "queued" {
			t.Fatal("sibling woke", turn)
		}
	}
	messages, _ := s.ListMessages(ctx, "a")
	n := 0
	for _, m := range messages {
		if m.Role == "user" && m.Content == "selected" {
			n++
			if m.Payload["media"] == nil {
				t.Fatal("media lost", m)
			}
		}
	}
	if n != 1 {
		t.Fatal("source replay or missing", messages)
	}
}

func TestIdleQueueSteerLaunchFailureRestoresOriginalPhase(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	e := New(s)
	defer e.Close()
	s.CreateSession(ctx, "a", "a", nil)
	s.CreateTurnWithStatus(ctx, "q", "a", "queued", "original", map[string]any{"model": "bootstrap"})
	s.UpdateTurnStatusAndPhase(ctx, "q", "queued", "steer_returned")
	e.beforeLaunchSessionStateErrorHook = func(context.Context, string, string) error { return errors.New("injected persistence fault") }
	if err := e.SteerQueuedTurn(ctx, "a", "q", ""); err == nil {
		t.Fatal("accepted failed launch")
	}
	q, _ := s.GetTurn(ctx, "q")
	if q.Status != "queued" || q.Phase != "steer_returned" {
		t.Fatal(q)
	}
	if _, _, err := s.GetSessionActiveTurn(ctx, "a"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	if msgs, _ := s.ListMessages(ctx, "a"); len(msgs) != 0 {
		t.Fatal(msgs)
	}
	e.beforeLaunchSessionStateErrorHook = nil
	if err := e.SteerQueuedTurn(ctx, "a", "q", ""); err != nil {
		t.Fatal(err)
	}
	waitForCondition(t, 3*time.Second, func() bool { q, _ := s.GetTurn(ctx, "q"); return q.Status == "completed" }, "explicit retry")
}

func TestIdleQueueSteerFencesRacedClaimAndForeignRow(t *testing.T) {
	for _, race := range []string{"claim", "cancel", "foreign"} {
		t.Run(race, func(t *testing.T) {
			ctx := context.Background()
			s := openTestStore(t)
			defer s.Close()
			e := New(s)
			defer e.Close()
			s.CreateSession(ctx, "a", "a", nil)
			s.CreateSession(ctx, "b", "b", nil)
			session := "a"
			if race == "foreign" {
				session = "b"
			}
			s.CreateTurnWithStatus(ctx, "q", session, "queued", "original", map[string]any{"model": "bootstrap"})
			e.beforeLaunchClaimHook = func(ctx context.Context, _, _ string) {
				switch race {
				case "claim":
					s.CreateTurnWithStatus(ctx, "new", "a", "running", "new", nil)
					s.ClaimSessionActiveTurn(ctx, "a", "new", "other", "new")
				case "cancel":
					s.CancelQueuedTurn(ctx, "a", "q")
				}
			}
			if err := e.SteerQueuedTurn(ctx, "a", "q", ""); !errors.Is(err, store.ErrQueueConflict) {
				t.Fatal(err)
			}
			msgs, _ := s.ListMessages(ctx, "a")
			if len(msgs) > 0 {
				t.Fatal(msgs)
			}
		})
	}
}

func TestIdleQueueSteerStorageFailureLeavesItemRetryable(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	e := New(s)
	defer e.Close()
	s.CreateSession(ctx, "a", "a", nil)
	s.CreateTurnWithStatus(ctx, "q", "a", "queued", "original", map[string]any{"model": "bootstrap"})
	if _, err := s.DB().Exec(`create trigger reject_idle_prompt before insert on messages when json_extract(NEW.payload_json,'$.idle_queue_action')=1 begin select raise(abort,'message persistence fault');end`); err != nil {
		t.Fatal(err)
	}
	if err := e.SteerQueuedTurn(ctx, "a", "q", ""); err == nil {
		t.Fatal("accepted without storage")
	}
	q, _ := s.GetTurn(ctx, "q")
	if q.Status != "queued" {
		t.Fatal(q)
	}
	if _, _, err := s.GetSessionActiveTurn(ctx, "a"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	msgs, _ := s.ListMessages(ctx, "a")
	if len(msgs) > 0 {
		t.Fatal(msgs)
	}
	s.DB().Exec(`drop trigger reject_idle_prompt`)
	if err := e.SteerQueuedTurn(ctx, "a", "q", ""); err != nil {
		t.Fatal(err)
	}
	waitForCondition(t, 3*time.Second, func() bool { q, _ := s.GetTurn(ctx, "q"); return q.Status == "completed" }, "retry completion")
	msgs, _ = s.ListMessages(ctx, "a")
	n := 0
	for _, m := range msgs {
		if m.Role == "user" {
			n++
		}
	}
	if n != 1 {
		t.Fatal(msgs)
	}
}
