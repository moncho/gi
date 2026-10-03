package inference

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	goai "github.com/rcarmo/go-ai"
)

// Pi's remote catalogue refresh (remote-catalog-provider.js): catalogues
// stored per provider with their validators, skipped while fresh,
// revalidated with If-None-Match, kept on failures, and new models
// registered.
func TestRefreshModelCatalogLikePi(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GI_CODING_AGENT_DIR", t.TempDir())
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	// Credentials: Copilot (OAuth, current) and OpenAI (environment key).
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{"github-copilot":{"type":"oauth","refresh":"r","access":"a","expires":9999999999999}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("MISTRAL_API_KEY", "")
	storePath := filepath.Join(dir, "models-store.json")
	if err := os.WriteFile(storePath, []byte(`{"custom-provider":{"models":[{"id":"kept"}],"checkedAt":1}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	requests := map[string][]*http.Request{}
	copilot := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Last-Modified", "Fri, 02 Oct 2026 10:00:00 GMT")
		w.Write([]byte(`{"models":[
			{"id":"gi-test-catalog-model-1","name":"Catalog","api":"openai-completions","baseUrl":"https://example.invalid","input":["text"],"contextWindow":1000,"maxTokens":100,"cost":{"input":0.1,"output":0.2,"cacheRead":0,"cacheWrite":0}},
			{"id":"embedder","type":"embedding"},
			{"name":"no id"}]}`))
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provider := strings.TrimPrefix(r.URL.Path, "/api/models/providers/")
		mu.Lock()
		requests[provider] = append(requests[provider], r)
		handler := copilot
		mu.Unlock()
		if r.URL.Query().Get("types") != "chat,image,classifier" {
			t.Errorf("types %q", r.URL.RawQuery)
		}
		if provider != "github-copilot" {
			http.NotFound(w, r)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	store := func() map[string]catalogEntry {
		raw, err := os.ReadFile(storePath)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]catalogEntry
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	count := func(provider string) int {
		mu.Lock()
		defer mu.Unlock()
		return len(requests[provider])
	}

	if err := RefreshModelCatalog(context.Background(), srv.URL, false); err != nil {
		t.Fatal(err)
	}
	entry := store()["github-copilot"]
	lastModified := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC).UnixMilli()
	if len(entry.Models) != 1 || entry.ETag != `"v1"` || entry.LastModified == nil || *entry.LastModified != lastModified || entry.CheckedAt == nil {
		t.Fatalf("copilot entry %+v", entry)
	}
	var model map[string]any
	_ = json.Unmarshal(entry.Models[0], &model)
	if model["provider"] != "github-copilot" || model["contextWindow"] != float64(1000) {
		t.Fatalf("model %v", model)
	}
	if goai.GetModel(goai.ProviderGitHubCopilot, "gi-test-catalog-model-1") == nil {
		t.Fatal("new model not registered")
	}
	missing := store()["openai"]
	if missing.LastModified == nil || *missing.LastModified != 0 || len(missing.Models) != 0 || missing.ETag != "" {
		t.Fatalf("404 entry %+v", missing)
	}
	var kept map[string]any
	if json.Unmarshal(store()["custom-provider"].Models[0], &kept) != nil || kept["id"] != "kept" {
		t.Fatal("other entries not kept")
	}
	if _, err := os.Stat(storePath + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("lock left behind")
	}
	if count("mistral") != 0 {
		t.Fatal("fetched for a provider without credentials")
	}

	// Fresh: nothing is fetched.
	if err := RefreshModelCatalog(context.Background(), srv.URL, false); err != nil || count("github-copilot") != 1 || count("openai") != 1 {
		t.Fatalf("refetched while fresh: %v", err)
	}

	// Forced: revalidated with the ETag; 304 keeps the body.
	mu.Lock()
	copilot = func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != `"v1"` {
			t.Errorf("If-None-Match %q", r.Header.Get("If-None-Match"))
		}
		w.WriteHeader(http.StatusNotModified)
	}
	mu.Unlock()
	if err := RefreshModelCatalog(context.Background(), srv.URL, true); err != nil {
		t.Fatal(err)
	}
	if after := store()["github-copilot"]; len(after.Models) != 1 || after.ETag != `"v1"` || *after.CheckedAt < *entry.CheckedAt {
		t.Fatalf("304 entry %+v", after)
	}
	if r := requests["openai"][1]; r.Header.Get("If-None-Match") != "" {
		t.Fatal("validator sent without a cached body")
	}

	// Server errors are retried twice, then reported; the cached body stays.
	mu.Lock()
	copilot = func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }
	mu.Unlock()
	before := count("github-copilot")
	err := RefreshModelCatalog(context.Background(), srv.URL, true)
	if err == nil || !strings.Contains(err.Error(), "model catalog request failed for github-copilot: 503") || count("github-copilot") != before+3 {
		t.Fatalf("%v, %d requests", err, count("github-copilot")-before)
	}
	if after := store()["github-copilot"]; len(after.Models) != 1 || after.ETag != `"v1"` {
		t.Fatalf("failure entry %+v", after)
	}
}

func TestParseCatalogShapes(t *testing.T) {
	for body, want := range map[string]int{
		`[{"id":"a"},{"id":"b","type":"image"}]`:        2,
		`{"models":[{"id":"a"}]}`:                       1,
		`{"x":{"id":"a"},"y":{"id":"b","type":"chat"}}`: 2,
		`{"models":{"id":"a"}}`:                         1, // Object.values: the inner object
	} {
		got, err := parseCatalog("p", json.RawMessage(body))
		if err != nil || len(got) != want {
			t.Errorf("%s: %d %v", body, len(got), err)
		}
	}
	if _, err := parseCatalog("p", json.RawMessage(`"nope"`)); err == nil {
		t.Error("string catalogue accepted")
	}
}
