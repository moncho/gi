package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

func TestDashboardWidgetToolPersistsBeforePublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "widgets.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.CreateSession(t.Context(), "s", "s", nil)
	db.CreateSession(t.Context(), "other", "other", nil)
	published := 0
	rt := ToolRuntime{Store: db, SessionID: "s", TurnID: "turn", PublishMessage: func(m store.Message) {
		persisted, e := db.DashboardWidget(t.Context(), "s", "test-widget")
		if e != nil || persisted.PostID != m.ID {
			t.Fatalf("published before persistence: %v", e)
		}
		published++
	}}
	call := goai.ToolCall{Name: "send_dashboard_widget", Arguments: map[string]any{"html": "<p>hello</p>", "title": "Widget test", "widget_id": "test-widget"}}
	result, err := ExecuteDashboardWidget(t.Context(), rt, call)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err = json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatal(err)
	}
	if data["status"] != "posted" || published != 1 {
		t.Fatal(result, published)
	}
	artifact, err := db.DashboardWidget(t.Context(), "s", "test-widget")
	if err != nil {
		t.Fatal(err)
	}
	if artifact.ContentBlock["type"] != "generated_widget" || artifact.ChatJID != "gi:s" {
		t.Fatalf("artifact %+v", artifact)
	}
	rows, err := db.ListMessages(t.Context(), "s")
	if err != nil || len(rows) != 1 || rows[0].Role != "assistant" {
		t.Fatal(rows, err)
	}
	snapshot, err := db.ContextSnapshot(t.Context(), "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Messages) != 1 || strings.Contains(snapshot.Messages[0].Content, "<p>") {
		t.Fatal("HTML entered model content", snapshot.Messages)
	}
	if _, err = ExecuteDashboardWidget(t.Context(), rt, call); err != store.ErrWidgetExists {
		t.Fatalf("duplicate: %v", err)
	}
	if published != 1 {
		t.Fatal("published a duplicate")
	}
	db.Close()
	db, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.DashboardWidget(t.Context(), "s", "test-widget"); err != nil {
		t.Fatal("lost on reopen", err)
	}
	if _, err = db.DashboardWidget(t.Context(), "other", "test-widget"); err == nil {
		t.Fatal("cross-session lookup")
	}
}

func TestDashboardWidgetToolValidatesBeforeWrite(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "widgets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.CreateSession(t.Context(), "s", "s", nil)
	rt := ToolRuntime{Store: db, SessionID: "s"}
	for _, args := range []map[string]any{
		{}, {"html": " "}, {"html": 42}, {"html": strings.Repeat("x", MaxWidgetHTMLBytes+1)},
		{"html": "x", "title": true}, {"html": "x", "interactive": "true"},
		{"html": "x", "widget_id": "a/b"}, {"html": "x", "chat_jid": "gi:other"}, {"html": "x", "unknown": true},
	} {
		if _, err = ExecuteDashboardWidget(t.Context(), rt, goai.ToolCall{Arguments: args}); err == nil {
			t.Fatalf("accepted %+v", args)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = ExecuteDashboardWidget(ctx, rt, goai.ToolCall{Arguments: map[string]any{"html": "x"}}); err == nil {
		t.Fatal("cancelled write accepted")
	}
	rows, _ := db.ListMessages(t.Context(), "s")
	if len(rows) != 0 {
		t.Fatal("validation wrote rows", rows)
	}
	if _, err = ExecuteDashboardWidget(t.Context(), ToolRuntime{}, goai.ToolCall{Arguments: map[string]any{"html": "x"}}); err == nil {
		t.Fatal("missing runtime accepted")
	}
}
