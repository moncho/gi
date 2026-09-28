package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestIdleQueuePromptOwnsIdentityAndDeduplicates(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "idle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.CreateSession(ctx, "a", "a", nil)
	s.CreateTurnWithStatus(ctx, "q", "a", "queued", "original", nil)
	if ok, err := s.ClaimIdleQueueAction(ctx, "a", "q", "test", "q", ""); err != nil || !ok {
		t.Fatal(ok, err)
	}
	wrong := map[string]any{"turn_id": "foreign", "idle_queue_action": false}
	for _, payload := range []map[string]any{nil, wrong} {
		if err = s.PersistIdleQueuePrompt(ctx, "a", "q", "original", payload); err != nil {
			t.Fatal(err)
		}
	}
	if wrong["turn_id"] != "foreign" {
		t.Fatal("mutated caller")
	}
	msgs, err := s.ListMessages(ctx, "a")
	if err != nil || len(msgs) != 1 || msgs[0].Payload["turn_id"] != "q" {
		t.Fatal(msgs, err)
	}
	if has, err := s.HasIdleQueuePrompt(ctx, "a", "q"); err != nil || !has {
		t.Fatal(has, err)
	}
}
