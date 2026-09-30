package tui

import (
	"context"
	"strings"
	"testing"
)

// Pi's /fork: pick an earlier user message; the new session holds only the
// history before it and the editor gets the message text back.
func TestForkFromEarlierUserMessage(t *testing.T) {
	c := sessionTestChat(t)
	ctx := context.Background()
	for _, m := range []struct{ id, role, text string }{
		{"m1", "user", "first question"}, {"m2", "assistant", "first answer"},
		{"m3", "user", "second question"}, {"m4", "assistant", "second answer"},
	} {
		if err := c.store.AddMessage(ctx, m.id, "A", m.role, m.text, map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	c.handleCommand("/fork")
	if !c.modelMenuOpen || c.modelMenuKind != "fork" || len(c.modelMenuChoices) != 2 || c.modelMenuSelected != 1 {
		t.Fatalf("selector: open=%v kind=%q choices=%v selected=%d", c.modelMenuOpen, c.modelMenuKind, c.modelMenuChoices, c.modelMenuSelected)
	}
	rows := c.piForkSelectorRows(80)
	var screen []string
	for _, row := range rows {
		var line strings.Builder
		for _, span := range row {
			line.WriteString(span.Text)
		}
		screen = append(screen, line.String())
	}
	joined := strings.Join(screen, "\n")
	for _, want := range []string{"Fork from Message", "› second question", "Message 2 of 2", "  first question"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("selector missing %q:\n%s", want, joined)
		}
	}
	c.acceptForkSelection()
	if c.modelMenuOpen || c.sessionID == "A" {
		t.Fatalf("fork did not switch: open=%v session=%s", c.modelMenuOpen, c.sessionID)
	}
	if c.input.Text() != "second question" {
		t.Fatalf("editor = %q, want the selected message", c.input.Text())
	}
	msgs, err := c.store.ListMessages(ctx, c.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range msgs {
		got = append(got, m.Content)
	}
	if strings.Join(got, "|") != "first question|first answer" {
		t.Fatalf("forked history = %v, want only messages before the selection", got)
	}
}

func TestForkWithoutUserMessages(t *testing.T) {
	c := sessionTestChat(t)
	c.handleCommand("/fork")
	if c.modelMenuOpen || !strings.Contains(strings.Join(c.transcript, "\n"), "No messages to fork from") {
		t.Fatalf("empty fork: open=%v transcript=%v", c.modelMenuOpen, c.transcript)
	}
}
