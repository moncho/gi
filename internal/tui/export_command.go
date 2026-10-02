package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcarmo/gi/internal/sessionexport"
)

// exportCommand is Pi's /export [path]: HTML by default, JSONL (Pi session
// format, importable by Pi) when the path ends in .jsonl.
func (c *chatTUI) exportCommand(text string) string {
	if c.store == nil || c.sessionID == "" {
		return "error: no active session to export"
	}
	path := pathCommandArgument(text, "/export")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	written, err := sessionexport.Export(ctx, c.store, c.sessionID, c.cfg.WorkspaceRoot, path)
	if err != nil {
		return fmt.Sprintf("error: Failed to export session: %v", err)
	}
	return "sys: Session exported to: " + written
}

// pathCommandArgument ports Pi's getPathCommandArgument: the first argument,
// optionally quoted with ' or ".
func pathCommandArgument(text, command string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, command+" ") {
		return ""
	}
	args := strings.TrimLeft(text[len(command)+1:], " \t")
	if args == "" {
		return ""
	}
	if q := args[0]; q == '"' || q == '\'' {
		if end := strings.IndexByte(args[1:], q); end >= 0 {
			return args[1 : end+1]
		}
		return ""
	}
	if i := strings.IndexAny(args, " \t"); i >= 0 {
		return args[:i]
	}
	return args
}
