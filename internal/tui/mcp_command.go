package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gimcp "github.com/rcarmo/gi/internal/mcp"
)

// mcpUsage is Pi's /mcp usage line.
const mcpUsage = "Usage: /mcp, /mcp login [server], /mcp logout [server], /mcp reconnect [server]"

// mcpCommand ports Pi's /mcp in its non-interactive form: the server status
// list and /mcp reconnect. Sign-in waits for MCP OAuth (#25 phase 6c).
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
	case "login", "logout":
		server, problem := c.pickOAuthServer(name)
		if problem != "" {
			return []string{problem}
		}
		if action == "logout" {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if c.engine.MCPSignOut(ctx, server) {
				return []string{fmt.Sprintf("Signed out of MCP server %q.", server)}
			}
			return []string{fmt.Sprintf("No stored credentials for MCP server %q.", server)}
		}
		return c.startMCPSignIn(server)
	case "reconnect":
		statuses, _ := c.engine.MCPStatus()
		if name == "" {
			// Pi asks when ambiguous; prefer the one failed or disconnected server.
			var candidates, preferred []string
			for _, st := range statuses {
				if st.State == gimcp.StateDisabled {
					continue
				}
				candidates = append(candidates, st.Name)
				if st.State == gimcp.StateFailed || st.State == gimcp.StateDisconnected {
					preferred = append(preferred, st.Name)
				}
			}
			switch {
			case len(candidates) == 0:
				return []string{"No enabled MCP server to reconnect."}
			case len(candidates) == 1:
				name = candidates[0]
			case len(preferred) == 1:
				name = preferred[0]
			default:
				return []string{"Which MCP server? Use /mcp reconnect <server>: " + strings.Join(candidates, ", ")}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := c.engine.MCPReconnect(ctx, name); err != nil {
			return []string{err.Error()}
		}
		statuses, _ = c.engine.MCPStatus()
		for _, st := range statuses {
			if st.Name == name {
				return []string{fmt.Sprintf("Reconnected to MCP server %q (%s).", name, describeMCPState(st))}
			}
		}
		return []string{fmt.Sprintf("Reconnected to MCP server %q.", name)}
	}
	return []string{mcpUsage}
}

// describeMCPState ports Pi's describeState (without resource counts).
func describeMCPState(st gimcp.Status) string {
	switch st.State {
	case gimcp.StateDisabled:
		return "disabled"
	case gimcp.StateFailed:
		msg := st.Error
		if msg == "" {
			msg = "unknown error"
		}
		return "failed: " + strings.SplitN(msg, "\n", 2)[0]
	case gimcp.StateConnected:
		plural := "s"
		if st.Tools == 1 {
			plural = ""
		}
		return fmt.Sprintf("connected · %d tool%s", st.Tools, plural)
	case gimcp.StateNeedsAuth:
		return "needs sign-in"
	case gimcp.StateConnecting:
		return "connecting…"
	}
	return st.State
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

// pickOAuthServer ports Pi's pickServer for login/logout: the named server,
// else the only OAuth server, else the only one needing sign-in.
func (c *chatTUI) pickOAuthServer(name string) (string, string) {
	statuses, _ := c.engine.MCPStatus()
	if name != "" {
		for _, st := range statuses {
			if st.Name == name {
				if !c.engine.MCPUsesOAuth(name) {
					return "", mcpOAuthNone
				}
				return name, ""
			}
		}
		return "", fmt.Sprintf("No MCP server named %q.", name)
	}
	var candidates, preferred []string
	for _, st := range statuses {
		if c.engine.MCPUsesOAuth(st.Name) {
			candidates = append(candidates, st.Name)
			if st.State == gimcp.StateNeedsAuth {
				preferred = append(preferred, st.Name)
			}
		}
	}
	switch {
	case len(candidates) == 0:
		return "", mcpOAuthNone
	case len(candidates) == 1:
		return candidates[0], ""
	case len(preferred) == 1:
		return preferred[0], ""
	}
	return "", "Which MCP server? Use /mcp login <server>: " + strings.Join(candidates, ", ")
}

// mcpSignInState is a sign-in waiting for the browser or a pasted URL.
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
	ui := func(fn func()) {
		switch {
		case c.app != nil:
			c.app.QueueUpdate(func() { fn(); c.markDirty() })
		case c.uiQueue != nil: // tests run updates on their own goroutine
			c.uiQueue <- fn
		default:
			fn()
		}
	}
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
