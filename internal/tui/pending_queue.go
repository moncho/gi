package tui

import (
	"context"
	"fmt"
)

// The pending panel is a store-backed view, not an editable copy of delivery.
// Keep the dock bounded at small terminal sizes, and never promise Pi's
// edit-all behavior: Gi's text-only restoration can reject active Steer/media.
func (c *chatTUI) pendingQueueLines(width, height int) []string {
	if c.store == nil || c.sessionID == "" {
		return nil
	}
	items, err := c.store.ListPendingTUIMessages(context.Background(), c.sessionID)
	if err != nil {
		return []string{pendingRowText("queue: pending display unavailable; /queue to inspect", width)}
	}
	if len(items) == 0 {
		return nil
	}
	visible := min(min(len(items), 6), max(0, (height-10)/3))
	if visible == 0 {
		return []string{pendingRowText(fmt.Sprintf("queue: %d pending; /queue to inspect", len(items)), width)}
	}
	lines := make([]string, 0, visible+1)
	for _, item := range items[:visible] {
		lines = append(lines, pendingRowText(item.Kind+": "+item.Text, width))
	}
	if remaining := len(items) - visible; remaining > 0 {
		lines = append(lines, pendingRowText(fmt.Sprintf("↳ %d more · /queue to inspect", remaining), width))
	} else {
		// Pi's hint; Alt+Up restores text-only queued messages into the editor.
		lines = append(lines, pendingRowText("↳ alt+up to edit all queued messages", width))
	}
	return lines
}

// pendingDockLines lays out queued messages as Pi's pending container: a
// blank spacer row, then the dim rows padded one column from the edge.
func (c *chatTUI) pendingDockLines(width, height int) []string {
	lines := c.pendingQueueLines(max(1, width-1), height)
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, "")
	for _, line := range lines {
		out = append(out, " "+line)
	}
	return out
}
