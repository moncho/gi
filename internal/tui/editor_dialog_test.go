package tui

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

// Pi's ExtensionEditorComponent rendered by Pi, key by key:
// bun scripts/golden-editor-dialog.mjs
func TestEditorDialogMatchesPi(t *testing.T) {
	raw, err := os.ReadFile("testdata/pi-editor-dialog.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]struct {
		Steps  []json.RawMessage `json:"steps"`
		Widths map[string][]struct {
			Step   string `json:"step"`
			Rows   []string
			Result *struct {
				Submit *string `json:"submit"`
				Cancel bool    `json:"cancel"`
			} `json:"result"`
		} `json:"widths"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	keys := map[string]gotui.KeyEvent{
		"enter": {Key: gotui.KeyEnter}, "esc": {Key: gotui.KeyEscape}, "backspace": {Key: gotui.KeyBackspace},
		"shift+enter": {Key: gotui.KeyEnter, Mod: gotui.ModShift},
	}
	for name, scenario := range golden {
		for w, states := range scenario.Widths {
			t.Run(name+"@"+w, func(t *testing.T) {
				width, _ := strconv.Atoi(w)
				d := &editorDialog{title: "Custom summarization instructions"}
				var submitted *string
				cancelled := false
				km := editorDialogKeys(d, func() {}, func(func()) {}, func(text string, submit bool) {
					if submit {
						submitted = &text
					} else {
						cancelled = true
					}
				})
				for i, state := range states {
					if i > 0 {
						var key string
						var pair []string
						if json.Unmarshal(scenario.Steps[i-1], &key) == nil {
							pressMenuKey(t, km, keys[key])
						} else if json.Unmarshal(scenario.Steps[i-1], &pair) == nil {
							for _, r := range pair[1] {
								pressMenuKey(t, km, gotui.KeyEvent{Key: gotui.KeyRune, Rune: r})
							}
						}
					}
					if got := spanRowsText(d.rows(width)); strings.Join(got, "\n") != strings.Join(state.Rows, "\n") {
						t.Fatalf("after %s:\n%s\nPi:\n%s", state.Step, strings.Join(got, "\n"), strings.Join(state.Rows, "\n"))
					}
					switch r := state.Result; {
					case r == nil && (submitted != nil || cancelled):
						t.Fatalf("after %s: finished early", state.Step)
					case r != nil && r.Cancel && !cancelled:
						t.Fatalf("after %s: not cancelled", state.Step)
					case r != nil && r.Submit != nil && (submitted == nil || *submitted != *r.Submit):
						t.Fatalf("after %s: submitted %v, Pi %q", state.Step, submitted, *r.Submit)
					}
				}
			})
		}
	}
}
