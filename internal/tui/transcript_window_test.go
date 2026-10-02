package tui

import (
	"fmt"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
)

func windowTestTranscript() []string {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("you: question %d %s", i, strings.Repeat("words ", i%7)))
		lines = append(lines, fmt.Sprintf("Gi: answer %d with **markdown** and a long line %s", i, strings.Repeat("lorem ipsum ", 3+i%11)))
	}
	return lines
}

func renderTranscriptRows(t *testing.T, c *chatTUI, windowed bool, width, height, offset int) []string {
	t.Helper()
	transcript := gotui.New(gotui.WithWidth(width), gotui.WithHeight(height), gotui.WithScrollable(gotui.ScrollVertical),
		gotui.WithScrollbarHidden(true), gotui.WithScrollOffset(0, offset), gotui.WithDirection(gotui.Column))
	blocks := c.buildTranscriptRenderableBlocks(c.visibleTranscript())
	if windowed {
		blocks = c.transcriptBlocks()
		c.addTranscriptWindow(transcript, blocks, width, height)
	} else {
		previous := ""
		for _, b := range blocks {
			transcript.AddChild(c.renderTranscriptBlockAfter(b, previous))
			if b.Kind != "thinking_indicator" {
				previous = b.Kind
			}
		}
	}
	if c.stickToBottom {
		transcript.ScrollToBottom()
	}
	buf := gotui.NewBuffer(width, height)
	transcript.RenderTo(buf, width, height)
	rows := make([]string, height)
	for y := 0; y < height; y++ {
		rows[y] = bufferRow(buf, y)
	}
	return rows
}

// Windowed rendering shows exactly what laying out every block shows, at
// any scroll offset and when following the bottom.
func TestTranscriptWindowMatchesFullLayout(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, transcript: windowTestTranscript(), transcriptExpanded: map[string]bool{}}
	const width, height = 60, 12
	for _, offset := range []int{0, 5, 37, 120, 250} {
		c.transcriptScroll, c.stickToBottom = offset, false
		full := renderTranscriptRows(t, c, false, width, height, offset)
		win := renderTranscriptRows(t, c, true, width, height, offset)
		if strings.Join(full, "\n") != strings.Join(win, "\n") {
			t.Fatalf("offset %d differs:\nfull:\n%s\nwindowed:\n%s", offset, strings.Join(full, "\n"), strings.Join(win, "\n"))
		}
	}
	c.stickToBottom = true
	full := renderTranscriptRows(t, c, false, width, height, 0)
	win := renderTranscriptRows(t, c, true, width, height, 0)
	if strings.Join(full, "\n") != strings.Join(win, "\n") {
		t.Fatalf("bottom differs:\nfull:\n%s\nwindowed:\n%s", strings.Join(full, "\n"), strings.Join(win, "\n"))
	}
	if c.blockCache.heightCount() == 0 {
		t.Fatal("heights not cached")
	}
}

// BenchmarkTranscriptFrame compares one frame's transcript layout for a long
// session: every block laid out versus the viewport window.
func BenchmarkTranscriptFrame(b *testing.B) {
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, windowTestTranscript()...)
	}
	for _, windowed := range []bool{false, true} {
		b.Run(fmt.Sprintf("windowed=%v", windowed), func(b *testing.B) {
			c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, transcript: lines, transcriptExpanded: map[string]bool{}, stickToBottom: true}
			buf := gotui.NewBuffer(100, 40) // reused, as by the app
			frame := func() {
				transcript := gotui.New(gotui.WithWidth(100), gotui.WithHeight(40), gotui.WithScrollable(gotui.ScrollVertical),
					gotui.WithScrollbarHidden(true), gotui.WithDirection(gotui.Column))
				if windowed {
					c.addTranscriptWindow(transcript, c.transcriptBlocks(), 100, 40)
				} else {
					previous := ""
					for _, block := range c.buildTranscriptRenderableBlocks(c.visibleTranscript()) {
						transcript.AddChild(c.renderTranscriptBlockAfter(block, previous))
						previous = block.Kind
					}
				}
				transcript.ScrollToBottom()
				buf.Clear()
				transcript.RenderTo(buf, 100, 40)
			}
			frame() // warm caches
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame()
			}
		})
	}
}

// The block list is rebuilt when a line changes (lines are replaced, never
// edited in place) and reused otherwise.
func TestTranscriptBlocksMemo(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, transcript: windowTestTranscript(), transcriptExpanded: map[string]bool{}}
	first := c.transcriptBlocks()
	if again := c.transcriptBlocks(); &again[0] != &first[0] {
		t.Fatal("unchanged transcript rebuilt")
	}
	c.transcript = append([]string(nil), c.transcript...)
	c.transcript[3] = "Gi: replaced answer"
	changed := c.transcriptBlocks()
	if &changed[0] == &first[0] || !strings.Contains(strings.Join(changed[3].Body, "")+changed[3].Header+changed[3].MarkdownSource, "replaced answer") {
		t.Fatalf("changed line not rebuilt: %+v", changed[3])
	}
	c.selectedTranscriptBlock = "x"
	if sel := c.transcriptBlocks(); &sel[0] == &changed[0] {
		t.Fatal("selection change not rebuilt")
	}
}

// A long session keeps every block's height but rendered elements only up
// to the LRU bound, and still renders the window exactly.
func TestTranscriptBlockCacheIsBounded(t *testing.T) {
	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines, windowTestTranscript()...)
	}
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, transcript: lines, transcriptExpanded: map[string]bool{}}
	const width, height = 60, 12
	blocks := len(c.transcriptBlocks())
	if blocks <= transcriptElementCacheMax {
		t.Fatalf("fixture too small: %d blocks", blocks)
	}
	for _, offset := range []int{0, 400, 2000} {
		c.transcriptScroll, c.stickToBottom = offset, false
		full := renderTranscriptRows(t, c, false, width, height, offset)
		win := renderTranscriptRows(t, c, true, width, height, offset)
		if strings.Join(full, "\n") != strings.Join(win, "\n") {
			t.Fatalf("offset %d differs", offset)
		}
	}
	if n := c.blockCache.elementCount(); n > transcriptElementCacheMax {
		t.Fatalf("%d rendered blocks kept, bound %d", n, transcriptElementCacheMax)
	}
	if n := c.blockCache.heightCount(); n < blocks-5 {
		t.Fatalf("only %d of %d heights kept", n, blocks)
	}
}
