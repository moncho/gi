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
	if len(c.blockHeights) == 0 {
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
			t := &testing.T{}
			renderTranscriptRows(t, c, windowed, 100, 40, 0) // warm caches
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				renderTranscriptRows(t, c, windowed, 100, 40, 0)
			}
		})
	}
}
