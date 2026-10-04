package inference

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rcarmo/gi/internal/store"
)

func TestContextMeasurementUnknownScopeReopenAndModelFit(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "context.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	for _, id := range []string{"A", "B"} {
		if _, err := s.CreateSession(ctx, id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateTurn(ctx, "one", "A", "hello", nil); err != nil {
		t.Fatal(err)
	}
	s.AppendTurnEvent(ctx, "one", "A", "inference.finished", map[string]any{"usage": map[string]any{"input": 50000, "total": 60000}})
	unknown, err := SessionContextUsage(ctx, s, "A", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if unknown["tokens"] != nil || unknown["percent"] != nil || unknown["contextWindow"] != 1000 {
		t.Fatalf("cumulative became context: %+v", unknown)
	}
	if err := s.AppendTurnEvent(ctx, "one", "A", "context.measured", map[string]any{"tokens": 850, "input": 800, "cache_read": 50, "model": "p/model", "iteration": 1}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	measured, err := SessionContextUsage(ctx, s, "A", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if measured["tokens"] != 850 || measured["percent"] != float64(85) {
		t.Fatal(measured)
	}
	if err := CheckSessionModelContext(ctx, s, "A", ModelOption{Label: "small", ContextWindow: 800}); err == nil {
		t.Fatal("undersized model accepted")
	}
	if err := CheckSessionModelContext(ctx, s, "A", ModelOption{Label: "exact", ContextWindow: 850}); err != nil {
		t.Fatal(err)
	}
	if err := CheckSessionModelContext(ctx, s, "B", ModelOption{Label: "small", ContextWindow: 100}); err != nil {
		t.Fatal("foreign measurement applied", err)
	}
	noWindow, _ := SessionContextUsage(ctx, s, "A", 0)
	if noWindow["tokens"] != 850 || noWindow["percent"] != nil || noWindow["contextWindow"] != nil {
		t.Fatal(noWindow)
	}
	options := []ModelOption{{ID: "bootstrap", Label: "test/bootstrap", Provider: "test", Enabled: true, ContextWindow: 800}}
	if _, err := SelectSessionModel(ctx, s, "A", options, "test/bootstrap"); err == nil {
		t.Fatal("shared selection skipped context gate")
	}
}

// After a durable compaction the earlier measurement is stale: report the
// local estimate of the compacted context instead (Piclaw), marked "estimate".
func TestContextUsageEstimatesAfterCompaction(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "context.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.CreateSession(ctx, "A", "A", nil)
	s.CreateTurn(ctx, "one", "A", "hello", nil)
	s.AddMessage(ctx, "m1", "A", "user", "hello there", nil)
	s.AppendTurnEvent(ctx, "one", "A", "context.measured", map[string]any{"tokens": 10, "input": 10})
	s.AppendTurnEvent(ctx, "one", "A", "compaction.completed", map[string]any{"durable_context": false})
	if u, _ := SessionContextUsage(ctx, s, "A", 1000); u["tokens"] != 10 || u["source"] != "provider_request" {
		t.Fatal("non-durable compaction made the measurement stale", u)
	}
	s.AppendTurnEvent(ctx, "one", "A", "compaction.completed", map[string]any{"durable_context": true})
	u, err := SessionContextUsage(ctx, s, "A", 1000)
	if err != nil || u["source"] != "estimate" || u["tokens"] == 10 || u["tokens"].(int) <= 0 {
		t.Fatal(u, err)
	}
	s.AppendTurnEvent(ctx, "one", "A", "context.measured", map[string]any{"tokens": 42, "input": 42})
	if u, _ := SessionContextUsage(ctx, s, "A", 1000); u["tokens"] != 42 || u["source"] != "provider_request" {
		t.Fatal("new measurement not preferred", u)
	}
}
