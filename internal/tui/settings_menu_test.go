package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

var settingsKeyEvents = map[string]gotui.KeyEvent{
	"up":        {Key: gotui.KeyUp},
	"down":      {Key: gotui.KeyDown},
	"enter":     {Key: gotui.KeyEnter},
	"space":     {Key: gotui.KeyRune, Rune: ' '},
	"backspace": {Key: gotui.KeyBackspace},
}

// /settings renders and changes values as Pi's SettingsList does, key by
// key (bun scripts/golden-settings.mjs).
func TestSettingsMenuMatchesPi(t *testing.T) {
	raw, err := os.ReadFile("testdata/pi-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]struct {
		Steps  []json.RawMessage `json:"steps"`
		Widths map[string][]struct {
			Step    string      `json:"step"`
			Rows    []string    `json:"rows"`
			Changes [][2]string `json:"changes"`
		} `json:"widths"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	for name, scenario := range golden {
		for w, states := range scenario.Widths {
			t.Run(name+"@"+w, func(t *testing.T) {
				var width int
				fmt.Sscan(w, &width)
				c := sessionTestChat(t)
				c.cfg.WorkspaceRoot = t.TempDir()
				c.engine.SetAutoCompaction(true)
				c.openSettingsMenu()
				for i, state := range states {
					if i > 0 {
						var key string
						var pair []string
						if json.Unmarshal(scenario.Steps[i-1], &key) == nil {
							pressMenuKey(t, c.KeyMap(), settingsKeyEvents[key])
						} else if json.Unmarshal(scenario.Steps[i-1], &pair) == nil {
							for _, r := range pair[1] {
								pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyRune, Rune: r})
							}
						}
					}
					if got := spanRowsText(c.piSettingsRows(width)); strings.Join(got, "\n") != strings.Join(state.Rows, "\n") {
						t.Fatalf("after %s:\n%s\nPi:\n%s", state.Step, strings.Join(got, "\n"), strings.Join(state.Rows, "\n"))
					}
				}
			})
		}
	}
}

// Changing a setting applies it at once and saves Pi's setting.
func TestSettingsMenuAppliesAndSaves(t *testing.T) {
	c := sessionTestChat(t)
	c.cfg.WorkspaceRoot = t.TempDir()
	c.engine.SetAutoCompaction(true)
	c.handleCommand("/settings")
	if c.modelMenuKind != "settings" {
		t.Fatalf("menu: %q", c.modelMenuKind)
	}
	press := func(names ...string) {
		for _, n := range names {
			pressMenuKey(t, c.KeyMap(), settingsKeyEvents[n])
		}
	}
	press("enter", "down", "enter", "down", "enter", "enter", "down", "enter", "down", "down", "enter")
	if c.engine.CompactionPolicy().Enabled || !c.cfg.HideThinkingBlock || c.cfg.QuietStartup != "header" || c.cfg.TreeFilterMode != "no-tools" || c.cfg.TUIWheelScrollLines != 1 {
		t.Fatalf("not applied: compact %v hide %v quiet %q tree %q wheel %d", c.engine.CompactionPolicy().Enabled, c.cfg.HideThinkingBlock, c.cfg.QuietStartup, c.cfg.TreeFilterMode, c.cfg.TUIWheelScrollLines)
	}
	raw, _ := os.ReadFile(filepath.Join(c.cfg.WorkspaceRoot, ".pi", "settings.json"))
	var saved map[string]any
	_ = json.Unmarshal(raw, &saved)
	if saved["compaction"].(map[string]any)["enabled"] != false || saved["hideThinkingBlock"] != true || saved["quietStartup"] != "header" || saved["treeFilterMode"] != "no-tools" || saved["fullscreenWheelScrollLines"] != float64(1) {
		t.Fatalf("saved: %s", raw)
	}
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEscape})
	if c.modelMenuOpen {
		t.Fatal("escape did not close")
	}
}
