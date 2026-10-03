package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

// pressAppKey runs the chat's binding for ev with exactly ev's modifiers.
func pressAppKey(t *testing.T, c *chatTUI, ev gotui.KeyEvent) {
	t.Helper()
	for _, b := range c.KeyMap() {
		p := b.Pattern
		if p.AnyRune || p.Mod != ev.Mod {
			continue
		}
		if (p.Rune != 0 && ev.Key == gotui.KeyRune && p.Rune == ev.Rune) || (p.Rune == 0 && p.Key != gotui.KeyRune && p.Key == ev.Key) {
			b.Handler(ev)
			return
		}
	}
	t.Fatalf("no app binding for %+v", ev)
}

// Pi's default app.* keys (docs/internal/keybindings.md) reach Pi's
// actions: Ctrl+P/Shift+Ctrl+P cycle models, Ctrl+L opens the model
// selector, Shift+Tab cycles thinking, Ctrl+T hides thinking blocks (saved
// as hideThinkingBlock) and Ctrl+C clears the editor.
func TestAppKeysMatchPiDefaults(t *testing.T) {
	defer func(keys piKeys) { defaultPiKeys = keys }(defaultPiKeys)
	defaultPiKeys = piKeys{}
	c := sessionTestChat(t)
	c.cfg.WorkspaceRoot = t.TempDir()
	transcriptSince := func(fn func()) string {
		n := len(c.transcript)
		fn()
		return strings.Join(c.transcript[n:], "\n")
	}

	for _, ev := range []gotui.KeyEvent{{Key: gotui.KeyRune, Rune: 'p', Mod: gotui.ModCtrl}, {Key: gotui.KeyRune, Rune: 'p', Mod: gotui.ModCtrl | gotui.ModShift}} {
		if got := transcriptSince(func() { pressAppKey(t, c, ev) }); !strings.Contains(got, "no enabled models configured") {
			t.Fatalf("%+v did not cycle models: %q", ev, got)
		}
	}
	if got := transcriptSince(func() { pressAppKey(t, c, gotui.KeyEvent{Key: gotui.KeyRune, Rune: 'l', Mod: gotui.ModCtrl}) }); c.modelMenuKind == "" && !strings.Contains(got, "no available models") {
		t.Fatalf("Ctrl+L did not open the model selector: %q", got)
	}
	c.closeModelMenu()
	if got := transcriptSince(func() { pressAppKey(t, c, gotui.KeyEvent{Key: gotui.KeyTab, Mod: gotui.ModShift}) }); !strings.Contains(strings.ToLower(got), "thinking") {
		t.Fatalf("Shift+Tab did not cycle thinking: %q", got)
	}

	hidden := c.cfg.HideThinkingBlock
	pressAppKey(t, c, gotui.KeyEvent{Key: gotui.KeyRune, Rune: 't', Mod: gotui.ModCtrl})
	if c.cfg.HideThinkingBlock == hidden {
		t.Fatal("Ctrl+T did not toggle thinking blocks")
	}
	raw, _ := os.ReadFile(filepath.Join(c.cfg.WorkspaceRoot, ".pi", "settings.json"))
	if !strings.Contains(string(raw), `"hideThinkingBlock": true`) {
		t.Fatalf("hideThinkingBlock not saved: %s", raw)
	}

	c.input.SetText("draft")
	pressAppKey(t, c, gotui.KeyEvent{Key: gotui.KeyRune, Rune: 'c', Mod: gotui.ModCtrl})
	if c.input.Text() != "" || c.lastCtrlC.IsZero() {
		t.Fatalf("Ctrl+C did not clear the editor: %q", c.input.Text())
	}
}

// Ctrl+G's editor gets the draft as prompt.md; the editor takes the saved
// text without its trailing newline, and keeps the draft if the editor fails
// (Pi's editInExternalEditor).
func TestExternalEditorRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script editor")
	}
	dir := t.TempDir()
	editor := filepath.Join(dir, "editor")
	script := "#!/bin/sh\ncase \"$1\" in */prompt.md) ;; *) exit 2;; esac\n[ \"$(cat \"$1\")\" = draft ] || exit 3\nprintf 'edited\\n' > \"$1\"\n"
	if err := os.WriteFile(editor, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if text, ok := editInExternalEditor(editor, "draft"); !ok || text != "edited" {
		t.Fatalf("external editor: %q %v", text, ok)
	}
	if _, ok := editInExternalEditor(filepath.Join(dir, "missing"), "draft"); ok {
		t.Fatal("a failing editor replaced the draft")
	}
}
