package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/inference"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/turn"
	goai "github.com/rcarmo/go-ai"
)

// Real provider → turn → read/write → live events → TUI; fixture only exists in
// the test binary. Reopening uses stored tool calls, not the fixture files.
func TestToolSyntaxPTYFixture(t *testing.T) {
	dir := os.Getenv("GI_TOOL_SYNTAX_PTY_DIR")
	if dir == "" {
		t.Skip("make test-tui-tool-syntax-pty")
	}
	var requests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		delta := map[string]any{"content": "Syntax fixture complete"}
		reason := "stop"
		if requests.Add(1) == 1 {
			argsRead, _ := json.Marshal(map[string]any{"path": "read.go"})
			argsWrite, _ := json.Marshal(map[string]any{"path": "write.go", "content": "package written\n\nfunc written() {\n\tprintln(\"WRITE-SENTINEL\")\n}\n"})
			delta = map[string]any{"tool_calls": []any{
				map[string]any{"index": 0, "id": "read-call", "type": "function", "function": map[string]any{"name": "read", "arguments": string(argsRead)}},
				map[string]any{"index": 1, "id": "write-call", "type": "function", "function": map[string]any{"name": "write", "arguments": string(argsWrite)}},
			}}
			reason = "tool_calls"
		}
		data, _ := json.Marshal(map[string]any{"id": "syntax", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", data)
	}))
	defer provider.Close()
	inference.Init()
	goai.RegisterModel(&goai.Model{ID: "syntax-fixture", Name: "Syntax fixture", Provider: "syntax-local", Api: goai.ApiOpenAICompletions, BaseURL: provider.URL, MaxTokens: 4096, ContextWindow: 16384})
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".pi", "agent"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".pi", "agent", "auth.json"), []byte(`{"syntax-local":{"type":"api_key","apiKey":"fixture"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Load(dir)
	cfg.DefaultModel = "syntax-local/syntax-fixture"
	cfg.DefaultProvider = "syntax-local"
	s, err := store.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	engine := turn.NewWithRuntimeConfig(s, cfg, cfg.SystemPrompt)
	defer engine.Close()
	if err = runWithEngineMode(s, engine, cfg, os.Getenv("GI_TOOL_SYNTAX_PTY_MODE") == "regular"); err != nil {
		t.Fatal(err)
	}
}
