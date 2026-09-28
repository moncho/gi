package turn

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/store"
)

// Stop matches installed Piclaw: abort the active run and let queued work continue.
func TestWebQueueHoldStopDoesNotPauseFIFO(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	e := New(s)
	defer e.Close()
	if _, err := s.CreateSession(ctx, "a", "test", nil); err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(gate) })
	e.beforeSetupHook = func(ctx context.Context, _, id string) {
		select {
		case <-ctx.Done():
		case <-gate:
		}
	}
	first, err := e.SubmitPrompt(ctx, RunInput{SessionID: "a", Prompt: "active", Model: "bootstrap"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.SubmitPrompt(ctx, RunInput{SessionID: "a", Prompt: "queued second", Model: "bootstrap", Intent: "queue"})
	if err != nil || !second.Queued {
		t.Fatal(second, err)
	}
	third, err := e.SubmitPrompt(ctx, RunInput{SessionID: "a", Prompt: "queued third", Model: "bootstrap", Intent: "queue"})
	if err != nil || !third.Queued {
		t.Fatal(third, err)
	}
	if err := e.StopWebActiveTurn(ctx, "a", first.TurnID); err != nil {
		t.Fatal(err)
	}
	waitForCondition(t, 3*time.Second, func() bool { active, _, _ := s.GetSessionActiveTurn(ctx, "a"); return active == second.TurnID }, "queue automatically advanced")
	if err := e.StopWebActiveTurn(ctx, "a", first.TurnID); !errors.Is(err, store.ErrQueueConflict) {
		t.Fatal("stale Stop affected successor", err)
	}
	once.Do(func() { close(gate) })
	waitForCondition(t, 3*time.Second, func() bool {
		a, _ := s.GetTurn(ctx, second.TurnID)
		b, _ := s.GetTurn(ctx, third.TurnID)
		return a.Status == "completed" && b.Status == "completed"
	}, "FIFO completion")
	msgs, err := s.ListMessages(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	var users []string
	for _, m := range msgs {
		if m.Role == "user" {
			users = append(users, m.Content)
		}
	}
	if len(users) != 2 || users[0] != "queued second" || users[1] != "queued third" {
		t.Fatal(users)
	}
}

func TestWebQueueHoldLegacyCrashRecoveryClearsObsoleteHold(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "hold.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateSession(ctx, "held", "held", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateTurnWithStatus(ctx, "stopped", "held", "running", "old", nil); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimSessionActiveTurn(ctx, "held", "stopped", "dead", "stopped"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if _, err = s.CreateTurnWithStatus(ctx, "next", "held", "queued", "preserve", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateTurnStatusAndPhase(ctx, "stopped", "cancelling", "cancelling"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`create table web_queue_holds(session_id text primary key,stop_turn_id text,created_at text);insert into web_queue_holds values('held','stopped',datetime('now'))`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB().Exec(`update session_active_turns set updated_at='2000-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e := New(s)
	defer e.Close()
	if _, err = e.recoverInterruptedTurns(ctx, "held"); err != nil {
		t.Fatal(err)
	}
	waitForCondition(t, 3*time.Second, func() bool { next, _ := s.GetTurn(ctx, "next"); return next.Status == "completed" }, "legacy held queue automatically continues")
	stopped, _ := s.GetTurn(ctx, "stopped")
	if stopped.Status != "aborted" {
		t.Fatal(stopped)
	}

}

func TestWebQueueHoldLegacyIdleQueueAutomaticallyStartsOnUpgrade(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "idle-held.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateSession(ctx, "a", "test", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateTurnWithStatus(ctx, "q", "a", "queued", "legacy queued prompt", map[string]any{"model": "bootstrap"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB().Exec(`create table web_queue_holds(session_id text primary key,stop_turn_id text,created_at text);insert into web_queue_holds values('a','finished',datetime('now'))`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e := New(s)
	defer e.Close()
	waitForCondition(t, 3*time.Second, func() bool { q, _ := s.GetTurn(ctx, "q"); return q.Status == "completed" }, "idle legacy queue automatically continues")
	ids, err := s.LegacyStopQueueHandoffs(ctx)
	if err != nil || len(ids) != 0 {
		t.Fatal(ids, err)
	}
	msgs, err := s.ListMessages(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	users := 0
	for _, m := range msgs {
		if m.Role == "user" && m.Content == "legacy queued prompt" {
			users++
		}
	}
	if users != 1 {
		t.Fatal(msgs)
	}
}
