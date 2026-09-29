package turn

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/tools"
	goai "github.com/rcarmo/go-ai"
)

// Opt-in real provider check on a disposable store with a harmless local tool.
// No credentials or model response bodies are printed.
func TestLiveProviderToolTurnPersistsReply(t *testing.T) {
	if os.Getenv("GI_RUN_LIVE_COPILOT_PROBE") != "1" {
		t.Skip("set GI_RUN_LIVE_COPILOT_PROBE=1 for a bounded live provider request")
	}
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	const sid = "live-tool-probe"
	const model = "github-copilot/gpt-5.4-mini"
	if _, err := s.CreateSession(ctx, sid, sid, map[string]any{"model": model, "status": "idle"}); err != nil {
		t.Fatal(err)
	}
	e := NewWithRuntimeConfig(s, config.RuntimeConfig{DefaultModel: model, DefaultProvider: "github-copilot", MaxIterations: 3}, "")
	defer e.Close()
	var calls atomic.Int32
	if err := e.RegisterTool(tools.RegisteredTool{
		Name: "probe_echo", Description: "Echo a harmless test value exactly. Use this tool before answering the test prompt.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"]}`),
		Executor: func(_ context.Context, _ tools.ToolRuntime, call goai.ToolCall) (string, error) {
			calls.Add(1)
			if value, _ := call.Arguments["value"].(string); strings.Contains(value, "ping") {
				return "probe_echo returned " + value, nil
			}
			return "probe_echo returned test value", nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.SetActiveTools([]string{"probe_echo", "tools"}); err != nil {
		t.Fatal(err)
	}
	admitted, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Model: model, Prompt: "Call the probe_echo tool with value ping, then reply with PROBE_DONE. Do not call any other tool."})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(90 * time.Second)
	var status string
	for time.Now().Before(deadline) {
		turn, err := s.GetTurn(ctx, admitted.TurnID)
		if err != nil {
			t.Fatal(err)
		}
		status = turn.Status
		if status == "completed" || status == "failed" || status == "cancelled" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if status != "completed" || calls.Load() != 1 {
		// Event types are enough to diagnose the path without logging model output.
		events, _ := s.ListTurnEvents(ctx, admitted.TurnID)
		types := make([]string, 0, len(events))
		for _, event := range events {
			types = append(types, event.Type)
		}
		t.Fatalf("live turn status=%s tool calls=%d events=%q", status, calls.Load(), types)
	}
	messages, err := s.ListMessages(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	user, result, assistant := false, false, false
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			user = true
		case "tool_result":
			if strings.Contains(msg.Content, "probe_echo returned") {
				result = true
			}
		case "assistant":
			if strings.Contains(msg.Content, "PROBE_DONE") {
				assistant = true
			}
		}
	}
	if !user || !result || !assistant {
		t.Fatalf("stored roles/markers incomplete: user=%t tool=%t assistant=%t", user, result, assistant)
	}
	// The worker may still be writing terminal hooks after its status commits.
	for time.Now().Before(deadline) {
		if _, _, err := s.GetSessionActiveTurn(ctx, sid); err != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Log("live provider tool turn and persisted reply verified")
}
