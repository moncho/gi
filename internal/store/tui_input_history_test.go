package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTUIInputHistorySessionIsolationBackfillAndRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, id := range []string{"A", "B"} {
		if _, err := s.CreateSession(ctx, id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AddMessage(ctx, "m1", "A", "user", "old prompt", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMessage(ctx, "m2", "A", "assistant", "not history", nil); err != nil {
		t.Fatal(err)
	}
	check := func(id string, limit int, want []string) {
		t.Helper()
		got, err := s.ListTUIInputHistory(ctx, id, limit)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s history = %#v (%v); want %#v", id, got, err, want)
		}
	}
	check("A", 10, []string{"old prompt"})
	check("B", 10, nil)
	for _, entry := range []string{"/where", "  multi\nline  ", "!!printf hi"} {
		if err := s.RecordTUIInput(ctx, "A", entry, 3); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RecordTUIInput(ctx, "B", "/help", 3); err != nil {
		t.Fatal(err)
	}
	check("A", 10, []string{"/where", "  multi\nline  ", "!!printf hi"})
	check("A", 2, []string{"  multi\nline  ", "!!printf hi"})
	check("B", 10, []string{"/help"})
	if err := s.RecordTUIInput(ctx, "missing", "bad", 3); err == nil {
		t.Fatal("accepted unknown session")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	check("A", 10, []string{"/where", "  multi\nline  ", "!!printf hi"})
	check("B", 10, []string{"/help"})
}
