package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

var scopedModelsKeyEvents = map[string]gotui.KeyEvent{
	"up":        {Key: gotui.KeyUp},
	"down":      {Key: gotui.KeyDown},
	"enter":     {Key: gotui.KeyEnter},
	"esc":       {Key: gotui.KeyEscape},
	"backspace": {Key: gotui.KeyBackspace},
	"ctrl+c":    {Key: gotui.KeyRune, Rune: 'c', Mod: gotui.ModCtrl},
	"ctrl+a":    {Key: gotui.KeyRune, Rune: 'a', Mod: gotui.ModCtrl},
	"ctrl+x":    {Key: gotui.KeyRune, Rune: 'x', Mod: gotui.ModCtrl},
	"ctrl+p":    {Key: gotui.KeyRune, Rune: 'p', Mod: gotui.ModCtrl},
	"ctrl+s":    {Key: gotui.KeyRune, Rune: 's', Mod: gotui.ModCtrl},
	"alt+up":    {Key: gotui.KeyUp, Mod: gotui.ModAlt},
	"alt+down":  {Key: gotui.KeyDown, Mod: gotui.ModAlt},
}

// pressMenuKey runs the binding in keys for ev: exact modifiers, else the
// printable-rune binding.
func pressMenuKey(t *testing.T, keys gotui.KeyMap, ev gotui.KeyEvent) {
	t.Helper()
	for _, b := range keys {
		p := b.Pattern
		if p.AnyRune || p.Mod != ev.Mod {
			continue
		}
		if (p.Rune != 0 && ev.Key == gotui.KeyRune && p.Rune == ev.Rune) || (p.Rune == 0 && p.Key != gotui.KeyRune && p.Key == ev.Key) {
			b.Handler(ev)
			return
		}
	}
	if ev.Key == gotui.KeyRune && ev.Mod == 0 {
		for _, b := range keys {
			if b.Pattern.AnyRune {
				b.Handler(ev)
				return
			}
		}
	}
	t.Fatalf("no binding for %+v", ev)
}

// /scoped-models renders and behaves as Pi's ScopedModelsSelectorComponent
// key by key: rows, the run's model scope after each change, what Ctrl+S
// saves, and closing (bun scripts/golden-scoped-models.mjs).
func TestScopedModelsMatchPi(t *testing.T) {
	defer func(keys piKeys) { defaultPiKeys = keys }(defaultPiKeys)
	defaultPiKeys = piKeys{}
	raw, err := os.ReadFile("testdata/pi-scoped-models.json")
	if err != nil {
		t.Fatal(err)
	}
	type state struct {
		Step      string    `json:"step"`
		Rows      []string  `json:"rows"`
		Changed   *[]string `json:"changed"`
		Persisted *[]string `json:"persisted"`
		Cancelled bool      `json:"cancelled"`
	}
	var golden struct {
		Models []struct {
			Provider string `json:"provider"`
			ID       string `json:"id"`
			Name     string `json:"name"`
		} `json:"models"`
		Scenarios map[string]struct {
			Enabled *[]string          `json:"enabled"`
			Steps   []json.RawMessage  `json:"steps"`
			Widths  map[string][]state `json:"widths"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	var all []string
	names := map[string]string{}
	for _, m := range golden.Models {
		id := m.Provider + "/" + m.ID
		all, names[id] = append(all, id), m.Name
	}
	for name, scenario := range golden.Scenarios {
		for w, states := range scenario.Widths {
			t.Run(fmt.Sprintf("%s@%s", name, w), func(t *testing.T) {
				var width int
				fmt.Sscan(w, &width)
				c := sessionTestChat(t)
				c.cfg.WorkspaceRoot = t.TempDir()
				s := &scopedModelsState{all: all, names: names, allOn: scenario.Enabled == nil}
				if scenario.Enabled != nil {
					s.enabled = slices.Clone(*scenario.Enabled)
				}
				c.scopedModels, c.modelMenuOpen, c.modelMenuKind = s, true, "scoped-models"
				check := func(i int) {
					t.Helper()
					want := states[i]
					if want.Cancelled {
						if c.modelMenuOpen {
							t.Fatalf("after %s: still open, Pi closed", want.Step)
						}
						return
					}
					if got := spanRowsText(c.piScopedModelsRows(width)); strings.Join(got, "\n") != strings.Join(want.Rows, "\n") {
						t.Fatalf("after %s:\n%s\nPi:\n%s", want.Step, strings.Join(got, "\n"), strings.Join(want.Rows, "\n"))
					}
					if want.Changed != nil {
						// Pi's updateSessionModels over the selector's ids.
						ids := *want.Changed
						scoped := slices.ContainsFunc(ids, func(id string) bool { return names[id] != "" }) &&
							slices.ContainsFunc(all, func(id string) bool { return !slices.Contains(ids, id) })
						if c.cfg.EnabledModelsConfigured != scoped || scoped && !slices.Equal(c.cfg.EnabledModels, ids) {
							t.Fatalf("after %s: scope %v %v, Pi %v %v", want.Step, c.cfg.EnabledModelsConfigured, c.cfg.EnabledModels, scoped, ids)
						}
					}
					saved, _ := os.ReadFile(filepath.Join(c.cfg.WorkspaceRoot, ".pi", "settings.json"))
					if want.Persisted != nil {
						ids := *want.Persisted
						var settings struct {
							EnabledModels *[]string `json:"enabledModels"`
						}
						_ = json.Unmarshal(saved, &settings)
						every := len(ids) == len(all) && !slices.ContainsFunc(ids, func(id string) bool { return names[id] == "" })
						if every != (settings.EnabledModels == nil) || !every && !slices.Equal(*settings.EnabledModels, ids) {
							t.Fatalf("after %s: saved %s, Pi %v", want.Step, saved, ids)
						}
					}
					if !c.modelMenuOpen {
						t.Fatalf("after %s: closed, Pi still open", want.Step)
					}
				}
				check(0)
				for i, rawStep := range scenario.Steps {
					var key string
					var pair []string
					if json.Unmarshal(rawStep, &key) == nil {
						pressMenuKey(t, c.KeyMap(), scopedModelsKeyEvents[key])
					} else if json.Unmarshal(rawStep, &pair) == nil {
						for _, r := range pair[1] {
							pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyRune, Rune: r})
						}
					}
					check(i + 1)
				}
			})
		}
	}
}

// Bare /scoped-models opens the selector on the configured enabledModels;
// with none configured every model is enabled (Pi's null).
func TestScopedModelsCommandOpensSelector(t *testing.T) {
	c := sessionTestChat(t)
	c.handleCommand("/scoped-models")
	if !c.modelMenuOpen || c.modelMenuKind != "scoped-models" || c.scopedModels == nil || !c.scopedModels.allOn {
		t.Fatalf("selector not open with all enabled: %v %q %+v", c.modelMenuOpen, c.modelMenuKind, c.scopedModels)
	}
	c.closeModelMenu()
	c.cfg.EnabledModels, c.cfg.EnabledModelsConfigured = []string{"openai/gpt-5", "anthropic/claude"}, true
	c.handleCommand("/scoped-models")
	if s := c.scopedModels; s == nil || s.allOn || !slices.Equal(s.enabled, []string{"openai/gpt-5", "anthropic/claude"}) {
		t.Fatalf("selector state: %+v", s)
	}
	if !strings.Contains(strings.Join(spanRowsText(c.piScopedModelsRows(80)), "\n"), "Model Configuration") {
		t.Fatal("selector rows")
	}
}
