package tui

import gotui "github.com/grindlemire/go-tui"

// The scrollbar is painted by go-tui, but the library does not handle mouse
// presses on its gutter. Match its thumb geometry (element_render.go) here.
func (c *chatTUI) handleTranscriptScrollbar(me gotui.MouseEvent) bool {
	if c.regularMode || c.modelMenuOpen || c.transcriptRegion == nil {
		c.scrollbarDragging = false
		return false
	}
	if c.scrollbarDragging {
		if me.Button != gotui.MouseLeft {
			return false
		}
		if me.Action == gotui.MouseDrag || me.Action == gotui.MouseRelease {
			c.moveTranscriptScrollbar(me.Y)
			if me.Action == gotui.MouseRelease {
				c.scrollbarDragging = false
			}
			return true
		}
	}
	if me.Button != gotui.MouseLeft || me.Action != gotui.MousePress {
		return false
	}
	r := c.transcriptRegion.ContentRect()
	_, contentHeight := c.transcriptRegion.ContentSize()
	_, maxScroll := c.transcriptRegion.MaxScroll()
	// No overflow means the library does not paint a scrollbar.
	if r.Width <= 0 || r.Height <= 0 || maxScroll <= 0 || contentHeight <= r.Height ||
		me.X != r.X+r.Width-1 || me.Y < r.Y || me.Y >= r.Y+r.Height {
		return false
	}
	c.clearTranscriptSelection()
	thumbHeight := max(1, r.Height*r.Height/contentHeight)
	travel := r.Height - thumbHeight
	_, offset := c.transcriptRegion.ScrollOffset()
	thumbTop := offset * travel / maxScroll
	y := me.Y - r.Y
	if y >= thumbTop && y < thumbTop+thumbHeight {
		c.scrollbarGrab = y - thumbTop
	} else {
		// A track press jumps to that position, centered on the thumb.
		c.scrollbarGrab = thumbHeight / 2
	}
	c.scrollbarDragging = true
	c.moveTranscriptScrollbar(me.Y)
	return true
}

func (c *chatTUI) moveTranscriptScrollbar(y int) {
	r := c.transcriptRegion.ContentRect()
	_, contentHeight := c.transcriptRegion.ContentSize()
	_, maxScroll := c.transcriptRegion.MaxScroll()
	if r.Height <= 0 || contentHeight <= r.Height || maxScroll <= 0 {
		c.scrollbarDragging = false
		return
	}
	travel := r.Height - max(1, r.Height*r.Height/contentHeight)
	if travel <= 0 {
		return
	}
	thumbTop := min(travel, max(0, y-r.Y-c.scrollbarGrab))
	c.setTranscriptPosition(thumbTop * maxScroll / travel)
	c.stickToBottom = c.transcriptScroll >= maxScroll
	if c.app != nil {
		c.app.MarkDirty()
	}
}
