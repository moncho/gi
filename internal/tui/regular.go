package tui

import (
	"context"
	"database/sql"
	"errors"
	"time"

	gotui "github.com/grindlemire/go-tui"
)

func (c *chatTUI) bindTranscriptNavigation() {
	if c.regularMode {
		c.input.onTranscriptTop, c.input.onTranscriptEnd = nil, nil
	} else {
		c.input.onTranscriptTop, c.input.onTranscriptEnd = c.scrollTranscriptToTop, c.scrollTranscriptToBottom
	}
}

// Regular mode never rewrites terminal-owned rows. Wait for native claim release
// before baking mutable draft/tool blocks into the terminal history.
func (c *chatTUI) regularBusy() bool {
	if c.running || c.compaction.active {
		return true
	}
	if c.store != nil && c.sessionID != "" {
		if _, _, err := c.store.GetSessionActiveTurn(context.Background(), c.sessionID); !errors.Is(err, sql.ErrNoRows) {
			return true // Unknown activity must not commit a mutable preview.
		}
	}
	return false
}

func (c *chatTUI) regularPending() []string {
	start := min(max(0, c.regularPrinted), len(c.transcript))
	return c.transcript[start:]
}

func (c *chatTUI) flushRegularTranscript() {
	if !c.regularMode || c.app == nil || c.workspaceIndex.active || c.modelMenuAltScreen {
		return
	}
	if c.regularSessionPending {
		c.regularSessionPending = false
		c.app.PrintAboveln("sys: session %s", c.sessionID)
	}
	if c.regularBusy() {
		return
	}
	// Native completion and legacy final-response delivery are separate queued
	// events. Never bake a draft span before its final replacement arrives.
	end := c.regularStableEnd()
	if end <= c.regularPrinted {
		return
	}
	lines := c.transcript[c.regularPrinted:end]
	root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100))
	// Print complete retained output. Baked scrollback cannot later expand in place.
	previousKind := c.regularPreviousKind()
	for _, block := range c.buildTranscriptRenderableBlocks(lines) {
		if block.Kind == "thinking_indicator" {
			continue
		}
		block.Expanded, block.Static = true, true
		root.AddChild(c.renderTranscriptBlockAfter(block, previousKind))
		if block.Kind != "thinking_indicator" {
			previousKind = block.Kind
		}
	}
	c.regularPrinted = end
	c.app.PrintAboveElement(root)
}

// Regular-mode resize repair. When the terminal grows, go-tui clears the rows
// between the dock's old and new start row, assuming they hold a stale copy
// of the dock. Terminals such as tmux pull scrollback down into exactly that
// band instead, so the newest transcript rows above the dock were erased.
// Once resizing settles, gi repaints just that band (the rows directly above
// the dock) with the newest transcript rows at the current width, and resets
// go-tui's model of the history rows. Other rows are never touched, so the
// user's scrollback (shell history, earlier output) is neither erased nor
// duplicated.
// Long enough to treat a window drag as one burst: an intermediate repaint
// would be pushed into scrollback by the next shrink and then repeated.
const regularReflowDelay = 250 * time.Millisecond

// noteRegularResize records the size before a resize burst and the tallest
// height reached, which bounds the band go-tui cleared.
func (c *chatTUI) noteRegularResize(oldWidth, oldHeight, newWidth, newHeight int) {
	if c.regularResizeBase == 0 {
		c.regularResizeBase = oldHeight
		c.regularResizeBaseWidth = oldWidth
	}
	if newWidth != oldWidth {
		c.regularResizeWidthChanged = true
	}
	c.regularResizePeak = max(c.regularResizePeak, max(oldHeight, newHeight))
	c.scheduleRegularReflow()
}

func (c *chatTUI) scheduleRegularReflow() {
	if !c.regularMode || c.app == nil {
		return
	}
	c.regularReflowGen++
	gen, app := c.regularReflowGen, c.app
	time.AfterFunc(regularReflowDelay, func() {
		app.QueueUpdate(func() {
			if gen == c.regularReflowGen {
				c.reflowRegular()
			}
		})
	})
}

// regularTranscriptRows renders the printed transcript at width into a
// buffer (all blocks expanded, as printed to scrollback).
func (c *chatTUI) regularTranscriptRows(width int) (*gotui.Buffer, int) {
	end := min(c.regularPrinted, len(c.transcript))
	if end <= 0 || width <= 0 {
		return nil, 0
	}
	root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(width))
	previousKind := ""
	for _, block := range c.buildTranscriptRenderableBlocks(c.transcript[:end]) {
		if block.Kind == "thinking_indicator" {
			continue
		}
		block.Expanded, block.Static = true, true
		root.AddChild(c.renderTranscriptBlockAfter(block, previousKind))
		previousKind = block.Kind
	}
	height := root.HeightForWidth(width)
	if height <= 0 {
		return nil, 0
	}
	buf := gotui.NewBuffer(width, height)
	root.RenderTo(buf, width, height)
	return buf, height
}

// regularRepairChanges paints the newest `rows` transcript rows into screen
// rows [top, top+rows), in place: nothing scrolls.
func (c *chatTUI) regularRepairChanges(width, top, rows int) []gotui.CellChange {
	if rows <= 0 || width <= 0 || top < 0 {
		return nil
	}
	buf, height := c.regularTranscriptRows(width)
	changes := make([]gotui.CellChange, 0, rows*4)
	offset := height - rows // transcript row shown on screen row top
	for y := 0; y < rows; y++ {
		src := y + offset
		if buf == nil || src < 0 {
			changes = append(changes, gotui.CellChange{X: 0, Y: top + y, EraseToEOL: true})
			continue
		}
		last := -1
		for x := 0; x < width; x++ {
			if cell := buf.Cell(x, src); !(cell.Rune == 0 || cell.Rune == ' ') || cell.Style != (gotui.Style{}) {
				last = x
			}
		}
		for x := 0; x <= last; x++ {
			cell := buf.Cell(x, src)
			if cell.Width == 0 && cell.Rune == 0 && x > 0 && buf.Cell(x-1, src).Width == 2 {
				continue // continuation of a wide cluster
			}
			if cell.Rune == 0 {
				cell = gotui.Cell{Rune: ' ', Style: cell.Style, Width: 1, Link: cell.Link}
			}
			changes = append(changes, gotui.CellChange{X: x, Y: top + y, Cell: cell})
		}
		changes = append(changes, gotui.CellChange{X: last + 1, Y: top + y, EraseToEOL: true})
	}
	return changes
}

func (c *chatTUI) reflowRegular() {
	if !c.regularMode || c.app == nil || c.workspaceIndex.active || c.modelMenuAltScreen {
		return
	}
	band := max(0, c.regularResizePeak-c.regularResizeBase)
	widthChanged := c.regularResizeWidthChanged
	c.regularResizeBase, c.regularResizePeak, c.regularResizeBaseWidth, c.regularResizeWidthChanged = 0, 0, 0, false
	w, h := c.app.Size()
	start := h - c.app.InlineHeight()
	if widthChanged {
		// The terminal rewrapped rows (including the dock drawn at another
		// width) during the burst: repaint all visible rows above the dock
		// at the settled width, as Pi re-renders.
		band = start
	}
	band = min(band, start)
	if changes := c.regularRepairChanges(w, start-band, band); len(changes) > 0 {
		c.app.Terminal().Flush(changes)
	}
	c.resetInlineHistoryModel()
}

func (c *chatTUI) renderRegular(app *gotui.App) *gotui.Element {
	w, h := app.Size()
	if c.workspaceIndex.active && c.regularWidth != 0 && (w != c.regularWidth || h != c.regularHeight) {
		c.workspaceIndex.resized = true
	}
	if c.modelMenuAltScreen && c.regularWidth != 0 && (w != c.regularWidth || h != c.regularHeight) {
		c.modelMenuResized = true
		// Clear only the temporary visible screen, never main scrollback.
		app.Terminal().SetCursor(0, 0)
		app.Terminal().ClearToEnd()
	}
	if !c.workspaceIndex.active && !c.modelMenuAltScreen && c.regularWidth != 0 && (w != c.regularWidth || h != c.regularHeight) {
		c.noteRegularResize(c.regularWidth, c.regularHeight, w, h)
	}
	c.regularWidth, c.regularHeight = w, h
	c.outputWidth, c.outputHeight = w, h
	if c.workspaceIndex.active {
		// Retain the previous inline height/layout while the alternate screen is
		// temporary, including multiline editor height. Only five rows contain UI.
		root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100), gotui.WithHeight(5))
		root.AddChild(c.renderWorkspaceIndex(w))
		return root
	}
	if c.modelMenuAltScreen {
		// Preserve the main-screen inline dock while a temporary selector owns
		// this screen, as with the bounded workspace-index panel above.
		// Keep the largest temporary selector region until close. The buffer
		// clears its unused rows when switching to a shorter action submenu,
		// rather than stranding old rows above a shrunken inline region.
		if c.modelMenuKind == "session" || c.modelMenuKind == "session-actions" || c.modelMenuKind == "session-rename" {
			c.modelMenuRenderedHeight = max(c.modelMenuRenderedHeight, c.modelMenuHeight())
		} else {
			c.modelMenuRenderedHeight = c.modelMenuHeight()
		}
		app.SetInlineHeight(max(c.modelMenuInlineHeight, c.modelMenuRenderedHeight))
		root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100), gotui.WithHeight(c.modelMenuRenderedHeight))
		root.AddChild(c.renderModelMenu(w))
		return root
	}
	c.ensureInput()
	c.input.width = w
	c.input.suspended = c.modelMenuOpen
	footer := c.footerLines(w)
	queue := c.pendingQueueLines(w, h)
	// Pi's Spacer(1) above the editor (widget container).
	widgets := append([]string{""}, c.extensionWidgetLines()...)
	if c.editorAskActive {
		widgets = append(widgets, "? "+c.editorAskPrompt+" (Enter submit · Esc cancel)")
	}
	menuHeight := c.modelMenuHeight() + c.slashMenuHeight()
	// Active output is temporary and bounded; the idle dock has only editor,
	// separators and existing footer. Leave at least one terminal-owned history row.
	previewHeight := 0
	pending := c.regularPending()
	if len(pending) > 0 && c.regularBusy() {
		previewHeight = min(3, len(pending))
		// A three-row tail hides table headers and most streamed cells. Keep
		// a bounded larger live region for source-backed tables; completed
		// output is still committed once to terminal-owned scrollback.
		for _, line := range pending {
			if meta, ok := parseTranscriptBlockMarker(line); ok && meta.MarkdownSource != "" {
				previewHeight = min(len(pending), max(3, h/2))
				break
			}
		}
	}
	c.boundEditor(h, 0, len(footer), len(widgets)+len(queue)+previewHeight, menuHeight, true)
	input := app.MountPersistent(c, 0, func() gotui.Component { return c.input })
	inputHeight := max(1, input.HeightForWidth(w))
	dock := min(h-1, 2+inputHeight+len(footer)+len(widgets)+len(queue)+menuHeight+previewHeight)
	app.SetInlineHeight(max(1, dock))
	root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100), gotui.WithHeight(dock))
	c.transcriptBlockRefs = nil
	if previewHeight > 0 {
		preview := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100), gotui.WithHeight(previewHeight), gotui.WithScrollable(gotui.ScrollVertical), gotui.WithScrollbarHidden(true))
		previousKind := c.regularPreviousKind()
		for _, block := range c.buildTranscriptRenderableBlocks(pending) {
			preview.AddChild(c.renderTranscriptBlockAfter(block, previousKind))
			if block.Kind != "thinking_indicator" {
				previousKind = block.Kind
			}
		}
		preview.ScrollToBottom()
		root.AddChild(preview)
	}
	if len(queue) > 0 {
		root.AddChild(c.renderLineBlock(queue, piFg(piDim)))
	}
	if c.modelMenuOpen {
		root.AddChild(c.renderModelMenu(w))
	}
	if len(widgets) > 0 {
		root.AddChild(c.renderLineBlock(widgets, gotui.NewStyle()))
	}
	root.AddChild(c.renderEditorTopBorder(c.input, w))
	root.AddChild(input)
	root.AddChild(c.renderEditorBottomBorder(c.input, w))
	if c.slash.active {
		root.AddChild(c.renderSlashMenu(w))
	}
	root.AddChild(c.renderFooter(w))
	c.inputRegion = input
	return root
}

// Terminal-owned scrollback cannot be navigated or mutated by app shortcuts.
// Leave Home/End to the editor; the terminal/multiplexer owns history keys.
func regularKeyMap(bindings gotui.KeyMap) gotui.KeyMap {
	out := make(gotui.KeyMap, 0, len(bindings))
	for _, b := range bindings {
		switch b.Pattern.Key {
		case gotui.KeyPageUp, gotui.KeyPageDown, gotui.KeyHome, gotui.KeyEnd, gotui.KeyF6, gotui.KeyF7, gotui.KeyF8:
			continue
		}
		if b.Pattern.Rune == 'o' && b.Pattern.Mod == gotui.ModCtrl {
			continue
		}
		out = append(out, b)
	}
	return out
}

func (c *chatTUI) regularStableEnd() int {
	end := len(c.transcript)
	if c.draftLineIndex >= 0 && c.draftLineCount > 0 {
		end = min(end, c.draftLineIndex)
	}
	for i := c.regularPrinted; i < end; i++ {
		if meta, ok := parseTranscriptBlockMarker(c.transcript[i]); ok && meta.Status == "running" {
			end = i
			break
		}
	}
	return max(c.regularPrinted, end)
}

// Regular mode prints one stable slice at a time. Preserve message spacing
// even when the assistant response was printed in an earlier slice.
func (c *chatTUI) regularPreviousKind() string {
	if c.regularPrinted <= 0 {
		return ""
	}
	end := min(c.regularPrinted, len(c.transcript))
	blocks := c.buildTranscriptRenderableBlocks(c.transcript[:end])
	for i := len(blocks) - 1; i >= 0; i-- {
		if blocks[i].Kind != "thinking_indicator" {
			return blocks[i].Kind
		}
	}
	return ""
}

// resetInlineHistoryModel tells go-tui that every history row above the dock
// holds content. go-tui models which rows are blank and prints into rows it
// believes are blank without scrolling them into scrollback. That model goes
// stale when gi paints rows itself (resize repaint) or when the dock height
// changes on a temporary alternate screen (selectors, workspace index): the
// scrolls it emitted there never touched the main screen. A width change
// invalidates the model (go-tui has no direct API); the next print, which is
// empty and writes nothing, re-establishes it conservatively as full. The
// final same-size resize also forces a full dock redraw.
func (c *chatTUI) resetInlineHistoryModel() {
	if c.app == nil || !c.regularMode {
		return
	}
	w, h := c.app.Size()
	c.app.Dispatch(gotui.ResizeEvent{Width: max(1, w-1), Height: h})
	c.app.Dispatch(gotui.ResizeEvent{Width: w, Height: h})
	c.app.PrintAbove("")
	c.app.MarkDirty()
}
