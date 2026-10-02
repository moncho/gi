package tui

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	gotui "github.com/grindlemire/go-tui"
)

// Inspect actual ANSI bytes, not just the renderer's own width calculations.
func TestPiRowRedrawANSI(t *testing.T) {
	var out bytes.Buffer
	term := gotui.NewANSITerminalWithCaps(&out, nil, gotui.Capabilities{Colors: gotui.ColorTrue, Unicode: true, Hyperlinks: true})
	buf := gotui.NewBuffer(40, 3)
	draw := func(text string) string {
		out.Reset()
		buf.Clear()
		buf.SetString(0, 0, text, gotui.NewStyle().Bold())
		buf.SetString(0, 1, "unchanged footer", gotui.NewStyle())
		gotui.RenderRows(term, buf, false)
		return out.String()
	}
	draw("│ 🇵🇹 ⚠️ 👩🏽‍💻 日本語 é │")
	raw := draw("│ 🇯🇵 ✅ 🧑🏻‍🚀 中文 é │")
	if !utf8.ValidString(raw) {
		t.Fatalf("invalid UTF-8: %q", raw)
	}
	if !strings.HasPrefix(raw, "\x1b[1;1H") || !strings.Contains(raw, "\x1b[K") {
		t.Fatalf("missing row clear: %q", raw)
	}
	if strings.Count(raw, "H") != 2 { // clear then contiguous repaint from column one
		t.Fatalf("unexpected cell cursor jumps: %q", raw)
	}
	for _, text := range []string{"🇯🇵", "✅", "🧑🏻‍🚀", "中文", "é"} {
		if !strings.Contains(raw, text) {
			t.Fatalf("split/missing cluster %q: %q", text, raw)
		}
	}
	if strings.Contains(raw, "unchanged footer") {
		t.Fatalf("unchanged row repainted: %q", raw)
	}
	if got := draw("│ 🇯🇵 ✅ 🧑🏻‍🚀 中文 é │"); got != "" {
		t.Fatalf("unchanged frame wrote %q", got)
	}
	raw = draw("short")
	if strings.Contains(raw, "🇯🇵") || !strings.Contains(raw, "\x1b[K") {
		t.Fatalf("shrink did not clear old row: %q", raw)
	}
	raw = draw("")
	if strings.Contains(raw, "short") || !strings.Contains(raw, "\x1b[K") {
		t.Fatalf("blank row did not clear: %q", raw)
	}
	out.Reset()
	gotui.RenderRows(term, buf, true)
	if got := strings.Count(out.String(), "\x1b[K"); got != 3 {
		t.Fatalf("forced redraw cleared %d rows", got)
	}
	if strings.Contains(out.String(), "\x1b[3J") {
		t.Fatal("erased terminal scrollback")
	}
}

func TestPiRowDiffPreservesWideStylesAndLinks(t *testing.T) {
	buf := gotui.NewBuffer(12, 2)
	style := gotui.NewStyle().Background(gotui.RGBColor(30, 40, 50))
	buf.SetString(0, 0, "界", style)
	buf.SetCell(2, 0, gotui.Cell{Rune: 'x', Width: 1, Style: style, Link: "https://example.com"})
	buf.SetRune(5, 0, ' ', style) // styled trailing spaces must survive
	changes := buf.RowDiff(false)
	if len(changes) != 7 || !changes[0].EraseToEOL || changes[0].X != 0 {
		t.Fatalf("row diff: %+v", changes)
	}
	if changes[1].Cell.Width != 2 || !changes[2].Cell.IsContinuation() || changes[3].Cell.Link == "" || !changes[6].Cell.Style.Equal(style) {
		t.Fatalf("lost presentation: %+v", changes)
	}
	var out bytes.Buffer
	term := gotui.NewANSITerminalWithCaps(&out, nil, gotui.Capabilities{Colors: gotui.ColorTrue, Hyperlinks: true})
	gotui.RenderRows(term, buf, false)
	if !strings.Contains(out.String(), "\x1b]8;;https://example.com\x1b\\") {
		t.Fatalf("lost OSC8 link: %q", out.String())
	}
	buf.Resize(5, 1)
	buf.Clear()
	buf.SetString(0, 0, "new", gotui.NewStyle())
	out.Reset()
	gotui.RenderRows(term, buf, true)
	if !strings.Contains(out.String(), "new") || strings.Count(out.String(), "\x1b[K") != 1 {
		t.Fatalf("resize: %q", out.String())
	}
}

func TestPiRowPagingOverlap(t *testing.T) {
	for _, height := range []int{2, 4, 12, 24} {
		c := &chatTUI{outputHeight: height, transcript: make([]string, 200), transcriptScroll: 50}
		viewport := c.transcriptViewportHeight()
		c.pageTranscript(1)
		want := 50 + max(1, viewport-4)
		if c.transcriptScroll != want {
			t.Fatalf("viewport %d: got %d want %d", viewport, c.transcriptScroll, want)
		}
		c.pageTranscript(-1)
		if c.transcriptScroll != 50 {
			t.Fatalf("reverse page: %d", c.transcriptScroll)
		}
	}
}
