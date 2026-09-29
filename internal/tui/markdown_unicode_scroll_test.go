package tui

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
)

func TestTableUsesRendererClusterBoundaries(t *testing.T) {
	// UAX #29 splits the joiner after ASCII here; go-tui and its terminal
	// output treat this as one cluster. The table must use the latter geometry.
	const joined = "a\u200d👩"
	if gotui.StringWidth(joined) != 2 {
		t.Fatalf("renderer width changed: %d", gotui.StringWidth(joined))
	}
	if got := wrapTableCell(joined, 2); len(got) != 1 || got[0] != joined {
		t.Fatalf("cluster split at renderer width: %q", got)
	}
	lines := renderMarkdownTranscript("", "| Value | Next |\n|---|---|\n| "+joined+" | 🇵🇹 |\n", 32)
	for _, line := range lines {
		if got, want := gotui.StringWidth(stripMarkdownInlineStyleMarkers(line)), gotui.StringWidth(lines[0]); got != want {
			t.Fatalf("grid width %d != %d: %q", got, want, line)
		}
	}
	if !strings.Contains(strings.Join(lines, ""), joined) {
		t.Fatal("lost joined cluster")
	}
}

func TestPreformattedWrapPreservesClustersAndCellBounds(t *testing.T) {
	source := "👩🏽‍💻🇵🇹 é 日本語 ✈️"
	lines := wrapPreformattedWithPrefix(source, 9, "    ")
	var joined strings.Builder
	for _, line := range lines {
		if got := gotui.StringWidth(line); got > 9 {
			t.Fatalf("overflow: %q (%d)", line, got)
		}
		joined.WriteString(strings.TrimPrefix(line, "    "))
	}
	if joined.String() != source {
		t.Fatalf("split/lost cluster: %q != %q", joined.String(), source)
	}
}

func TestTableScrollFramesEmitValidUTF8AndAlignedCells(t *testing.T) {
	const markdown = "| Value | Result |\n|---|---|\n| a\u200d👩 👩🏽‍💻 | 🇵🇹 ⚠️ |\n| العربية 日本語 | ✈️ é |\n"
	for _, width := range []int{20, 38, 80} {
		c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, outputWidth: width}
		c.transcript = renderChatMarkdown("assistant", "Gi: ", strings.Repeat("before\n\n", 10)+markdown+strings.Repeat("\nafter\n", 20), width)
		root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(width), gotui.WithHeight(8), gotui.WithScrollable(gotui.ScrollVertical), gotui.WithScrollbarHidden(true))
		for _, block := range c.buildTranscriptRenderableBlocks(c.transcript) {
			root.AddChild(c.renderTranscriptBlock(block))
		}
		root.Calculate(width, 8)
		buffer := gotui.NewBuffer(width, 8)
		gotui.RenderTree(buffer, root)
		var output bytes.Buffer
		term := gotui.NewANSITerminalWithCaps(&output, nil, gotui.Capabilities{Colors: gotui.ColorTrue, Unicode: true})
		_, maxY := root.MaxScroll()
		if maxY < 8 {
			t.Fatalf("width %d: no scrolling: maxY=%d blocks=%d transcript=%d", width, maxY, len(c.buildTranscriptRenderableBlocks(c.transcript)), len(c.transcript))
		}
		endcapX, seen := -1, 0
		for step := 0; step <= 2*maxY; step++ {
			offset := step
			if offset > maxY {
				offset = 2*maxY - step
			}
			root.ScrollTo(0, offset)
			buffer.Clear()
			root.Calculate(width, 8)
			gotui.RenderTree(buffer, root)
			output.Reset()
			term.Flush(buffer.Diff())
			if !utf8.Valid(output.Bytes()) {
				t.Fatalf("width %d step %d: invalid UTF-8 in terminal update: %q", width, step, output.Bytes())
			}
			for y := 0; y < 8; y++ {
				for x := 0; x < width; x++ {
					r := buffer.Cell(x, y).Rune
					if r != '┐' && r != '┤' && r != '┘' {
						continue
					}
					if endcapX == -1 {
						endcapX = x
					}
					if x != endcapX {
						t.Fatalf("width %d step %d: table endcap at %d, expected %d", width, step, x, endcapX)
					}
					seen++
				}
			}

			buffer.Swap()
		}
		if seen == 0 {
			t.Fatalf("width %d: no table borders entered the viewport", width)
		}
	}
}
