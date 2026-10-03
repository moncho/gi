package turn

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/inference"
	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

// A response keeps its usage, stop reason, thinking blocks and thinking
// level, so session export can write Pi's assistant messages and
// thinking_level_change entries.
func TestResponseRecordedWithUsageThinkingAndLevel(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "record.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e := New(s)
	defer e.Close()
	inference.Init()
	low, high := "low", "high"
	model := "record-test/reasoner"
	goai.RegisterModel(&goai.Model{ID: "reasoner", Provider: "record-test", Api: goai.ApiOpenAICompletions, ContextWindow: 32000, MaxTokens: 1024, Reasoning: true, ThinkingLevelMap: map[goai.ModelThinkingLevel]*string{"off": nil, "minimal": nil, "low": &low, "medium": nil, "high": &high}})
	if _, err := s.CreateSession(ctx, "R", "R", map[string]any{"selected_model": model, "thinking_model": model, "thinking_level": "high"}); err != nil {
		t.Fatal(err)
	}
	original := streamWithToolsWithHooks
	defer func() { streamWithToolsWithHooks = original }()
	streamWithToolsWithHooks = func(ctx context.Context, _ string, _ *goai.Context, _ func(map[string]any), _ *inference.StreamHooks) (*inference.StreamResult, error) {
		msg := goai.Message{Role: "assistant", StopReason: goai.StopReasonLength, Content: []goai.ContentBlock{
			{Type: "thinking", Thinking: "Plan.", ThinkingSignature: "sig"},
			{Type: "text", Text: "done"},
		}}
		usage := goai.Usage{Input: 12, Output: 7, CacheRead: 3, TotalTokens: 22, Cost: goai.CostBreakdown{Input: 0.1, Output: 0.2, Total: 0.3}}
		return &inference.StreamResult{Message: &msg, Text: "done", Usage: &usage}, nil
	}
	run, err := e.SubmitPrompt(ctx, RunInput{SessionID: "R", Prompt: "go", Model: model})
	if err != nil {
		t.Fatal(err)
	}
	waitForCondition(t, 3*time.Second, func() bool { row, err := s.GetTurn(ctx, run.TurnID); return err == nil && row.Status == "completed" }, "completion")
	waitForCondition(t, 3*time.Second, func() bool {
		runner := e.runner("R")
		runner.mu.Lock()
		defer runner.mu.Unlock()
		_, _, err := s.GetSessionActiveTurn(ctx, "R")
		return errors.Is(err, sql.ErrNoRows)
	}, "cleanup")
	messages, err := s.ListMessages(ctx, "R")
	if err != nil {
		t.Fatal(err)
	}
	var reply *store.Message
	for i := range messages {
		if messages[i].Role == "assistant" {
			reply = &messages[i]
		}
	}
	if reply == nil {
		t.Fatalf("no reply in %+v", messages)
	}
	p := reply.Payload
	usage, _ := p["usage"].(map[string]any)
	cost, _ := usage["cost"].(map[string]any)
	if usage["input"] != float64(12) || usage["totalTokens"] != float64(22) || cost["total"] != 0.3 {
		t.Fatalf("usage %v", p["usage"])
	}
	blocks, _ := p["thinking_blocks"].([]any)
	if len(blocks) != 1 || p["stop_reason"] != "length" || p["thinking_level"] != "high" {
		t.Fatalf("payload %v", p)
	}
	if b, _ := blocks[0].(map[string]any); b["thinking"] != "Plan." || b["thinkingSignature"] != "sig" {
		t.Fatalf("thinking %v", blocks)
	}
}
