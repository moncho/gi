package tui

import (
	"strings"
	"testing"
)

func TestBuiltinHighlighter(t *testing.T) {
	h := NewHighlighter(DefaultPalette())

	t.Run("colorizes keywords without rewriting text", func(t *testing.T) {
		code := "var x = 1"
		lines := h.Highlight("go", code)
		if len(lines) != 1 {
			t.Fatalf("want 1 line, got %d", len(lines))
		}
		// Contract: concatenated span text equals the input line.
		var b strings.Builder
		for _, s := range lines[0] {
			b.WriteString(s.Text)
		}
		if b.String() != code {
			t.Errorf("reconstructed %q, want %q", b.String(), code)
		}
		// The "var" keyword span carries the keyword color.
		kw := DefaultPalette()[TokenKeyword]
		found := false
		for _, s := range lines[0] {
			if s.Text == "var" {
				found = true
				if !s.Style.Fg.Equal(kw) {
					t.Errorf("var foreground = %v, want %v", s.Style.Fg, kw)
				}
			}
		}
		if !found {
			t.Error(`no "var" span found`)
		}
	})

	t.Run("unknown language yields uncolored spans", func(t *testing.T) {
		lines := h.Highlight("rust", "let x = 1")
		for _, s := range lines[0] {
			if !s.Style.Fg.Equal(DefaultColor()) {
				t.Errorf("span %q has color %v, want default", s.Text, s.Style.Fg)
			}
		}
	})
}
