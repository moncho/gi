package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/gi/internal/sessionexport"
)

// The theme an HTML export takes is Pi's getResolvedThemeColors and
// getThemeExportColors for the active theme: built-in, or a custom theme
// with every colour form, "" tokens, fallbacks and export colours
// (scripts/golden-export-html.mjs).
func TestExportThemeMatchesPi(t *testing.T) {
	data, err := os.ReadFile("../sessionexport/testdata/pi-export-html.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Themes map[string]struct {
			Colors   [][2]string         `json:"colors"`
			Export   map[string]string   `json:"export"`
			Terminal map[string][3]uint8 `json:"terminal"`
		} `json:"themes"`
		ThemeFiles map[string]json.RawMessage `json:"themeFiles"`
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, doc := range golden.ThemeFiles {
		if err := os.WriteFile(filepath.Join(dir, name+".json"), doc, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	previousDirs, previousTheme, previousTerminal := customThemeDirs, piActiveTheme, piTerminal
	customThemeDirs = func() []string { return []string{dir} }
	piTerminal.colors, piTerminal.scheme = terminalColors{}, ""
	defer func() { customThemeDirs, piActiveTheme, piTerminal = previousDirs, previousTheme, previousTerminal }()
	for name, want := range golden.Themes {
		piActiveTheme = name
		piTerminal.colors = terminalColors{}
		if c, ok := want.Terminal["background"]; ok {
			fg := want.Terminal["foreground"]
			piTerminal.colors = terminalColors{background: &rgb{c[0], c[1], c[2]}, foreground: &rgb{fg[0], fg[1], fg[2]}}
		}
		got := exportTheme()
		w := sessionexport.Theme{Colors: want.Colors, PageBg: want.Export["pageBg"], CardBg: want.Export["cardBg"], InfoBg: want.Export["infoBg"]}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s:\n got %v\nwant %v", name, got, w)
		}
	}
}
