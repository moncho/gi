package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

// /import asks Pi's confirmation, then the Pi session becomes a new session
// the TUI switches to; No or Escape cancels; a missing file or no path is
// Pi's error.
func TestImportCommand(t *testing.T) {
	raw, err := os.ReadFile("../sessionimport/testdata/pi-sessions.json")
	if err != nil {
		t.Fatal(err)
	}
	var g map[string]struct{ JSONL string }
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	c := sessionTestChat(t)
	c.cfg.WorkspaceRoot = t.TempDir()
	if err := os.WriteFile(filepath.Join(c.cfg.WorkspaceRoot, "pi.jsonl"), []byte(g["branched"].JSONL), 0o644); err != nil {
		t.Fatal(err)
	}
	last := func() string { return c.transcript[len(c.transcript)-1] }
	c.appendTranscript(c.importCommand("/import")...)
	if last() != "error: Usage: /import <path.jsonl>" {
		t.Fatal(last())
	}
	c.appendTranscript(c.importCommand("/import pi.jsonl")...)
	if c.modelMenuKind != "select" || !strings.HasPrefix(c.selectDialog.title, "Import session\nReplace current session with pi.jsonl?") {
		t.Fatalf("menu %q", c.modelMenuKind)
	}
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyDown})
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEnter}) // No
	if last() != "sys: Import cancelled" || c.sessionID != "A" {
		t.Fatalf("%s session %s", last(), c.sessionID)
	}
	c.appendTranscript(c.importCommand("/import pi.jsonl")...)
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEnter}) // Yes
	if c.sessionID == "A" || last() != "sys: Session imported from: pi.jsonl" {
		t.Fatalf("session %s: %s", c.sessionID, last())
	}
	transcript := strings.Join(c.transcript, "\n")
	for _, want := range []string{"Continuing.", "Look at this"} {
		if !strings.Contains(transcript, want) {
			t.Fatalf("transcript lacks %q:\n%s", want, transcript)
		}
	}
	if sess, _ := c.store.GetSession(t.Context(), c.sessionID); sess.Title != "Parser work" {
		t.Fatalf("title %q", sess.Title)
	}
	c.appendTranscript(c.importCommand("/import missing.jsonl")...)
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEnter})
	if want := "error: Failed to import session: File not found: " + filepath.Join(c.cfg.WorkspaceRoot, "missing.jsonl"); last() != want {
		t.Fatal(last())
	}
}
