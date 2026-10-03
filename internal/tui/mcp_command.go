package tui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	gimcp "github.com/rcarmo/gi/internal/mcp"
)

// mcpUsage is Pi's /mcp usage line.
const mcpUsage = "Usage: /mcp, /mcp login [server], /mcp logout [server], /mcp reconnect [server]"

// mcpCommand ports Pi's /mcp in its non-interactive form: the server status
// list, sign-in, sign-out and reconnect, with Pi's server picker when the
// name is omitted and ambiguous.
func (c *chatTUI) mcpCommand(fields []string) []string {
	args := fields[1:]
	if len(args) == 0 {
		return strings.Split(c.mcpStatusText(), "\n")
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
		pick := c.mcpOAuthPick()
		if action == "reconnect" {
			pick = mcpReconnectPick
		}
		server, candidates, message := c.pickMCPServer(name, pick)
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

// mcpStatusText ports Pi's formatStatus.
func (c *chatTUI) mcpStatusText() string {
	statuses, errs := c.engine.MCPStatus()
	if len(statuses) == 0 && len(errs) == 0 {
		return "No MCP servers configured. Add them to " + gimcp.UserConfigPath() + "."
	}
	var lines []string
	for _, st := range statuses {
		if st.State == gimcp.StateNeedsAuth {
			lines = append(lines, fmt.Sprintf("%s: needs sign-in, run /mcp login %s (%s)", st.Name, st.Name, st.Exposure))
			continue
		}
		tools := ""
		if st.State == gimcp.StateConnected {
			tools = fmt.Sprintf(", %d tools", st.Tools)
		}
		state := st.State
		if st.State == gimcp.StateDisconnected {
			state = "disconnected, reconnects on next call"
		}
		errText := ""
		if st.Error != "" && st.State != gimcp.StateConnected {
			errText = "\n    " + strings.ReplaceAll(st.Error, "\n", "\n    ")
		}
		lines = append(lines, fmt.Sprintf("%s: %s%s (%s)%s", st.Name, state, tools, st.Exposure, errText))
	}
	for _, err := range errs {
		lines = append(lines, "config error: "+err.Error())
	}
	return strings.Join(lines, "\n")
}

const mcpOAuthNone = "No enabled MCP server uses OAuth. Only HTTP servers without an Authorization header do."

// mcpPick is Pi's pickServer options: which servers qualify, which one is
// preferred when several do, and the message when none does.
type mcpPick struct {
	eligible  func(gimcp.Status) bool
	preferred func(gimcp.Status) bool
	none      string
}

var mcpReconnectPick = mcpPick{
	eligible:  func(st gimcp.Status) bool { return st.State != gimcp.StateDisabled },
	preferred: func(st gimcp.Status) bool { return st.State == gimcp.StateFailed || st.State == gimcp.StateDisconnected },
	none:      "No enabled MCP server to reconnect.",
}

func (c *chatTUI) mcpOAuthPick() mcpPick {
	return mcpPick{
		eligible:  func(st gimcp.Status) bool { return c.engine.MCPUsesOAuth(st.Name) },
		preferred: func(st gimcp.Status) bool { return st.State == gimcp.StateNeedsAuth },
		none:      mcpOAuthNone,
	}
}

// pickMCPServer ports Pi's pickServer: the named server, else the only
// eligible one, else the only preferred one. Otherwise it returns the
// candidates for a picker; a message means there is nothing to pick.
func (c *chatTUI) pickMCPServer(name string, pick mcpPick) (server string, candidates []string, message string) {
	statuses, _ := c.engine.MCPStatus()
	if name != "" {
		for _, st := range statuses {
			if st.Name == name {
				if !pick.eligible(st) {
					return "", nil, pick.none
				}
				return name, nil, ""
			}
		}
		return "", nil, fmt.Sprintf("No MCP server named %q.", name)
	}
	var preferred []string
	for _, st := range statuses {
		if pick.eligible(st) {
			candidates = append(candidates, st.Name)
			if pick.preferred(st) {
				preferred = append(preferred, st.Name)
			}
		}
	}
	switch {
	case len(candidates) == 0:
		return "", nil, pick.none
	case len(candidates) == 1:
		return candidates[0], nil, ""
	case len(preferred) == 1:
		return preferred[0], nil, ""
	}
	return "", candidates, ""
}

// runMCPAction runs a /mcp action on a resolved server.
func (c *chatTUI) runMCPAction(action, server string) []string {
	switch action {
	case "login":
		return c.startMCPSignIn(server)
	case "logout":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if c.engine.MCPSignOut(ctx, server) {
			return []string{fmt.Sprintf("Signed out of MCP server %q.", server)}
		}
		return []string{fmt.Sprintf("No stored credentials for MCP server %q.", server)}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := c.engine.MCPReconnect(ctx, server); err != nil {
		return []string{err.Error()}
	}
	statuses, _ := c.engine.MCPStatus()
	for _, st := range statuses {
		if st.Name == server {
			return []string{fmt.Sprintf("Reconnected to MCP server %q (%s).", server, gimcp.DescribeState(st))}
		}
	}
	return []string{fmt.Sprintf("Reconnected to MCP server %q.", server)}
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
			switch {
			case errors.Is(err, gimcp.ErrSignInCancelled):
				c.appendTranscript("Sign-in cancelled.")
			case err != nil:
				c.appendTranscript(err.Error())
			default:
				tools := 0
				statuses, _ := c.engine.MCPStatus()
				for _, st := range statuses {
					if st.Name == server {
						tools = st.Tools
					}
				}
				c.appendTranscript(fmt.Sprintf("Signed in to MCP server %q (%d tools).", server, tools))
			}
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
