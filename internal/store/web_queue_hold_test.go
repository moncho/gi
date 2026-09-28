package store

import (
	"context"
	"path/filepath"
	"testing"
)

// Upgrade removes only the invented pause fence, not queue data or metadata.
func TestWebQueueHoldLegacyMigrationRetainsQueuedWork(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateSession(ctx, "a", "a", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateTurnWithStatus(ctx, "q", "a", "queued", "retained", map[string]any{"model": "bootstrap", "media": []any{"attachment:7"}}); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetTurn(ctx, "q")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB().Exec(`create table web_queue_holds(session_id text primary key,stop_turn_id text,created_at text);insert into web_queue_holds values('a','old',datetime('now'))`); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var tables int
	if err = s.DB().QueryRow(`select count(*) from sqlite_master where name='web_queue_holds'`).Scan(&tables); err != nil || tables != 0 {
		t.Fatal(tables, err)
	}
	after, err := s.GetTurn(ctx, "q")
	if err != nil || after.Status != "queued" || after.Prompt != before.Prompt || after.Metadata["model"] != "bootstrap" || after.Metadata["media"] == nil {
		t.Fatal(after, err)
	}
	if ok, err := s.ClaimSessionActiveTurn(ctx, "a", "q", "worker", "q"); err != nil || !ok {
		t.Fatal("stranded queued work", ok, err)
	}
}
