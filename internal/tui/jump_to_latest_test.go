package tui

import (
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

func scrolledTranscript(offset int) (*chatTUI, *gotui.Buffer) {
	transcript := gotui.New(
		gotui.WithWidth(40),
		gotui.WithHeight(4),
		gotui.WithScrollable(gotui.ScrollVertical),
		gotui.WithScrollbarHidden(true), // as gi's transcript (Pi has no transcript scrollbar)
		gotui.WithScrollOffset(0, offset),
	)
	lines := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
	for _, line := range lines {
		transcript.AddChild(gotui.New(gotui.WithWidth(40), gotui.WithHeight(1), gotui.WithText(line)))
	}
	buf := gotui.NewBuffer(40, 6)
	transcript.RenderTo(buf, 40, 6)
	c := &chatTUI{transcript: lines, transcriptScroll: offset, transcriptRef: gotui.NewRef(), transcriptRegion: transcript}
	c.transcriptRef.Set(transcript)
	return c, buf
}

func bufferRow(buf *gotui.Buffer, y int) string {
	var b strings.Builder
	for x := 0; x < buf.Width(); x++ {
		cell := buf.Cell(x, y)
		if cell.Width == 0 {
			continue
		}
		if cell.Rune == 0 {
			b.WriteRune(' ')
		} else {
			b.WriteRune(cell.Rune)
			b.WriteString(cell.Combining)
		}
	}
	return b.String()
}

// Scrolled away from the end, Pi's indicator is centred on the transcript's
// last row; clicking it scrolls to the bottom and resumes following.
func TestJumpToLatestIndicator(t *testing.T) {
	c, buf := scrolledTranscript(0)
	c.compositeJumpToLatest(buf)
	width := gotui.StringWidth(jumpToLatestLabel)
	column := (40 - width) / 2
	if got := bufferRow(buf, 3); got != "4"+strings.Repeat(" ", column-1)+jumpToLatestLabel+strings.Repeat(" ", 40-column-width) {
		t.Fatalf("last transcript row %q", got)
	}
	if cell := buf.Cell(column+1, 3); cell.Style.Bg != piSelectedBg || cell.Style.Fg != piText {
		t.Fatalf("style %+v", cell.Style)
	}
	if c.handleJumpToLatestClick(gotui.MouseEvent{Button: gotui.MouseLeft, Action: gotui.MousePress, X: column - 1, Y: 3}) {
		t.Fatal("click beside the indicator was taken")
	}
	if !c.HandleMouse(gotui.MouseEvent{Button: gotui.MouseLeft, Action: gotui.MousePress, X: column + 2, Y: 3}) || !c.stickToBottom {
		t.Fatal("click on the indicator did not jump to the latest message")
	}

	// Following the end, or in regular mode, there is no indicator.
	c, buf = scrolledTranscript(4)
	c.compositeJumpToLatest(buf)
	if strings.Contains(bufferRow(buf, 3), "Jump") || c.jumpToLatest.width != 0 {
		t.Fatal("indicator drawn at the bottom")
	}
	c, buf = scrolledTranscript(0)
	c.regularMode = true
	c.compositeJumpToLatest(buf)
	if strings.Contains(bufferRow(buf, 3), "Jump") {
		t.Fatal("indicator drawn in regular mode")
	}
}

func TestJumpToLatestTruncates(t *testing.T) {
	if got := truncateToDisplayWidth(jumpToLatestLabel, 10); got != " ↓ Jump to" {
		t.Fatalf("%q", got)
	}
}
