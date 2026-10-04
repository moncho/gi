package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func deleteFixture(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "delete.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for _, id := range []string{"a", "b"} {
		if _, err = s.CreateSession(t.Context(), id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	for i, role := range []string{"user", "assistant", "user", "assistant"} {
		id := string(rune('a' + i))
		if err = s.AddMessage(t.Context(), id, "a", role, "content "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.AddMessage(t.Context(), "other", "b", "assistant", "other", nil); err != nil {
		t.Fatal(err)
	}
	return s, path
}
func TestDeleteMessageAtomicCheckpointResetReopenAndIsolation(t *testing.T) {
	s, path := deleteFixture(t)
	before, err := s.ContextSnapshot(t.Context(), "a")
	if err != nil {
		t.Fatal(err)
	}
	if err = commitDeleteTestCheckpoint(t, s, before, "summary contains deleted text", 2); err != nil {
		t.Fatal(err)
	}
	stale, err := s.ContextSnapshot(t.Context(), "a")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteMessage(t.Context(), "b", "a"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	if err = s.DeleteMessage(t.Context(), "a", "a"); err != nil {
		t.Fatal(err)
	}
	after, err := s.ContextSnapshot(t.Context(), "a")
	if err != nil || after.Version != 2 || after.Summary != "" || len(after.Messages) != 3 {
		t.Fatal(after, err)
	}
	if err = commitDeleteTestCheckpoint(t, s, stale, "old summary", 1); !errors.Is(err, ErrContextChanged) {
		t.Fatal("stale snapshot accepted", err)
	}
	if err = s.DeleteMessage(t.Context(), "a", "a"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	peer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	rows, err := peer.ListMessages(t.Context(), "a")
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	hits, err := peer.SearchMessages(t.Context(), "a", "content a", "current", 10, 0)
	if err != nil || len(hits) != 0 {
		t.Fatal(hits, err)
	}
	page, err := peer.PageMessages(t.Context(), "a", "", "", 50)
	if err != nil || len(page.Messages) != 3 {
		t.Fatal(page, err)
	}
	other, err := peer.ListMessages(t.Context(), "b")
	if err != nil || len(other) != 1 {
		t.Fatal(other, err)
	}
}
func TestDeleteMessageRollsBackFailureAndRejectsActiveQueuedProtected(t *testing.T) {
	for _, mode := range []string{"write", "cancel", "queued", "running", "claim", "protected"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := deleteFixture(t)
			ctx := t.Context()
			switch mode {
			case "write":
				_, err := s.DB().Exec(`CREATE TRIGGER reject_reset BEFORE INSERT ON context_checkpoints BEGIN SELECT raise(abort,'rejected'); END`)
				if err != nil {
					t.Fatal(err)
				}
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "queued", "running", "claim":
				status := mode
				if mode == "claim" {
					status = "completed"
				}
				if _, err := s.CreateTurnWithStatus(t.Context(), "turn", "a", status, "audit", nil); err != nil {
					t.Fatal(err)
				}
				if mode == "claim" {
					if _, err := s.ClaimSessionActiveTurn(t.Context(), "a", "turn", "worker", "token"); err != nil {
						t.Fatal(err)
					}
				}
			case "protected":
				if _, err := s.DB().Exec("UPDATE messages SET role='tool_result' WHERE id='a'"); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.DeleteMessage(ctx, "a", "a"); err == nil {
				t.Fatal("unsafe deletion accepted")
			}
			rows, err := s.ListMessages(t.Context(), "a")
			if err != nil || len(rows) != 4 {
				t.Fatal("partial delete", rows, err)
			}
		})
	}
}
func TestDeleteMessageRetainsAuditAndMedia(t *testing.T) {
	s, _ := deleteFixture(t)
	if _, err := s.CreateTurnWithStatus(t.Context(), "turn", "a", "completed", "retained audit", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendTurnEvent(t.Context(), "turn", "a", "completed", map[string]any{"content": "retained"}); err != nil {
		t.Fatal(err)
	}
	media, err := s.CreateMedia(t.Context(), "a", "kept.txt", "text/plain", []byte("kept bytes"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteMessage(t.Context(), "a", "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTurn(t.Context(), "turn"); err != nil {
		t.Fatal(err)
	}
	_, raw, err := s.GetMediaContent(t.Context(), media.ID)
	if err != nil || string(raw) != "kept bytes" {
		t.Fatal(string(raw), err)
	}
	events, err := s.ListTurnEvents(t.Context(), "turn")
	if err != nil || len(events) != 1 {
		t.Fatal(events, err)
	}
}

func commitDeleteTestCheckpoint(t *testing.T, s *Store, snapshot ContextSnapshot, summary string, count int) error {
	t.Helper()
	boundary, err := PrepareContextBoundary(snapshot, count)
	if err != nil {
		return err
	}
	tx, err := s.DB().BeginTx(t.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = commitContextBoundary(t.Context(), tx, "a", summary, boundary); err != nil {
		return err
	}
	return tx.Commit()
}

func TestDeleteMessageCascadeUsesTurnRootAndSessionIsolation(t *testing.T) {
	s, path := deleteFixture(t)
	defer s.Close()
	ctx := t.Context()
	for _, m := range []struct{ id, session, role, turn string }{
		{"root", "a", "user", "shared-turn"}, {"answer", "a", "assistant", "shared-turn"},
		{"steer", "a", "user", "shared-turn"}, {"later-answer", "a", "assistant", "shared-turn"},
		{"notice", "a", "system", "shared-turn"}, {"other-answer", "b", "assistant", "shared-turn"},
	} {
		if err := s.AddMessage(ctx, m.id, m.session, m.role, m.id, map[string]any{"kind": "chat", "turn_id": m.turn}); err != nil {
			t.Fatal(err)
		}
	}
	// Keep root ordering explicit even on platforms with a coarse clock.
	if _, err := s.db.ExecContext(ctx, "update messages set created_at='2000-01-01T00:00:00Z' where id='root'"); err != nil {
		t.Fatal(err)
	}
	page, err := s.PageConversationMessages(ctx, "a", "", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range page.Messages {
		if m.ID == "answer" || m.ID == "later-answer" {
			if m.ReplyToID != "root" {
				t.Fatalf("page reply %s: %q", m.ID, m.ReplyToID)
			}
		}
	}
	search, err := s.SearchConversationMessages(ctx, "a", "answer", "all", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range search {
		if m.SessionID == "a" && m.ReplyToID != "root" {
			t.Fatalf("search: %+v", m)
		}
		if m.SessionID == "b" && m.ReplyToID != "" {
			t.Fatalf("cross-session parent: %+v", m)
		}
	}
	ids, err := s.DeleteMessageWithReplies(ctx, "a", "root", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 {
		t.Fatalf("deleted: %v", ids)
	}
	// Keep steering user rows, notices, unrelated rows and other sessions intact.
	rows, err := s.ListMessages(ctx, "a")
	if err != nil || len(rows) != 6 {
		t.Fatalf("remaining %d: %v", len(rows), err)
	}
	other, err := s.ListMessages(ctx, "b")
	if err != nil || len(other) != 2 {
		t.Fatal(other, err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range ids {
		if _, err = s.MessageReplyTo(ctx, "a", id); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("reopen %s: %v", id, err)
		}
	}
}

func TestCascadeDeleteRollsBackReplyFailure(t *testing.T) {
	s, _ := deleteFixture(t)
	defer s.Close()
	ctx := t.Context()
	s.AddMessage(ctx, "root", "a", "user", "prompt", map[string]any{"turn_id": "t"})
	s.AddMessage(ctx, "reply", "a", "assistant", "answer", map[string]any{"turn_id": "t"})
	before, err := s.ContextSnapshot(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if err = commitDeleteTestCheckpoint(t, s, before, "old summary", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `create trigger reject_reply before delete on messages when old.id='reply' begin select raise(abort,'injected reply failure'); end`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteMessageWithReplies(ctx, "a", "root", true); err == nil {
		t.Fatal("expected rollback")
	}
	rows, _ := s.ListMessages(ctx, "a")
	if len(rows) != 6 {
		t.Fatalf("partial cascade: %d", len(rows))
	}
	cp, err := s.ContextSnapshot(ctx, "a")
	if err != nil || cp.Summary != "old summary" {
		t.Fatalf("checkpoint changed: %+v %v", cp, err)
	}
}
