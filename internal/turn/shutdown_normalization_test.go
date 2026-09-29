package turn

import (
	"context"
	"errors"
	"testing"

	"github.com/rcarmo/gi/internal/store"
)

// Cleanup can reach normalization after Engine.Close has cancelled its
// background context. CoordinationContext then returns nil; never pass it to
// database/sql (which panics before it can report a closed database).
func TestNormalizeSessionStateAfterEngineCloseDoesNotUseNilContext(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	const sid = "closed-engine-normalization"
	if _, err := s.CreateSession(ctx, sid, sid, map[string]any{"status": "running", "active_turn_id": "held"}); err != nil {
		t.Fatal(err)
	}
	e := New(s)
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	caller, cancel := context.WithCancel(ctx)
	cancel()
	if got := store.CoordinationContext(caller, e.backgroundContext()); got != nil {
		t.Fatalf("expected no live coordination context, got %v", got)
	}
	if err := e.normalizeInactiveSessionState(caller, sid, "idle", "", false); !errors.Is(err, context.Canceled) {
		t.Fatalf("inactive normalization after close = %v", err)
	}
	if err := e.normalizeRunningSessionState(caller, sid, "held", false, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("running normalization after close = %v", err)
	}
	current, err := s.GetSession(ctx, sid)
	if err != nil || current.State["status"] != "running" {
		t.Fatalf("shutdown normalization changed session: %#v %v", current, err)
	}
}
