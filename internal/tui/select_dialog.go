package tui

import (
	gotui "github.com/grindlemire/go-tui"
)

// Pi's ctx.ui.select (ExtensionSelectorComponent): a titled list of options
// that replaces the editor. Up/Down (or k/j) move without wrapping, Enter
// selects, Escape or Ctrl+C cancels, Ctrl+O toggles tool output.

type selectDialog struct {
	title    string
	onSelect func(choice string)
	onCancel func()
}

// openSelect shows Pi's selector; onCancel may be nil.
func (c *chatTUI) openSelect(title string, options []string, onSelect func(string), onCancel func()) {
	if len(options) == 0 {
		return
	}
	c.selectDialog = selectDialog{title: title, onSelect: onSelect, onCancel: onCancel}
	c.modelMenuOpen = true
	c.modelMenuKind = "select"
	c.modelMenuAll = options
	c.modelMenuChoices = options
	c.modelMenuValues = nil
	c.modelMenuQuery = ""
	c.modelMenuSelected = 0
	c.modelMenuScroll = 0
	c.modelMenuError = ""
	c.inputActive = false
	if c.app != nil {
		c.app.BlurFocused()
		c.app.MarkDirty()
	}
}

func (c *chatTUI) finishSelect(choice string, cancelled bool) {
	dialog := c.selectDialog
	c.selectDialog = selectDialog{}
	c.closeModelMenu()
	switch {
	case cancelled && dialog.onCancel != nil:
		dialog.onCancel()
	case !cancelled && dialog.onSelect != nil:
		dialog.onSelect(choice)
	}
	if c.app != nil {
		c.app.MarkDirty()
	}
}

func (c *chatTUI) selectDialogKeys() gotui.KeyMap {
	move := func(delta int) {
		c.modelMenuSelected = max(0, min(len(c.modelMenuChoices)-1, c.modelMenuSelected+delta))
		if c.app != nil {
			c.app.MarkDirty()
		}
	}
	confirm := func(gotui.KeyEvent) {
		if i := c.modelMenuSelected; i >= 0 && i < len(c.modelMenuChoices) {
			c.finishSelect(c.modelMenuChoices[i], false)
		}
	}
	cancel := func(gotui.KeyEvent) { c.finishSelect("", true) }
	return gotui.KeyMap{
		gotui.OnPreemptStop(gotui.KeyEscape, cancel),
		gotui.OnPreemptStop(gotui.KeyCtrlC, cancel),
		gotui.OnPreemptStop(gotui.KeyUp, func(gotui.KeyEvent) { move(-1) }),
		gotui.OnPreemptStop(gotui.KeyDown, func(gotui.KeyEvent) { move(1) }),
		gotui.OnPreemptStop(gotui.Rune('k'), func(gotui.KeyEvent) { move(-1) }),
		gotui.OnPreemptStop(gotui.Rune('j'), func(gotui.KeyEvent) { move(1) }),
		gotui.OnPreemptStop(gotui.KeyEnter, confirm),
		gotui.OnPreemptStop(gotui.Rune('o').Ctrl(), func(gotui.KeyEvent) { c.toggleToolOutput() }),
	}
}

// piSelectDialogRows renders ExtensionSelectorComponent: border, title,
// options, key hints, border. Each line is a pi-tui Text with one column of
// padding, word-wrapped to the remaining width.
func (c *chatTUI) piSelectDialogRows(width int) spanRows {
	var rows spanRows
	text := func(spans ...gotui.TextSpan) {
		for _, line := range piWrapLine(spans, max(1, width-2)) {
			rows = append(rows, append([]gotui.TextSpan{{Text: " "}}, line...))
		}
	}
	rows = append(rows, piRule(width, piBorder), nil)
	text(gotui.TextSpan{Text: c.selectDialog.title, Style: piFg(piAccent).Bold()})
	rows = append(rows, nil)
	for i, option := range c.modelMenuChoices {
		if i == c.modelMenuSelected {
			text(gotui.TextSpan{Text: "→ ", Style: piFg(piAccent)}, gotui.TextSpan{Text: option, Style: piFg(piAccent)})
		} else {
			text(gotui.TextSpan{Text: "  "}, gotui.TextSpan{Text: option, Style: piFg(piText)})
		}
	}
	rows = append(rows, nil)
	hints := keyHintSpans("↑↓", "navigate")
	hints = append(hints, gotui.TextSpan{Text: "  "})
	hints = append(hints, keyHintSpans("enter", "select")...)
	hints = append(hints, gotui.TextSpan{Text: "  "})
	hints = append(hints, keyHintSpans("escape/ctrl+c", "cancel")...)
	text(hints...)
	return append(rows, nil, piRule(width, piBorder))
}
