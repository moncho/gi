package tui

import (
	"context"
	"fmt"
	"strings"
)

// codemodeCommand handles /codemode [on|off|only|default|status]: the
// session toggle for the codemode tool (#25).
func (c *chatTUI) codemodeCommand(fields []string) []string {
	if c.engine == nil || c.sessionID == "" {
		return []string{"codemode: no active session"}
	}
	ctx := context.Background()
	arg := "status"
	if len(fields) > 1 {
		arg = strings.ToLower(strings.TrimSpace(fields[1]))
	}
	switch arg {
	case "status":
	case "on", "off", "only", "default":
		mode := arg
		if mode == "default" {
			mode = ""
		}
		if err := c.engine.SetSessionCodemode(ctx, c.sessionID, mode); err != nil {
			return []string{"codemode: " + err.Error()}
		}
	default:
		return []string{"usage: /codemode [on|off|only|default|status]"}
	}
	enabled, mode, source := c.engine.CodemodeStatus(ctx, c.sessionID)
	state := "off"
	if enabled {
		state = "on (" + mode + ")"
	}
	lines := []string{fmt.Sprintf("codemode: %s for this session (from %s); applies from the next prompt", state, source)}
	if warning := c.engine.CodemodeWarning(ctx, c.sessionID); warning != "" {
		lines = append(lines, warning)
	}
	return lines
}
