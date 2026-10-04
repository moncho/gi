package turn

import (
	goai "github.com/rcarmo/go-ai"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/plan"
)

func TestPlanToolPublishesAndSavedPlanEntersContextSeparately(t *testing.T) {
	db := openTestStore(t)
	defer db.Close()
	db.CreateSession(t.Context(), "s", "s", nil)
	engine := New(db)
	defer engine.Close()
	events := engine.Subscribe("s")
	defer engine.Unsubscribe("s", events)
	before, err := engine.emitHook(t.Context(), HookRequest{Name: HookBeforeAgentStart, SessionID: "s", SystemPrompt: "stable"})
	if err != nil || before.Message != "" {
		t.Fatal(before, err)
	}
	reply, err := engine.ExecuteToolByName(t.Context(), "plan", "s", map[string]any{"action": "write", "markdown": "- [ ] confidential plan text"})
	if err != nil {
		t.Fatal(reply, err)
	}
	select {
	case event := <-events:
		if event["key"] != "plan.changes" || event["chat_jid"] != "gi:s" || event["source"] != "tool" || event["action"] != "write" {
			t.Fatal(event)
		}
	case <-time.After(time.Second):
		t.Fatal("missing plan event")
	}
	after, err := engine.emitHook(t.Context(), HookRequest{Name: HookBeforeAgentStart, SessionID: "s", SystemPrompt: "stable"})
	if err != nil {
		t.Fatal(err)
	}
	if after.Message != "" || strings.Contains(after.SystemPrompt, "confidential plan text") || !strings.Contains(after.SystemPrompt, "## Plan Sidebar") {
		t.Fatal(after)
	}
	native := []goai.Message{goai.UserMessage("native prompt")}
	decorated, err := engine.emitHook(t.Context(), HookRequest{Name: HookBeforeProviderRequest, SessionID: "s", Messages: native, Payload: map[string]any{"stage": "context"}})
	if err != nil || len(decorated.Messages) != 2 || !strings.Contains(decorated.Messages[0].Content[0].Text, "confidential plan text") {
		t.Fatal(decorated, err)
	}
	if len(native) != 1 || native[0].Content[0].Text != "native prompt" {
		t.Fatal("mutated history", native)
	}
	raw, err := engine.emitHook(t.Context(), HookRequest{Name: HookBeforeProviderRequest, SessionID: "s", Messages: native, Payload: map[string]any{"stage": "payload"}})
	if err != nil || raw.Messages != nil {
		t.Fatal("decorated raw provider payload", raw, err)
	}
	empty := ""
	db.MutateSessionPlan(t.Context(), "s", plan.Mutation{Action: "write", Markdown: &empty})
	after, err = engine.emitHook(t.Context(), HookRequest{Name: HookBeforeAgentStart, SessionID: "s", SystemPrompt: "stable"})
	if err != nil || after.Message != "" {
		t.Fatal("empty context", after, err)
	}
	emptyContext, err := engine.emitHook(t.Context(), HookRequest{Name: HookBeforeProviderRequest, SessionID: "s", Messages: native, Payload: map[string]any{"stage": "context"}})
	if err != nil || emptyContext.Messages != nil {
		t.Fatal("empty plan injected", emptyContext, err)
	}
}
