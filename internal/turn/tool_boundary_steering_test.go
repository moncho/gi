package turn

import (
	"context"
	"reflect"
	"testing"

	"github.com/rcarmo/gi/internal/tools"
	goai "github.com/rcarmo/go-ai"
)

// Pi 0.87.1 finishes both sequential calls from one assistant response before
// delivering steering queued while the first call executes. A queued instruction
// must not silently replace the second tool result with a synthetic skip.
func TestToolBoundarySteeringCompletesCurrentBatch(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	e := New(s)
	defer e.Close()
	const sessionID, turnID, queueID = "tool-boundary", "active", "queued"
	if _, err := s.CreateSession(ctx, sessionID, sessionID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, turnID, sessionID, "running", "prompt", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, queueID, sessionID, "queued", "steer after tools", nil); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimSessionActiveTurn(ctx, sessionID, turnID, "test", "tool-boundary-claim"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	var executed []string
	for _, name := range []string{"first_boundary", "second_boundary"} {
		name := name
		if err := e.RegisterTool(tools.RegisteredTool{Name: name, Description: name, Executor: func(ctx context.Context, _ tools.ToolRuntime, _ goai.ToolCall) (string, error) {
			executed = append(executed, name)
			if name == "first_boundary" {
				if err := s.SteerQueuedTurn(ctx, sessionID, queueID, turnID); err != nil {
					return "", err
				}
			}
			return "result " + name, nil
		}}); err != nil {
			t.Fatal(err)
		}
	}
	calls := []goai.ToolCall{{ID: "one", Name: "first_boundary", Arguments: map[string]any{}}, {ID: "two", Name: "second_boundary", Arguments: map[string]any{}}}
	outcome := e.runner(sessionID).executeToolCallsPhase(ctx, s, turnID, sessionID, "bootstrap", "agent", 1, &goai.Context{}, calls, nil, "", 0, &goai.Usage{})
	if outcome.terminated || outcome.skipRemainingTools || !reflect.DeepEqual(executed, []string{"first_boundary", "second_boundary"}) || len(outcome.pendingSteering) != 1 {
		t.Fatalf("tool boundary: executed=%v pending=%v skip=%t terminated=%t", executed, outcome.pendingSteering, outcome.skipRemainingTools, outcome.terminated)
	}
	messages, err := s.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Content != "result first_boundary" || messages[1].Content != "result second_boundary" {
		t.Fatalf("tool results: %#v", messages)
	}
	events, err := s.ListTurnEvents(ctx, turnID)
	if err != nil {
		t.Fatal(err)
	}
	var toolEvents []string
	for _, event := range events {
		if event.Type == "tool.skipped" {
			t.Fatalf("queued steering skipped an announced tool: %#v", event)
		}
		if event.Type == "tool.started" || event.Type == "tool.finished" {
			toolEvents = append(toolEvents, event.Type+":"+event.Payload["tool"].(string))
		}
	}
	if !reflect.DeepEqual(toolEvents, []string{"tool.started:first_boundary", "tool.finished:first_boundary", "tool.started:second_boundary", "tool.finished:second_boundary"}) {
		t.Fatalf("tool event order: %v", toolEvents)
	}
	if n := e.runner(sessionID).injectSteeringMessages(ctx, sessionID, turnID, &goai.Context{}, outcome.pendingSteering); n != 1 {
		t.Fatalf("steering not injected after both tools: %d", n)
	}
	messages, err = s.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 3 || messages[2].Content != "steer after tools" || messages[2].Payload["steering"] != true {
		t.Fatalf("steering persistence order: %#v, %v", messages, err)
	}
}

func TestToolBoundarySteeringAfterHookResultCompletesBatch(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	e := New(s)
	defer e.Close()
	const sessionID, turnID, queueID = "hook-boundary", "active-hook", "queued-hook"
	if _, err := s.CreateSession(ctx, sessionID, sessionID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, turnID, sessionID, "running", "prompt", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, queueID, sessionID, "queued", "steer after hook", nil); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimSessionActiveTurn(ctx, sessionID, turnID, "test", "hook-boundary-claim"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if _, err := e.RegisterHook(HookToolCall, "first-hook", func(ctx context.Context, req HookRequest) (HookResponse, error) {
		if req.ToolCall.Name != "hook_boundary" {
			return HookResponse{}, nil
		}
		if err := s.SteerQueuedTurn(ctx, sessionID, queueID, turnID); err != nil {
			return HookResponse{}, err
		}
		text := "hook result"
		return HookResponse{Action: "respond", Handled: true, ToolResult: &text}, nil
	}); err != nil {
		t.Fatal(err)
	}
	executed := false
	if err := e.RegisterTool(tools.RegisteredTool{Name: "second_boundary", Description: "second", Executor: func(context.Context, tools.ToolRuntime, goai.ToolCall) (string, error) {
		executed = true
		return "second result", nil
	}}); err != nil {
		t.Fatal(err)
	}
	calls := []goai.ToolCall{{ID: "one", Name: "hook_boundary", Arguments: map[string]any{}}, {ID: "two", Name: "second_boundary", Arguments: map[string]any{}}}
	outcome := e.runner(sessionID).executeToolCallsPhase(ctx, s, turnID, sessionID, "bootstrap", "agent", 1, &goai.Context{}, calls, nil, "", 0, &goai.Usage{})
	if outcome.terminated || outcome.skipRemainingTools || !executed || len(outcome.pendingSteering) != 1 {
		t.Fatalf("hook tool boundary: executed=%t pending=%v skip=%t terminated=%t", executed, outcome.pendingSteering, outcome.skipRemainingTools, outcome.terminated)
	}
	messages, err := s.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 2 || messages[0].Content != "hook result" || messages[1].Content != "second result" {
		t.Fatalf("hook and tool results: %#v, %v", messages, err)
	}
}
