package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestPendingQueuePanelUsesDurableSteerThenFollowUp(t *testing.T) {
	c := sessionTestChat(t)
	ctx := context.Background()
	c.input.SetText("newer draft🙂")
	if _, err := c.store.CreateTurnWithStatus(ctx, "display-first", c.sessionID, "queued", "follow\nnewline", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.store.EnqueueSteering(ctx, c.sessionID, "", "user", "steer now🙂", nil, nil, "one-at-a-time"); err != nil {
		t.Fatal(err)
	}
	want := []string{"Steering: steer now🙂", "Follow-up: follow", "↳ /queue to inspect · Alt+Up restores text-only queue when safe"}
	got := c.pendingQueueLines(80, 32)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("panel=%q want=%q", got, want)
	}
	if c.input.Text() != "newer draft🙂" {
		t.Fatalf("read-only panel changed draft: %q", c.input.Text())
	}
	other := &chatTUI{store: c.store, sessionID: c.sessionID}
	if strings.Join(other.pendingQueueLines(80, 32), "\n") != strings.Join(want, "\n") {
		t.Fatal("second TUI should read the same durable queue")
	}
	other.sessionID = "B"
	if got := other.pendingQueueLines(80, 32); len(got) != 0 {
		t.Fatalf("other session leaked queued work: %q", got)
	}
	if err := c.store.CancelQueuedTurn(ctx, c.sessionID, "display-first"); err != nil {
		t.Fatal(err)
	}
	if got := c.pendingQueueLines(80, 32); len(got) != 2 || got[0] != want[0] {
		t.Fatalf("panel did not refresh after removal: %q", got)
	}
	if got := c.pendingQueueLines(40, 12); len(got) != 1 || got[0] != "queue: 1 pending; /queue to inspect" {
		t.Fatalf("small-terminal summary missing: %q", got)
	}
	if got := c.pendingQueueLines(12, 32); len(got) != 2 || strings.Contains(strings.Join(got, ""), "\n") || got[0] != "Steering:..." {
		t.Fatalf("narrow panel should truncate without wrapping: %q", got)
	}
}

func TestPendingRowTextMatchesPiFirstLineAndGraphemeCut(t *testing.T) {
	cases := []struct {
		text  string
		width int
		want  string
	}{
		{"Follow-up: first\nsecond", 30, "Follow-up: first"},
		{"Steering: abcdefghijklmnop", 12, "Steering:..."},
		{"Follow-up: 🧑‍💻abc", 13, "Follow-up:..."},
		{"Steering: 🧑‍💻abc", 13, "Steering: ..."},
		{"Steering: e\u0301ax", 12, "Steering:..."},
		{"Steering: x\x1b[31mred", 35, "Steering: x [31mred"},
		{"Steering: abcd", 2, ".."},
	}
	for _, tc := range cases {
		if got := pendingRowText(tc.text, tc.width); got != tc.want {
			t.Errorf("pendingRowText(%q,%d)=%q want %q", tc.text, tc.width, got, tc.want)
		}
	}
}

func TestPendingQueuePanelBoundedAndClears(t *testing.T) {
	c := sessionTestChat(t)
	ctx := context.Background()
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("display-%02d", i)
		if _, err := c.store.CreateTurnWithStatus(ctx, id, c.sessionID, "queued", "item", nil); err != nil {
			t.Fatal(err)
		}
	}
	lines := c.pendingQueueLines(24, 32)
	if len(lines) != 7 || !strings.Contains(lines[6], "2 more") {
		t.Fatalf("bounded panel=%q", lines)
	}
	rows, err := c.store.ListQueuedTurns(ctx, c.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if err := c.store.CancelQueuedTurn(ctx, c.sessionID, row.ID); err != nil {
			t.Fatal(err)
		}
	}
	if got := c.pendingQueueLines(80, 32); len(got) != 0 {
		t.Fatalf("empty panel=%q", got)
	}
}
