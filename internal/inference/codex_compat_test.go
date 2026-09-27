package inference

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	goai "github.com/rcarmo/go-ai"
)

func TestCodexPayloadHook(t *testing.T) {
	model := &goai.Model{Api: goai.ApiOpenAICodexResponses}
	original := map[string]any{"max_output_tokens": 100, "input": []any{}, "large": json.Number("9007199254740993")}
	called := false
	hook := codexPayloadHook(func(payload any, m *goai.Model) (any, error) {
		called = true
		if m != model || payload == nil {
			t.Fatal("hook lost its inputs")
		}
		return original, nil
	})
	value, err := hook(struct{}{}, model)
	if err != nil || !called {
		t.Fatalf("hook: called=%v err=%v", called, err)
	}
	fields := value.(map[string]json.RawMessage)
	if _, ok := fields["max_output_tokens"]; ok {
		t.Fatal("unsupported limit retained")
	}
	if string(fields["large"]) != "9007199254740993" {
		t.Fatal("numeric precision changed")
	}
	if original["max_output_tokens"] != 100 {
		t.Fatal("mutated caller's payload")
	}
	for _, other := range []*goai.Model{nil, {Api: goai.ApiOpenAIResponses}} {
		value, err := codexPayloadHook(nil)(original, other)
		if err != nil || value.(map[string]any)["max_output_tokens"] != 100 {
			t.Fatal("non-Codex payload changed")
		}
	}
	want := errors.New("hook denied")
	if _, err := codexPayloadHook(func(any, *goai.Model) (any, error) { return nil, want })(original, model); !errors.Is(err, want) {
		t.Fatal("hook error swallowed")
	}
	if _, err := codexPayloadHook(nil)([]string{"invalid"}, model); err == nil {
		t.Fatal("accepted non-object payload")
	}
	if _, err := codexPayloadHook(nil)(nil, model); err == nil {
		t.Fatal("accepted null payload")
	}
}

func TestCodexStreamOmitsUnsupportedLimitOnNativeRequest(t *testing.T) {
	Init()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".pi", "agent"), 0700); err != nil {
		t.Fatal(err)
	}
	// Synthetic unsigned JWT only supplies the local provider account claim.
	claim := base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"fixture-account"}}`))
	auth := map[string]any{"openai-codex": map[string]any{"type": "oauth", "access": "fixture." + claim + ".fixture"}}
	raw, _ := json.Marshal(auth)
	if err := os.WriteFile(filepath.Join(home, ".pi", "agent", "auth.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var requests []map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default provider tries a WebSocket upgrade first. Reject locally so the
		// real go-ai transport falls back to its SSE request builder.
		if r.Method != http.MethodPost {
			http.Error(w, "use SSE", http.StatusNotFound)
			return
		}
		if r.URL.Path != "/codex/responses" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var source io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "zstd" {
			decoder, err := zstd.NewReader(r.Body)
			if err != nil {
				t.Errorf("zstd: %v", err)
				http.Error(w, "bad encoding", 400)
				return
			}
			defer decoder.Close()
			source = decoder
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(source).Decode(&body); err != nil {
			t.Errorf("request: %v", err)
			http.Error(w, "bad request", 400)
			return
		}
		mu.Lock()
		requests = append(requests, body)
		mu.Unlock()
		if _, ok := body["max_output_tokens"]; ok {
			http.Error(w, `{"error":{"message":"Unsupported parameter: max_output_tokens"}}`, 400)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"message\",\"id\":\"msg-fixture\"}}\n\n")
		fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello from fixture\"}\n\n")
		fmt.Fprint(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"fixture-response\",\"status\":\"completed\",\"usage\":{\"input_tokens\":5,\"output_tokens\":3,\"total_tokens\":8}}}\n\n")
	}))
	defer server.Close()
	id := fmt.Sprintf("gi-codex-wire-fixture-%d", time.Now().UnixNano())
	goai.RegisterModel(&goai.Model{ID: id, Provider: "openai-codex", Api: goai.ApiOpenAICodexResponses, BaseURL: server.URL, ContextWindow: 32768, MaxTokens: 4096, Input: []string{"text"}})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hookCalls := 0
	result, err := StreamWithToolsWithHooks(ctx, "openai-codex/"+id, &goai.Context{Messages: []goai.Message{{Role: "user", Content: []goai.ContentBlock{{Type: "text", Text: "hello"}}}}}, nil, &StreamHooks{OnPayload: func(payload any, model *goai.Model) (any, error) { hookCalls++; return nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Text != "Hello from fixture" {
		t.Fatalf("unexpected result %#v", result)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 1 {
		t.Fatalf("POST requests=%d", len(requests))
	}
	if hookCalls < 1 {
		t.Fatal("user payload hook not called")
	}
	if _, ok := requests[0]["max_output_tokens"]; ok {
		t.Fatal("unsupported parameter sent")
	}
}
