package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/turn"
)

// A turn held because gi stopped mid-turn is announced once when its
// session opens, and /retry skip dismisses it without resending (#21).
func TestHeldInterruptionNoticeAndSkip(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "gi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.CreateSession(ctx, "S", "S", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, "turn_held", "S", "failed", "deploy the thing", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateTurnStatusAndPhase(ctx, "turn_held", "failed", "held_for_retry_or_skip"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkTurnFailureWithFallbackErr(ctx, nil, "turn_held", "S", turn.InterruptedFailureKind, "review", "Interrupted when gi stopped; held instead of resending"); err != nil {
		t.Fatal(err)
	}
	e := turn.New(s)
	defer e.Close()
	c := &chatTUI{store: s, engine: e, sessionID: "S", cfg: config.RuntimeConfig{}}
	c.noticeHeldInterruptions()
	c.noticeHeldInterruptions() // once per session
	text := strings.Join(c.transcript, "\n")
	if strings.Count(text, "were held, not resent") != 1 || !strings.Contains(text, "turn_held  deploy the thing") || !strings.Contains(text, "/retry skip <id>") {
		t.Fatalf("notice:\n%s", text)
	}
	out := strings.Join(c.retryCommand([]string{"/retry", "skip", "turn_held"}), "\n")
	if !strings.Contains(out, "skipped turn_held") {
		t.Fatalf("skip: %s", out)
	}
	if f, err := s.GetTurnFailure(ctx, "turn_held"); err != nil || f.ResolutionState != "skipped" {
		t.Fatalf("failure %+v %v", f, err)
	}
}
