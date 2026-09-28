package turn

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/store"
)

func seedEndedQueueSteer(t *testing.T, s *store.Store) {
	t.Helper()
	ctx := context.Background()
	if _, err := s.CreateSession(ctx, "a", "a", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, "old", "a", "running", "old", nil); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimSessionActiveTurn(ctx, "a", "old", "runner", "old"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := s.UpdateTurnStatusAndPhase(ctx, "old", "completed", "completed"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReleaseSessionActiveTurn(ctx, "a", "old"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, "q", "a", "queued", "selected", map[string]any{"model": "bootstrap", "thinking_level": "medium", "media": []any{map[string]any{"id": 7, "session_id": "a"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestQueueSteerEndedRunLaunchesSameRowOnce(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	e := New(s)
	defer e.Close()
	seedEndedQueueSteer(t, s)
	if err := e.SteerQueuedTurn(ctx, "a", "q", "old"); err != nil {
		t.Fatal(err)
	}
	if err := e.SteerQueuedTurn(ctx, "a", "q", "old"); !errors.Is(err, store.ErrQueueConflict) {
		t.Fatal("duplicate", err)
	}
	waitForCondition(t, 3*time.Second, func() bool { _, _, err := s.GetSessionActiveTurn(ctx, "a"); return errors.Is(err, sql.ErrNoRows) }, "fallback cleanup")
	q, err := s.GetTurn(ctx, "q")
	if err != nil || q.Status != "completed" || q.Metadata["thinking_level"] != "medium" {
		t.Fatal(q, err)
	}
	messages, err := s.ListMessages(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, m := range messages {
		if m.Role == "user" {
			n++
			if m.Content != "selected" || m.Payload["turn_id"] != "q" || m.Payload["media"] == nil {
				t.Fatal(m)
			}
		}
	}
	if n != 1 {
		t.Fatal("missing/replayed prompt", messages)
	}
}

func TestQueueSteerEndedRunFencesClaimAndReturnedRows(t *testing.T) {
	for _, race := range []string{"replacement", "replacement-ended", "cancel", "foreign", "returned", "unknown", "claimed", "compaction"} {
		t.Run(race, func(t *testing.T) {
			ctx := context.Background()
			s := openTestStore(t)
			defer s.Close()
			e := New(s)
			defer e.Close()
			seedEndedQueueSteer(t, s)
			observed := "old"
			if race == "unknown" {
				observed = "missing"
			}
			e.beforeLaunchClaimHook = func(ctx context.Context, _, _ string) {
				switch race {
				case "replacement", "replacement-ended":
					if _, err := s.CreateTurnWithStatus(ctx, "new", "a", "running", "new", nil); err != nil {
						t.Fatal(err)
					}
					if ok, err := s.ClaimSessionActiveTurn(ctx, "a", "new", "other", "new"); err != nil || !ok {
						t.Fatal(ok, err)
					}
					if race == "replacement-ended" {
						s.UpdateTurnStatusAndPhase(ctx, "new", "completed", "completed")
						s.ReleaseSessionActiveTurn(ctx, "a", "new")
					}
				case "cancel":
					if err := s.CancelQueuedTurn(ctx, "a", "q"); err != nil {
						t.Fatal(err)
					}
				case "foreign":
					s.CreateSession(ctx, "b", "b", nil)
					if _, err := s.DB().Exec(`update turns set session_id='b' where id='q'`); err != nil {
						t.Fatal(err)
					}
				case "returned":
					s.UpdateTurnStatusAndPhase(ctx, "q", "queued", "steer_returned")
				case "claimed":
					if err := s.MarkTurnClaimed(ctx, "q", "other"); err != nil {
						t.Fatal(err)
					}
				case "compaction":
					if _, err := s.DB().Exec(`update turns set metadata_json=json_set(metadata_json,'$.operation','manual_compaction') where id='q'`); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := e.SteerQueuedTurn(ctx, "a", "q", observed); !errors.Is(err, store.ErrQueueConflict) {
				t.Fatal("accepted", race, err)
			}
			if messages, err := s.ListMessages(ctx, "a"); err != nil || len(messages) > 0 {
				t.Fatal(messages, err)
			}
		})
	}
}

func TestQueueSteerEndedRunPersistenceFailureCanRetry(t *testing.T) {
	for _, fault := range []string{"launch", "prompt"} {
		t.Run(fault, func(t *testing.T) {
			ctx := context.Background()
			s := openTestStore(t)
			defer s.Close()
			e := New(s)
			defer e.Close()
			seedEndedQueueSteer(t, s)
			if fault == "launch" {
				e.beforeLaunchSessionStateErrorHook = func(context.Context, string, string) error { return errors.New("injected launch failure") }
			} else {
				if _, err := s.DB().Exec(`create trigger fail_ended_prompt before insert on messages begin select raise(abort,'injected persistence failure');end`); err != nil {
					t.Fatal(err)
				}
			}
			if err := e.SteerQueuedTurn(ctx, "a", "q", "old"); err == nil {
				t.Fatal("accepted failed launch")
			}
			q, _ := s.GetTurn(ctx, "q")
			if q.Status != "queued" || q.ClaimedBy != "" {
				t.Fatal(q)
			}
			var last string
			if err := s.DB().QueryRow(`select turn_id from session_last_run where session_id='a'`).Scan(&last); err != nil || last != "old" {
				t.Fatal(last, err)
			}
			if msgs, err := s.ListMessages(ctx, "a"); err != nil || len(msgs) != 0 {
				t.Fatal(msgs, err)
			}
			e.beforeLaunchSessionStateErrorHook = nil
			s.DB().Exec(`drop trigger if exists fail_ended_prompt`)
			if err := e.SteerQueuedTurn(ctx, "a", "q", "old"); err != nil {
				t.Fatal(err)
			}
			waitForCondition(t, 3*time.Second, func() bool { q, _ := s.GetTurn(ctx, "q"); return q.Status == "completed" }, "retry completion")
		})
	}
}
