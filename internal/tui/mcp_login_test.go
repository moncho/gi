package tui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"
	gimcp "github.com/rcarmo/gi/internal/mcp"
	"github.com/rcarmo/gi/internal/mcp/mcptest"
)

// mcpLoginTest is a chat with one OAuth MCP server needing sign-in.
func mcpLoginTest(t *testing.T) (*chatTUI, func(string, func() bool)) {
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
	return c, waitFor
}

// /mcp login signs in on the manager's sign-in screen (Pi 1.0.1): it shows
// the link, takes the URL the browser was redirected to, and the server
// reconnects with its tools.
func TestTUIMCPLoginPastedRedirect(t *testing.T) {
	c, waitFor := mcpLoginTest(t)
	c.mcpCommand([]string{"/mcp"})
	if menu := c.mcpManagerMenu(); len(menu.items) != 1 || menu.items[0].description != "needs sign-in · codemode · global" {
		t.Fatalf("%+v", menu.items)
	}
	c.closeMCPManager()
	c.mcpCommand([]string{"/mcp", "login"})
	if c.mcpManager == nil || c.mcpManager.status[1] != "Contacting the authorization server…" {
		t.Fatalf("manager %+v", c.mcpManager)
	}
	waitFor("the sign-in screen", func() bool { return c.mcpManager != nil && c.mcpManager.signin != nil })
	authURL := c.mcpManager.signin.url.url
	if !strings.Contains(authURL, "/authorize?") {
		t.Fatalf("authorization URL %q", authURL)
	}
	// The browser could not reach this machine: follow the authorization
	// redirect without the callback and paste the URL it points to.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	keys := c.KeyMap()
	pressMenuKey(t, keys, gotui.KeyEvent{Key: gotui.KeyEnter}) // nothing to submit yet
	if c.mcpManager.signin == nil {
		t.Fatal("empty input submitted")
	}
	for _, ev := range runeEvents(resp.Header.Get("Location")) {
		pressMenuKey(t, keys, ev)
	}
	pressMenuKey(t, keys, gotui.KeyEvent{Key: gotui.KeyEnter})
	waitFor("signed in", func() bool {
		return strings.Contains(strings.Join(c.transcript, "\n"), `Signed in to MCP server "remote" (4 tools).`)
	})
	if c.mcpManager != nil || c.modelMenuOpen {
		t.Fatal("the manager stayed open")
	}
	if out := c.mcpCommand([]string{"/mcp", "logout", "remote"}); out[0] != `Signed out of MCP server "remote".` {
		t.Fatal(out)
	}
	st, _ := c.engine.MCPStatus()
	if st[0].State != gimcp.StateNeedsAuth {
		t.Fatalf("after logout: %+v", st[0])
	}
}

// Escape on the sign-in screen cancels the sign-in (Pi: "Sign-in
// cancelled."); in the manager the message shows on the server's screen.
func TestTUIMCPLoginCancel(t *testing.T) {
	c, waitFor := mcpLoginTest(t)
	c.mcpCommand([]string{"/mcp", "login", "remote"})
	waitFor("the sign-in screen", func() bool { return c.mcpManager != nil && c.mcpManager.signin != nil })
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEscape})
	waitFor("cancelled", func() bool { return strings.Contains(strings.Join(c.transcript, "\n"), "Sign-in cancelled.") })
	if c.mcpManager != nil {
		t.Fatal("the manager stayed open")
	}

	c.mcpCommand([]string{"/mcp"})
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEnter}) // remote
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEnter}) // Sign in
	waitFor("the sign-in screen", func() bool { return c.mcpManager.signin != nil })
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEscape})
	waitFor("back on the server", func() bool { return c.mcpManager.status[0] == "" && c.mcpManager.messages["remote"] != "" })
	if menu := c.mcpManagerMenu(); c.mcpManager.screen != "server" || menu.errorText != "Sign-in cancelled." {
		t.Fatalf("screen %q error %q", c.mcpManager.screen, menu.errorText)
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
