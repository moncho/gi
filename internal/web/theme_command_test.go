package web

import (
	"strings"
	"testing"
)

func TestThemeCommandMatchesPiclaw(t *testing.T) {
	tint := func(r *themeCommandResult) string {
		if r.Payload == nil || r.Payload.Tint == nil {
			return ""
		}
		return *r.Payload.Tint
	}
	cases := []struct {
		prompt, status, message, theme, tint string
	}{
		{"/theme ristretto", "success", "Theme set to Ristretto.", "ristretto", ""},
		{"/THEME Drac", "success", "Theme set to Dracula.", "dracula", ""},
		{"/theme auto", "success", "Theme set to Default.", "default", ""},
		{"/theme dark", "error", "Unknown theme: dark. Omit the name to show available themes.", "", ""},
		{"/tint", "error", "Usage: /tint #hex (e.g. /tint #3b82f6), /tint orange, or /tint off", "", ""},
		{"/tint #E11D48", "success", "Tint set to #e11d48.", "default", "#e11d48"},
		{"/tint 3bf", "success", "Tint set to #33bbff.", "default", "#33bbff"},
		{"/tint Orange", "success", "Tint set to orange.", "default", "orange"},
		{"/tint off", "success", "Tint cleared (default light/dark restored).", "default", ""},
		{"/tint $$notacolor", "error", "Invalid tint value: $$notacolor. Use a hex color (e.g. #3b82f6), a named color (e.g. orange), or /tint off.", "", ""},
	}
	for _, c := range cases {
		r := themeCommand(c.prompt)
		if r == nil || r.Status != c.status || r.Message != c.message || tint(r) != c.tint {
			t.Fatalf("%s: got %+v", c.prompt, r)
		}
		if got := ""; r.Payload != nil {
			got = r.Payload.Theme
			if got != c.theme {
				t.Fatalf("%s: theme %q, want %q", c.prompt, got, c.theme)
			}
		} else if c.theme != "" {
			t.Fatalf("%s: missing payload", c.prompt)
		}
	}
	for _, prompt := range []string{"hello", "/themes", "/model", "  "} {
		if themeCommand(prompt) != nil {
			t.Fatalf("%q must not be a theme command", prompt)
		}
	}
}

func TestThemeListShowsEveryPresetWithSwatches(t *testing.T) {
	list := themeCommand("/theme").Message
	if !strings.HasPrefix(list, "Available themes:\n| Theme | Mode | Swatches |") || !strings.HasSuffix(list, "Usage: /theme <name>\n(omit name to show this list)") {
		t.Fatalf("list frame:\n%s", list)
	}
	for _, p := range themePresets {
		if !strings.Contains(list, "| "+p.Label+" ("+p.Name+") |") {
			t.Fatalf("missing %s", p.Name)
		}
	}
	if !strings.Contains(list, "| Default (default) | auto (light) | ![](data:image/svg+xml;base64,") || strings.Count(list, "![](") != 8*len(themePresets) {
		t.Fatalf("swatches:\n%s", list)
	}
}
