package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestDashboardWidgetWriteRollsBackAndDoesNotStealIdentity(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "widget.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.CreateSession(t.Context(), "s", "s", nil)
	if _, err = db.DB().Exec(`update sessions set updated_at='old' where id='s'; create trigger reject_widget before insert on messages when json_extract(new.payload_json,'$.kind')='dashboard_widget' begin select raise(abort,'injected widget failure'); end`); err != nil {
		t.Fatal(err)
	}
	block := map[string]any{"type": "generated_widget", "widget_id": "one", "artifact": map[string]any{"kind": "html", "html": "hello"}}
	if _, err = db.PostDashboardWidget(t.Context(), "s", "t", "fallback", block); err == nil {
		t.Fatal("expected write failure")
	}
	session, err := db.GetSession(t.Context(), "s")
	if err != nil || session.UpdatedAt != "old" {
		t.Fatal("partial activity update", session, err)
	}
	if _, err = db.DashboardWidget(t.Context(), "s", "one"); err == nil {
		t.Fatal("partial artifact write")
	}
	db.DB().Exec(`drop trigger reject_widget`)
	first, err := db.PostDashboardWidget(t.Context(), "s", "t", "fallback", block)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.PostDashboardWidget(t.Context(), "s", "t", "replaced", block); !errors.Is(err, ErrWidgetExists) {
		t.Fatal("duplicate allowed", err)
	}
	persisted, err := db.DashboardWidget(t.Context(), "s", "one")
	if err != nil || persisted.PostID != first.ID {
		t.Fatal("identity stolen", persisted, err)
	}
}
