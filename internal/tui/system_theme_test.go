package tui

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

func mustHex(t *testing.T, s string) rgb {
	t.Helper()
	c, ok := parseOSCColor(s)
	if !ok {
		t.Fatalf("bad colour %q", s)
	}
	return c
}

// Golden colours from Pi's generateSystemThemeColors
// (scripts/golden-system-theme.mjs).
func TestSystemThemeMatchesPi(t *testing.T) {
	raw, err := os.ReadFile("testdata/pi-system-theme.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Background, Foreground, Appearance string
		Palette                                  []string
		Colors                                   map[string]string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		in := systemThemeInput{saturation: 1}
		bg := mustHex(t, c.Background)
		in.background = &bg
		if c.Foreground != "" {
			fg := mustHex(t, c.Foreground)
			in.foreground = &fg
		}
		for _, p := range c.Palette {
			in.palette = append(in.palette, mustHex(t, p))
		}
		colors, appearance, ok := generateSystemThemeColors(in)
		if !ok || appearance != c.Appearance {
			t.Fatalf("%s: appearance %q (ok=%v), want %q", c.Name, appearance, ok, c.Appearance)
		}
		var keys []string
		for k := range c.Colors {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if colors[k] != c.Colors[k] {
				t.Errorf("%s.%s = %q, want %q", c.Name, k, colors[k], c.Colors[k])
			}
		}
		if len(colors) != len(c.Colors) {
			t.Errorf("%s: %d tokens, want %d", c.Name, len(colors), len(c.Colors))
		}
	}
}
