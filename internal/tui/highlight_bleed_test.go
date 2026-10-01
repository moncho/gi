package tui

import (
	"bytes"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

// Pi 1.0 (#10169): colour must not bleed past selection or search
// highlights when a styled token ends at the highlight boundary. gi renders
// per cell; check the row renderer's ANSI output resets attributes after a
// highlight that follows a styled token.
func TestNoColourBleedPastHighlights(t *testing.T) {
	var out bytes.Buffer
	term := gotui.NewANSITerminalWithCaps(&out, nil, gotui.Capabilities{Colors: gotui.ColorTrue, Unicode: true})
	buf := gotui.NewBuffer(20, 1)
	token := gotui.NewStyle().Foreground(piAccent).Bold()
	highlight := gotui.NewStyle().Background(piSearchMatchBg).Foreground(piText)
	buf.SetString(0, 0, "abc", token)            // styled token
	buf.SetString(3, 0, "def", highlight)        // highlight starting at its end
	buf.SetString(6, 0, "ghi", gotui.NewStyle()) // plain text after the highlight
	gotui.RenderRows(term, buf, true)
	raw := out.String()
	plain := strings.Index(raw, "ghi")
	hl := strings.Index(raw, "def")
	if plain < 0 || hl < 0 {
		t.Fatalf("missing text: %q", raw)
	}
	// Between the highlight and the plain text there must be a full reset,
	// so neither the token's bold/colour nor the highlight background leaks.
	between := raw[hl+3 : plain]
	resets := func(s string) bool {
		return strings.Contains(s, "\x1b[0m") || strings.Contains(s, "\x1b[m") || strings.Contains(s, "\x1b[0;")
	}
	if !resets(between) {
		t.Fatalf("no reset before plain text after highlight: %q", raw)
	}
	beforeHL := raw[strings.Index(raw, "abc")+3 : hl]
	if !resets(beforeHL) {
		t.Fatalf("token bold not cleared before highlight: %q", raw)
	}
}
