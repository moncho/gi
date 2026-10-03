package tui

import (
	"strings"

	gotui "github.com/grindlemire/go-tui"
)

// editorDialog is Pi's ctx.ui.editor dialog (ExtensionEditorComponent):
// a title over a multi-line editor. Enter submits, Shift+Enter or Ctrl+J
// adds a line, Escape or Ctrl+C cancels and Ctrl+G edits the text in the
// external editor. Golden: scripts/golden-editor-dialog.mjs.
type editorDialog struct {
	title    string
	text     string
	onSubmit func(string)
	onCancel func()
}

// openEditorDialog shows the dialog in the selector area.
func (c *chatTUI) openEditorDialog(title, prefill string, onSubmit func(string), onCancel func()) {
	c.editorDialog = &editorDialog{title: title, text: prefill, onSubmit: onSubmit, onCancel: onCancel}
	c.openMenuKind("editor-dialog")
}

func (c *chatTUI) closeEditorDialog() {
	c.editorDialog = nil
	c.closeModelMenu()
}

func (c *chatTUI) editorDialogKeys() gotui.KeyMap {
	d := c.editorDialog
	return editorDialogKeys(d, c.markDirty, func(f func()) {
		if c.app == nil {
			return
		}
		command := c.externalEditorCommand()
		c.app.RunWithTerminal(func() {
			if text, ok := editInExternalEditor(command, d.text); ok {
				d.text = text
			}
			f()
		})
	}, func(text string, submit bool) {
		c.closeEditorDialog()
		if submit {
			d.onSubmit(text)
		} else if d.onCancel != nil {
			d.onCancel()
		}
	})
}

// editorDialogKeys binds Pi's keys; done closes the dialog, submitting the
// text (the editor clears, as Pi's does) or cancelling.
func editorDialogKeys(d *editorDialog, dirty func(), external func(func()), done func(text string, submit bool)) gotui.KeyMap {
	newline := func(gotui.KeyEvent) { d.text += "\n"; dirty() }
	cancel := func(gotui.KeyEvent) { done(d.text, false) }
	return gotui.KeyMap{
		gotui.OnPreemptStop(gotui.KeyEnter.Shift(), newline),
		gotui.OnPreemptStop(gotui.KeyCtrlJ, newline),
		gotui.OnPreemptStop(gotui.KeyEnter, func(gotui.KeyEvent) {
			text := d.text
			d.text = ""
			done(text, true)
		}),
		gotui.OnPreemptStop(gotui.KeyEscape, cancel),
		gotui.OnPreemptStop(gotui.KeyCtrlC, cancel),
		gotui.OnPreemptStop(gotui.KeyCtrlG, func(gotui.KeyEvent) { external(dirty) }),
		gotui.OnPreemptStop(gotui.KeyBackspace, func(gotui.KeyEvent) {
			if r := []rune(d.text); len(r) > 0 {
				d.text = string(r[:len(r)-1])
				dirty()
			}
		}),
		gotui.OnFocused(gotui.AnyRune, func(ke gotui.KeyEvent) {
			d.text += string(ke.Rune)
			dirty()
		}),
	}
}

// rows is Pi's layout: borders around the title, the editor (Pi's Editor
// with its own borders) and the key hints.
func (d *editorDialog) rows(width int) spanRows {
	padded := func(spans []gotui.TextSpan) spanRows {
		var out spanRows
		for _, line := range piWrapLine(spans, max(1, width-2)) {
			out = append(out, append([]gotui.TextSpan{{Text: " "}}, line...))
		}
		return out
	}
	rows := spanRows{piRule(width, piBorder), nil}
	rows = append(rows, padded([]gotui.TextSpan{{Text: d.title, Style: piFg(piAccent)}})...)
	rows = append(rows, nil, piRule(width, piBorder))
	lines := strings.Split(d.text, "\n")
	for i, line := range lines {
		spans := []gotui.TextSpan{{Text: line}}
		if i == len(lines)-1 {
			spans = append(spans, gotui.TextSpan{Text: " ", Style: gotui.NewStyle().Reverse()})
		}
		rows = append(rows, piWrapLine(spans, max(1, width))...)
	}
	rows = append(rows, piRule(width, piBorder), nil)
	var hint []gotui.TextSpan
	for i, h := range [][2]string{{"enter", "submit"}, {"shift+enter/ctrl+j", "newline"}, {"escape/ctrl+c", "cancel"}, {"ctrl+g", "external editor"}} {
		if i > 0 {
			hint = append(hint, gotui.TextSpan{Text: "  "})
		}
		hint = append(hint, keyHintSpans(h[0], h[1])...)
	}
	rows = append(rows, padded(hint)...)
	return append(rows, nil, piRule(width, piBorder))
}
