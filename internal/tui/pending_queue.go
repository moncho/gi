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
		return []string{selectorText("queue: pending display unavailable; /queue to inspect", width)}
	}
	if len(items) == 0 {
		return nil
	}
	visible := min(min(len(items), 6), max(0, (height-10)/3))
	if visible == 0 {
		return []string{selectorText(fmt.Sprintf("queue: %d pending; /queue to inspect", len(items)), width)}
	}
	lines := make([]string, 0, visible+1)
	for _, item := range items[:visible] {
		lines = append(lines, selectorText(item.Kind+": "+item.Text, width))
	}
	if remaining := len(items) - visible; remaining > 0 {
		lines = append(lines, selectorText(fmt.Sprintf("↳ %d more · /queue to inspect", remaining), width))
	} else {
		lines = append(lines, selectorText("↳ /queue to inspect · Alt+Up restores text-only queue when safe", width))
	}
	return lines
}
