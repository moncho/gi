package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	gimcp "github.com/rcarmo/gi/internal/mcp"
)

// mcpUsage is Pi's /mcp usage line.
const mcpUsage = gimcp.CommandUsage

// mcpCommand ports Pi's /mcp: the manager without arguments; sign-in,
// sign-out and reconnect, with Pi's server picker when the name is omitted
// and ambiguous.
func (c *chatTUI) mcpCommand(fields []string) []string {
	args := fields[1:]
	if len(args) == 0 {
		c.openMCPManager() // Pi's manager (Pi prints formatStatus only without a TUI)
		return nil
	}
	if len(args) > 2 {
		return []string{mcpUsage}
	}
	action, name := args[0], ""
	if len(args) == 2 {
		name = args[1]
	}
	switch action {
	case "login", "logout", "reconnect":
		server, candidates, message := c.engine.MCPPickServer(action, name)
		switch {
		case message != "":
			return []string{message}
		case server == "":
			// Pi asks with ctx.ui.select; cancelling does nothing.
			c.openSelect("MCP server", candidates, func(choice string) {
				c.appendTranscript(c.runMCPAction(action, choice)...)
			}, nil)
			return nil
		}
		return c.runMCPAction(action, server)
	}
	return []string{mcpUsage}
}

// runMCPAction runs a /mcp action on a resolved server.
func (c *chatTUI) runMCPAction(action, server string) []string {
	switch action {
	case "login":
		return c.startMCPSignIn(server)
	case "logout":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return []string{c.engine.MCPLogoutCommand(ctx, server)}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return []string{c.engine.MCPReconnectCommand(ctx, server)}
}

type mcpSignInState struct {
	server string
	cancel context.CancelFunc
	paste  chan string
}

// startMCPSignIn runs Pi's in-session sign-in: it shows the authorization
// link, opens the browser, and asks in the editor for the redirect URL in
// case the browser cannot reach this machine (Esc cancels).
func (c *chatTUI) startMCPSignIn(server string) []string {
	if c.mcpSignIn != nil {
		return []string{fmt.Sprintf("Already signing in to MCP server %q; Esc cancels it.", c.mcpSignIn.server)}
	}
	ctx, cancel := context.WithCancel(context.Background())
	state := &mcpSignInState{server: server, cancel: cancel, paste: make(chan string, 1)}
	c.mcpSignIn = state
	ui := c.runOnUI
	prompt := gimcp.SignInPrompt{
		ShowAuthorizationURL: func(u string) {
			ui(func() {
				c.appendTranscript(fmt.Sprintf("Sign in to MCP server %q in your browser:", server), "("+u+")")
				c.editorAskHandler = func(answer string, cancelled bool) {
					if cancelled {
						state.cancel()
						return
					}
					select {
					case state.paste <- answer:
					default:
					}
				}
				c.setEditorAsk("mcp-login", fmt.Sprintf("Waiting for sign-in to %q. If the browser cannot reach this machine, paste the URL it was redirected to.", server), "")
			})
			openBrowser(u)
		},
		PromptForRedirectURL: func(ctx context.Context) string {
			select {
			case s := <-state.paste:
				return s
			case <-ctx.Done():
				return ""
			}
		},
	}
	go func() {
		err := c.engine.MCPSignIn(ctx, server, prompt)
		ui(func() {
			c.mcpSignIn = nil
			if c.editorAskActive && c.editorAskKey == "mcp-login" {
				c.editorAskHandler = nil
				c.exitEditorAsk()
			}
			c.appendTranscript(c.engine.MCPSignInResult(server, err))
		})
	}()
	return nil
}

// openBrowser opens sign-in pages (tests replace it).
var openBrowser = gimcp.OpenBrowser

// runOnUI runs fn on the UI goroutine (from background goroutines).
func (c *chatTUI) runOnUI(fn func()) {
	switch {
	case c.app != nil:
		c.app.QueueUpdate(func() { fn(); c.markDirty() })
	case c.uiQueue != nil: // tests run updates on their own goroutine
		c.uiQueue <- fn
	default:
		fn()
	}
}

// watchMCPNotices shows the engine's MCP notices (Pi's ctx.ui.notify):
// warnings as "Warning: …", others as plain lines.
func (c *chatTUI) watchMCPNotices() {
	if c.engine == nil {
		return
	}
	c.engine.SetMCPNotifier(func(level, text string) {
		c.runOnUI(func() { c.showQueueCommand(mcpNoticeLines(level, text)) })
	})
}

func mcpNoticeLines(level, text string) []string {
	if level == "warning" {
		text = "Warning: " + text
	}
	return strings.Split(text, "\n")
}

var mcpArgSplit = regexp.MustCompile(`\s+`)

// mcpArgumentCompletions ports Pi's /mcp getArgumentCompletions: actions,
// then the eligible servers with their state (OAuth servers for login and
// logout, enabled servers for reconnect). Like JavaScript's split, a
// trailing space yields an empty last field: "login " completes servers.
func (c *chatTUI) mcpArgumentCompletions(prefix string) []slashItem {
	parts := mcpArgSplit.Split(strings.TrimLeftFunc(prefix, unicode.IsSpace), -1)
	if len(parts) > 2 {
		return nil
	}
	action := parts[0]
	var items []slashItem
	if len(parts) == 1 {
		for _, item := range []string{"login", "logout", "reconnect"} {
			if strings.HasPrefix(item, action) {
				items = append(items, slashItem{name: item, value: item + " "})
			}
		}
		return items
	}
	if (action != "login" && action != "logout" && action != "reconnect") || c.engine == nil {
		return nil
	}
	statuses, _ := c.engine.MCPStatus()
	for _, st := range statuses {
		eligible := st.State != gimcp.StateDisabled
		if action != "reconnect" {
			eligible = c.engine.MCPUsesOAuth(st.Name)
		}
		if eligible && strings.HasPrefix(st.Name, parts[1]) {
			items = append(items, slashItem{name: st.Name, value: action + " " + st.Name, description: gimcp.DescribeState(st)})
		}
	}
	return items
}
