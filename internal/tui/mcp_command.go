package tui

import (
	"context"
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
		// The browser flow runs from a shell for now; reconnect afterwards.
		target := name
		if target == "" {
			target = "<server>"
		}
		return []string{fmt.Sprintf("Run `gi mcp %s %s` in a shell, then /mcp reconnect %s.", action, target, target)}
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
			lines = append(lines, fmt.Sprintf("%s: needs sign-in, run gi mcp login %s (%s)", st.Name, st.Name, st.Exposure))
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
