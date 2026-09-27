package tui

import (
	gotui "github.com/grindlemire/go-tui"
	"testing"
)

func TestPiEnterAndAltEnterChooseDifferentAdmissionCallbacks(t *testing.T) {
	var sent, queued []string
	m := newMultilineInput(80, "", func(s string) { sent = append(sent, s) }, nil)
	m.onFollowUp = func(s string) { queued = append(queued, s) }
	for _, tc := range []struct {
		mod  gotui.Modifier
		text string
	}{{0, "steering direction"}, {gotui.ModAlt, "later task"}} {
		m.SetText(tc.text)
		m.enter(gotui.KeyEvent{Key: gotui.KeyEnter, Mod: tc.mod})
	}
	if len(sent) != 1 || sent[0] != "steering direction" || len(queued) != 1 || queued[0] != "later task" {
		t.Fatalf("send=%v followUp=%v", sent, queued)
	}
	m.SetText("draft")
	m.enter(gotui.KeyEvent{Key: gotui.KeyEnter, Mod: gotui.ModShift})
	if m.Text() != "draft\n" || len(sent) != 1 || len(queued) != 1 {
		t.Fatalf("newline dispatched: %q %v %v", m.Text(), sent, queued)
	}
}

func TestPiFollowUpAdmissionRetainsDraftWhenNoModelSelected(t *testing.T) {
	c := sessionTestChat(t)
	c.cfg.DefaultModel = ""
	c.input.SetText("unsent follow-up")
	before := c.input.Text()
	c.onFollowUp(before)
	if c.input.Text() != before {
		t.Fatal("follow-up rejection cleared draft")
	}
	if len(c.history) != 0 {
		t.Fatal("rejected follow-up added to history")
	}
}
