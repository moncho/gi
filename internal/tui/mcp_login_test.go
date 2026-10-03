package tui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gimcp "github.com/rcarmo/gi/internal/mcp"
	"github.com/rcarmo/gi/internal/mcp/mcptest"
)

// /mcp login signs in from the session: the link is shown, the editor asks
// for a pasted redirect URL, and the server reconnects with its tools.
func TestTUIMCPLoginPastedRedirect(t *testing.T) {
	f := mcptest.NewOAuth(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(cfgPath, []byte(fmt.Sprintf(`{"mcpServers": {"remote": {"url": %q}}}`, f.URL+"/mcp")), 0o600); err != nil {
		t.Fatal(err)
	}
	c := sessionTestChat(t)
	c.uiQueue = make(chan func(), 16)
	previous := openBrowser
	openBrowser = func(string) {}
	t.Cleanup(func() { openBrowser = previous })
	c.engine.EnableMCPConfig(gimcp.LoadConfig(cfgPath, "", false), gimcp.NewCredentialStore(filepath.Join(dir, "mcp-auth.json")), "")
	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !cond() {
			select {
			case fn := <-c.uiQueue:
				fn()
				continue
			default:
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s:\n%s", what, strings.Join(c.transcript, "\n"))
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	waitFor("needs-auth", func() bool {
		st, _ := c.engine.MCPStatus()
		return len(st) == 1 && st[0].State == gimcp.StateNeedsAuth
	})
	if out := c.mcpCommand([]string{"/mcp"}); !strings.Contains(out[0], "remote: needs sign-in, run /mcp login remote") {
		t.Fatal(out)
	}
	c.mcpCommand([]string{"/mcp", "login"})
	var authURL string
	waitFor("the sign-in link", func() bool {
		for _, line := range c.transcript {
			if strings.HasPrefix(line, "(") && strings.Contains(line, "/authorize?") {
				authURL = strings.Trim(line, "()")
			}
		}
		return authURL != "" && c.editorAskActive
	})
	// The browser could not reach this machine: follow the authorization
	// redirect without the callback and paste the URL it points to.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	c.completeEditorAsk(resp.Header.Get("Location"))
	waitFor("signed in", func() bool {
		return strings.Contains(strings.Join(c.transcript, "\n"), `Signed in to MCP server "remote" (4 tools).`)
	})
	if out := c.mcpCommand([]string{"/mcp", "logout", "remote"}); out[0] != `Signed out of MCP server "remote".` {
		t.Fatal(out)
	}
	st, _ := c.engine.MCPStatus()
	if st[0].State != gimcp.StateNeedsAuth {
		t.Fatalf("after logout: %+v", st[0])
	}
}

// Esc at the sign-in prompt cancels the sign-in (Pi: "Sign-in cancelled.").
func TestTUIMCPLoginCancel(t *testing.T) {
	f := mcptest.NewOAuth(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(cfgPath, []byte(fmt.Sprintf(`{"mcpServers": {"remote": {"url": %q}}}`, f.URL+"/mcp")), 0o600); err != nil {
		t.Fatal(err)
	}
	c := sessionTestChat(t)
	c.uiQueue = make(chan func(), 16)
	previous := openBrowser
	openBrowser = func(string) {}
	t.Cleanup(func() { openBrowser = previous })
	c.engine.EnableMCPConfig(gimcp.LoadConfig(cfgPath, "", false), gimcp.NewCredentialStore(filepath.Join(dir, "mcp-auth.json")), "")
	c.mcpCommand([]string{"/mcp", "login", "remote"})
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(strings.Join(c.transcript, "\n"), "Sign-in cancelled.") {
		select {
		case fn := <-c.uiQueue:
			fn()
			if c.editorAskActive && c.editorAskKey == "mcp-login" {
				c.cancelEditorAsk()
			}
			continue
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("not cancelled:\n%s", strings.Join(c.transcript, "\n"))
		}
		time.Sleep(20 * time.Millisecond)
	}
	if c.mcpSignIn != nil || c.editorAskActive {
		t.Fatal("sign-in state left behind")
	}
}

// Pi's startup report reaches the transcript once servers finished their
// first connection attempt: a server needing sign-in is listed as a warning.
func TestTUIMCPStartupReport(t *testing.T) {
	f := mcptest.NewOAuth(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(cfgPath, []byte(fmt.Sprintf(`{"mcpServers": {"remote": {"url": %q}}}`, f.URL+"/mcp")), 0o600); err != nil {
		t.Fatal(err)
	}
	c := sessionTestChat(t)
	c.uiQueue = make(chan func(), 16)
	c.watchMCPNotices()
	c.engine.EnableMCPConfig(gimcp.LoadConfig(cfgPath, "", false), gimcp.NewCredentialStore(filepath.Join(dir, "mcp-auth.json")), "")
	want := []string{"Warning: MCP servers need attention:", "  remote: needs sign-in", "Run /mcp to fix."}
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(strings.Join(c.transcript, "\n"), strings.Join(want, "\n")) {
		select {
		case fn := <-c.uiQueue:
			fn()
			continue
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("no startup report:\n%s", strings.Join(c.transcript, "\n"))
		}
		time.Sleep(20 * time.Millisecond)
	}
}
