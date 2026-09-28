package turn

import (
	"context"
	"testing"
)

func TestToolOutputReporterPersistsBeforeNotificationAndFlushesFinal(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()
	e := New(s)

	ctx := context.Background()
	_, _ = s.CreateSession(ctx, "a", "a", nil)
	_, _ = s.CreateTurn(ctx, "t", "a", "prompt", nil)
	_ = s.AppendTurnEvent(ctx, "t", "a", "tool.started", map[string]any{"tool": "shell", "tool_call_id": "call", "occurrence_id": "occ"})
	r := &sessionRunner{store: s, engine: e}
	report := r.toolOutputReporter("t", "a", "call", "occ")
	if err := report("first", false); err != nil {
		t.Fatal(err)
	}
	if err := report("first\nsecond", false); err != nil {
		t.Fatal(err)
	}
	if err := report("first\nsecond", true); err != nil {
		t.Fatal(err)
	}
	a, err := s.SessionActivity(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if a["tool"].(map[string]any)["output_preview"] != "first\nsecond" {
		t.Fatal(a)
	}
	var count int
	s.DB().QueryRow(`select count(*) from turn_events where event_type='tool.output'`).Scan(&count)
	if count != 2 {
		t.Fatal(count)
	}
	// A failed preview write stays failed; a final flush cannot disguise it.
	if _, err = s.DB().Exec(`CREATE TRIGGER reject_output BEFORE INSERT ON turn_events WHEN NEW.event_type='tool.output' BEGIN SELECT RAISE(ABORT,'fixture output failure'); END`); err != nil {
		t.Fatal(err)
	}
	failed := r.toolOutputReporter("t", "a", "call", "occ")
	if failed("new", true) == nil || failed("", true) == nil {
		t.Fatal("persistence failure lost")
	}
}
