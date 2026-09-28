package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMessageDisplayRowsStableOptInAndScoped(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rows.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if _, err = s.CreateSession(ctx, id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range []struct{ id, session string }{{"msg_1000000000000000001", "a"}, {"msg_1000000000000000002", "b"}} {
		if err = s.AddMessage(ctx, m.id, m.session, "user", "find me", nil); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.PageConversationMessages(ctx, "a", "", "", 10)
	if err != nil || len(page.Messages) != 1 {
		t.Fatal(page, err)
	}
	m := page.Messages[0]
	if m.DisplayRowID <= 0 || m.ID != "msg_1000000000000000001" {
		t.Fatal(m)
	}
	raw, err := s.PageMessages(ctx, "a", "", "", 10)
	if err != nil || raw.Messages[0].DisplayRowID != 0 {
		t.Fatal(raw, err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	found, err := s.SearchConversationMessages(ctx, "a", "find", "current", 10, 0)
	if err != nil || len(found) != 1 || found[0].DisplayRowID != m.DisplayRowID || found[0].ID != m.ID {
		t.Fatal(found, err)
	}
	other, err := s.PageConversationMessages(ctx, "b", "", "", 10)
	if err != nil || other.Messages[0].DisplayRowID == m.DisplayRowID {
		t.Fatal(other, err)
	}
}
