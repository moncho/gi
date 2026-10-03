package mcp

import (
	"fmt"
	"strings"
)

// CommandUsage is Pi's MCP_USAGE.
const CommandUsage = "Usage: /mcp, /mcp login [server], /mcp logout [server], /mcp reconnect [server]"

// FormatStatus ports Pi's formatStatus, the /mcp text without a TUI: one
// line per server with its state, tool count and exposure, connection errors
// indented below it, then config errors.
func FormatStatus(statuses []Status, errs []error) string {
	if len(statuses) == 0 && len(errs) == 0 {
		return "No MCP servers configured. Add them to " + UserConfigPath() + " or .pi/mcp.json."
	}
	var lines []string
	for _, st := range statuses {
		exposure := st.Exposure
		if exposure == "" {
			exposure = ExposureCodemode
		}
		if st.State == StateNeedsAuth {
			lines = append(lines, fmt.Sprintf("%s: needs sign-in, run /mcp login %s (%s)", st.Name, st.Name, exposure))
			continue
		}
		tools := ""
		if st.State == StateConnected {
			tools = fmt.Sprintf(", %d tools", st.Tools)
		}
		state := st.State
		switch st.State {
		case StateDisconnected:
			state = "disconnected, reconnects on next call"
		case "":
			state = "starting"
		}
		errText := ""
		if st.Error != "" && st.State != StateConnected {
			errText = "\n    " + strings.ReplaceAll(st.Error, "\n", "\n    ")
		}
		lines = append(lines, fmt.Sprintf("%s: %s%s (%s)%s", st.Name, state, tools, exposure, errText))
	}
	for _, err := range errs {
		lines = append(lines, "config error: "+err.Error())
	}
	return strings.Join(lines, "\n")
}
