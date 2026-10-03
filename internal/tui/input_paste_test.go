package tui

import (
	"fmt"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

// A multi-line paste lands in the editor as one edit; nothing is submitted
// (each pasted newline used to arrive as Enter).
func TestPasteInsertsMultipleLinesWithoutSubmitting(t *testing.T) {
	var submitted []string
	m := newMultilineInput(40, "", func(s string) { submitted = append(submitted, s) }, nil)
	m.focused = true
	c := &chatTUI{input: m}
	if !c.handlePaste(gotui.PasteEvent{Text: "first\r\nsecond\rthird\tx\x07"}) {
		t.Fatal("paste not handled")
	}
	if m.Text() != "first\nsecond\nthird    x" || len(submitted) != 0 {
		t.Fatalf("text %q submitted %v", m.Text(), submitted)
	}
	m.enter(gotui.KeyEvent{Key: gotui.KeyEnter})
	if len(submitted) != 1 || submitted[0] != "first\nsecond\nthird    x" {
		t.Fatalf("submitted %v", submitted)
	}
	// Not focused (a menu is open): the app replays it as keys instead.
	m.focused = false
	if c.handlePaste(gotui.PasteEvent{Text: "x"}) {
		t.Fatal("paste taken without focus")
	}
}

// Pi's large-paste markers: more than 10 lines or 1000 characters become a
// marker that moves and deletes as one unit and expands on submit.
func TestLargePasteMarkersLikePi(t *testing.T) {
	var submitted string
	m := newMultilineInput(40, "", func(s string) { submitted = s }, nil)
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	big := strings.Join(lines, "\n")
	m.paste("say ")
	m.paste(big)
	m.paste(" and ")
	m.paste(strings.Repeat("y", 1200))
	if m.Text() != "say [paste #1 +12 lines] and [paste #2 1200 chars]" {
		t.Fatalf("text %q", m.Text())
	}
	// The cursor jumps over a marker.
	m.moveLeft()
	if m.cursorPos != len([]rune("say [paste #1 +12 lines] and ")) {
		t.Fatalf("cursor %d", m.cursorPos)
	}
	// Deleting #1 renumbers #2 to #1.
	m.cursorPos = len([]rune("say [paste #1 +12 lines]"))
	m.backspace()
	if m.Text() != "say  and [paste #1 1200 chars]" || len(m.pastes) != 1 || m.pastes[1] != strings.Repeat("y", 1200) {
		t.Fatalf("after delete %q %v", m.Text(), len(m.pastes))
	}
	m.undo()
	if m.Text() != "say [paste #1 +12 lines] and [paste #2 1200 chars]" || len(m.pastes) != 2 {
		t.Fatalf("undo %q", m.Text())
	}
	m.enter(gotui.KeyEvent{Key: gotui.KeyEnter})
	if submitted != "say "+big+" and "+strings.Repeat("y", 1200) {
		t.Fatalf("submitted %q", submitted[:min(60, len(submitted))])
	}
	// Drafts store the expanded text, with the cursor mapped across.
	m.SetText("")
	m.paste(big)
	if m.ExpandedText() != big || m.expandedCursor() != len([]rune(big)) {
		t.Fatalf("expanded %d", m.expandedCursor())
	}
}

// A pasted path after a word character gets a separating space (Pi).
func TestPastedPathSpacing(t *testing.T) {
	m := newMultilineInput(40, "", nil, nil)
	m.SetText("look at")
	m.paste("/tmp/x")
	m.paste("~/y")
	if m.Text() != "look at /tmp/x ~/y" {
		t.Fatalf("%q", m.Text())
	}
}
