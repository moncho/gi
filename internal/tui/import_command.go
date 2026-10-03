package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gisession "github.com/rcarmo/gi/internal/session"
	"github.com/rcarmo/gi/internal/sessionimport"
	"github.com/rcarmo/gi/internal/store"
)

// importCommand is Pi's /import <path.jsonl>: after confirming, the Pi
// session file becomes a new gi session, which replaces the current one in
// the TUI (the current session stays listed in /resume).
func (c *chatTUI) importCommand(text string) []string {
	path := pathCommandArgument(text, "/import")
	if path == "" {
		return []string{"error: Usage: /import <path.jsonl>"}
	}
	if c.store == nil {
		return []string{"error: Failed to import session: no session store"}
	}
	c.openSelect("Import session\nReplace current session with "+path+"?", []string{"Yes", "No"}, func(choice string) {
		if choice != "Yes" {
			c.appendTranscript("sys: Import cancelled")
			return
		}
		c.appendTranscript(c.importSession(path)...)
	}, func() { c.appendTranscript("sys: Import cancelled") })
	return nil
}

// resolveUserPath is Pi's resolvePath: ~ is the home directory, relative
// paths are against the workspace.
func resolveUserPath(path, cwd string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	return filepath.Clean(path)
}

// importSession imports path into a new session and switches to it.
func (c *chatTUI) importSession(path string) []string {
	resolved := resolveUserPath(path, c.cfg.WorkspaceRoot)
	if _, err := os.Stat(resolved); err != nil {
		return []string{"error: Failed to import session: File not found: " + resolved}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	id := store.NowID("session")
	alloc := gisession.AllocateDefaultSession("agent", "gi", "default", id)
	state := map[string]any{"status": "idle", "queue_count": 0, "model": c.cfg.DefaultModel, "provider": c.cfg.DefaultProvider, "thinking_level": c.cfg.DefaultThinkingLevel}
	sess, _, err := c.store.ResolveOrCreateMainSessionFromAllocation(ctx, store.ResolveOrCreateSessionFromAllocationInput{ID: id, Title: "@agent", State: state, Allocation: alloc})
	if err != nil {
		return []string{fmt.Sprintf("error: Failed to import session: %v", err)}
	}
	if _, err := sessionimport.ImportFile(ctx, c.store, resolved, sess.ID); err != nil {
		if errors.Is(err, sessionimport.ErrNotSession) {
			return []string{fmt.Sprintf("error: Failed to import session: %s: %v", resolved, err)}
		}
		return []string{fmt.Sprintf("error: Failed to import session: %v", err)}
	}
	c.switchSession(sess.ID)
	return []string{"sys: Session imported from: " + path}
}
