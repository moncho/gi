package tools

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

func TestPlanToolReadWriteDetailsAndPublication(t *testing.T) {
	if !json.Valid(PlanTool().Parameters) {
		t.Fatal("invalid plan parameter schema")
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "plans.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.CreateSession(t.Context(), "s", "s", nil)
	db.CreateSession(t.Context(), "other", "other", nil)
	published := 0
	var details map[string]any
	rt := ToolRuntime{Store: db, SessionID: "s", SetDetails: func(v map[string]any) { details = v }, PublishPlan: func(p store.SessionPlan, source, action string) {
		saved, e := db.SessionPlan(t.Context(), "s")
		if e != nil || saved.Markdown != p.Markdown || source != "tool" {
			t.Fatal("premature event", p, e)
		}
		published++
	}}
	invoke := func(args map[string]any) (string, error) {
		return ExecutePlan(t.Context(), rt, goai.ToolCall{Arguments: args})
	}
	reply, err := invoke(map[string]any{"action": "write", "markdown": "## Heading\n- [ ] one\n- [-] two\n- [x] three"})
	if err != nil || reply != "Updated plan for gi:s." || published != 1 {
		t.Fatal(reply, err, published)
	}
	reply, err = invoke(map[string]any{"action": "read"})
	if err != nil || !strings.Contains(reply, "- [-] two") || published != 1 || details["chat_jid"] != "gi:s" {
		t.Fatal(reply, err, details)
	}
	reply, err = invoke(map[string]any{"action": "patch", "patches": []any{map[string]any{"operation": "update", "match": "two", "status": "completed"}}})
	if err != nil || reply != "Patched plan for gi:s." || published != 2 {
		t.Fatal(reply, err)
	}
	reply, err = invoke(map[string]any{"action": "edit", "edits": []any{map[string]any{"oldText": "three", "newText": "done"}}})
	if err != nil || reply != "Edited plan for gi:s." {
		t.Fatal(reply, err)
	}
	reply, err = invoke(map[string]any{"action": "update", "plan": []any{map[string]any{"step": "next", "status": "pending"}}})
	if err != nil || reply != "Plan updated." {
		t.Fatal(reply, err)
	}
	for _, args := range []map[string]any{
		{"action": "read", "chat_jid": "gi:other"}, {"action": "write"}, {"action": "reset"}, {"action": "unknown"},
		{"action": "write", "markdown": false}, {"action": "read", "typo": true},
		{"action": "patch", "patches": []any{map[string]any{"operation": "remove", "index": 0}}},
	} {
		if _, err = invoke(args); err == nil {
			t.Fatal("invalid accepted", args)
		}
	}
	if published != 4 {
		t.Fatal("published invalid mutation", published)
	}
	if _, err = ExecutePlan(t.Context(), ToolRuntime{}, goai.ToolCall{Arguments: map[string]any{"action": "read"}}); err == nil {
		t.Fatal("no runtime accepted")
	}
}
