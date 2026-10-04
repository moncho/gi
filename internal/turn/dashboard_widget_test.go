package turn

import (
	"context"
	"github.com/rcarmo/gi/internal/tools"
	goai "github.com/rcarmo/go-ai"
	"testing"
	"time"
)

func TestDashboardWidgetToolPublishesPersistedContentBlock(t *testing.T) {
	db := openTestStore(t)
	defer db.Close()
	db.CreateSession(t.Context(), "s", "s", nil)
	engine := New(db)
	defer engine.Close()
	events := engine.Subscribe("s")
	defer engine.Unsubscribe("s", events)
	result, err := engine.ExecuteToolByName(t.Context(), "send_dashboard_widget", "s", map[string]any{"title": "Widget", "html": "<p>Hello</p>", "widget_id": "persisted"})
	if err != nil {
		t.Fatal(result, err)
	}
	select {
	case event := <-events:
		if event["type"] != "new_post" || event["chat_jid"] != "gi:s" || event["sender"] != "agent" {
			t.Fatal(event)
		}
		data, ok := event["data"].(map[string]any)
		if !ok || data["content_blocks"] == nil {
			t.Fatal("missing widget block", event)
		}
		artifact, err := db.DashboardWidget(t.Context(), "s", "persisted")
		if err != nil || artifact.PostID != event["id"] {
			t.Fatal(artifact, err)
		}
	case <-time.After(time.Second):
		t.Fatal("missing widget SSE event")
	}
}

func TestDashboardWidgetCompletesTurnAfterToolBatch(t *testing.T) {
	db := openTestStore(t)
	defer db.Close()
	engine := New(db)
	defer engine.Close()
	ctx := t.Context()
	db.CreateSession(ctx, "s", "s", nil)
	db.CreateTurnWithStatus(ctx, "turn", "s", "running", "show widget", map[string]any{"model": "bootstrap"})
	if ok, err := db.ClaimSessionActiveTurn(ctx, "s", "turn", "test", "claim"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	db.AddMessage(ctx, "prompt", "s", "user", "show widget", map[string]any{"turn_id": "turn"})
	if _, err := db.CreateTurnWithStatus(ctx, "follow", "s", "queued", "follow-up", nil); err != nil {
		t.Fatal(err)
	}
	laterRan := false
	engine.RegisterTool(tools.RegisteredTool{Name: "after_widget", Executor: func(context.Context, tools.ToolRuntime, goai.ToolCall) (string, error) {
		laterRan = true
		return "ok", nil
	}})
	calls := []goai.ToolCall{{ID: "widget-call", Name: "send_dashboard_widget", Arguments: map[string]any{"html": "<p>Hello</p>", "widget_id": "final-widget"}}, {ID: "later", Name: "after_widget"}}
	outcome := engine.runner("s").executeToolCallsPhase(ctx, db, "turn", "s", "bootstrap", "agent", 1, &goai.Context{}, calls, nil, "", 0, &goai.Usage{})
	if !outcome.terminated || !laterRan {
		t.Fatalf("batch not drained/completed: %+v later=%v", outcome, laterRan)
	}
	turn, err := db.GetTurn(ctx, "turn")
	if err != nil || turn.Status != "completed" {
		t.Fatal(turn, err)
	}
	if queued, err := db.GetTurn(ctx, "follow"); err != nil || queued.Status != "queued" {
		t.Fatal("widget phase consumed queued follow-up", queued, err)
	}
	widget, err := db.DashboardWidget(ctx, "s", "final-widget")
	if err != nil {
		t.Fatal(err)
	}
	if parent, err := db.MessageReplyTo(ctx, "s", widget.PostID); err != nil || parent != "prompt" {
		t.Fatal(parent, err)
	}
}
