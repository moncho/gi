package tui

import (
	"os"
	"strings"
	"testing"
)

// /hotkeys opens with Pi's own text for Pi's default keys on each platform
// (bun scripts/golden-hotkeys.mjs), then gi's table.
func TestHotkeysMatchPi(t *testing.T) {
	for file, keys := range map[string]piKeys{
		"testdata/pi-hotkeys.md":         {},
		"testdata/pi-hotkeys-darwin.md":  {darwin: true},
		"testdata/pi-hotkeys-wsl.md":     {wsl: true},
		"testdata/pi-hotkeys-windows.md": {windows: true},
	} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if got := keys.hotkeysMarkdown(); got != string(raw) {
			t.Fatalf("/hotkeys for %s differs from Pi's:\n%s\n--- Pi ---\n%s", file, got, raw)
		}
	}
	if got := (piKeys{wsl: true}).giHotkeysMarkdown(); !strings.Contains(got, "`Ctrl+F` | Search the transcript") {
		t.Fatalf("Windows keys search with Ctrl+F (Pi's tui.altScreen.search):\n%s", got)
	}
	c := sessionTestChat(t)
	lines := strings.Join(c.hotkeyLines(), "\n")
	for _, want := range []string{"Keyboard Shortcuts", "Navigation", "Ctrl+]", "Alt+Y", "Ctrl+T", "Ctrl+Up", "Alt+M"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("/hotkeys lacks %q:\n%s", want, lines)
		}
	}
}
