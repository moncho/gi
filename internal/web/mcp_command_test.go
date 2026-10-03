package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/config"
	gimcp "github.com/rcarmo/gi/internal/mcp"
	"github.com/rcarmo/gi/internal/mcp/mcptest"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/turn"
)

// /mcp in the web UI: Pi's status text, sign-in with a pasted redirect URL,
// reconnect and sign-out, replied as system messages that are not model
// context.
func TestWebMCPCommand(t *testing.T) {
	f := mcptest.NewOAuth(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(cfgPath, []byte(fmt.Sprintf(`{"mcpServers": {"remote": {"url": %q}}}`, f.URL+"/mcp")), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(dir, "gi.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	engine := turn.New(db)
	t.Cleanup(func() { engine.Close() })
	engine.EnableMCPConfig(gimcp.LoadConfig(cfgPath, "", false), gimcp.NewCredentialStore(filepath.Join(dir, "mcp-auth.json")), "")
	srv := New(db, engine, config.RuntimeConfig{DefaultProvider: "test", DefaultModel: "test-model"})
	session, err := db.CreateSession(context.Background(), "s1", "mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	events := engine.Subscribe(session.ID)
	send := func(prompt string) {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"prompt": prompt})
		res := httptest.NewRecorder()
		srv.Handler().ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/sessions/"+session.ID+"/prompt", bytes.NewReader(body)))
		if res.Code != 200 || !strings.Contains(res.Body.String(), `"ui_only":true`) {
			t.Fatalf("%s: %d %s", prompt, res.Code, res.Body.String())
		}
	}
	seen := 0
	// next waits for the next system reply containing want.
	next := func(want string) string {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			msgs, _ := db.ListMessages(context.Background(), session.ID)
			for ; seen < len(msgs); seen++ {
				m := msgs[seen]
				if m.Role != "system" || m.Payload["kind"] != "mcp" {
					t.Fatalf("reply %+v", m)
				}
				if strings.Contains(m.Content, want) {
					seen++
					return m.Content
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("no reply containing %q", want)
		return ""
	}
	deadline := time.Now().Add(10 * time.Second)
	for st, _ := engine.MCPStatus(); len(st) != 1 || st[0].State != gimcp.StateNeedsAuth; st, _ = engine.MCPStatus() {
		if time.Now().After(deadline) {
			t.Fatalf("status %+v", st)
		}
		time.Sleep(20 * time.Millisecond)
	}
	send("/mcp")
	if got := next("remote"); got != "remote: needs sign-in, run /mcp login remote (codemode)" {
		t.Fatalf("status %q", got)
	}
	select {
	case ev := <-events:
		if ev["type"] != "new_post" || ev["sender"] != "system" {
			t.Fatalf("event %+v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no new_post event")
	}
	send("/mcp frob")
	if got := next("Usage"); got != gimcp.CommandUsage {
		t.Fatal(got)
	}
	send("/mcp login remote http://127.0.0.1/callback")
	next(`No sign-in to MCP server "remote" is waiting for a redirect URL.`)

	send("/mcp login")
	link := next("Sign in to MCP server")
	authURL := strings.Split(link, "\n")[1]
	if !strings.Contains(authURL, "/authorize?") || !strings.Contains(link, "run /mcp login remote <the URL it was redirected to>") {
		t.Fatalf("link %q", link)
	}
	send("/mcp login")
	next(`Already signing in to MCP server "remote".`)
	// The browser could not reach gi: paste the URL it was redirected to.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	send("/mcp login remote " + resp.Header.Get("Location"))
	next(`Signed in to MCP server "remote" (4 tools).`)

	send("/mcp reconnect")
	next(`Reconnected to MCP server "remote" (connected · 4 tools).`)
	send("/mcp logout remote")
	next(`Signed out of MCP server "remote".`)

	// Replies are timeline messages, not model context.
	snapshot, err := db.ContextSnapshot(context.Background(), session.ID)
	if err != nil || len(snapshot.Messages) != 0 {
		t.Fatalf("context %+v %v", snapshot.Messages, err)
	}
}
