package tui

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	gotui "github.com/grindlemire/go-tui"
)

func TestEditorViewportTracksCursorWithoutChangingDraft(t *testing.T) {
	for _, width := range []int{1, 2, 9, 60, 100, 140} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := newMultilineInput(width, "", nil, nil)
			m.Focus()
			m.maxLines = 5
			original := strings.Repeat("中文🙂e\u0301 draft abc\n", 80) + "tail"
			m.SetText(original)
			m.undoText = "previous"
			m.yankText = "yank"
			for _, cursor := range []int{0, 1, 4, 19, utf8.RuneCountInString(original) / 2, utf8.RuneCountInString(original)} {
				m.cursorPos = cursor
				lines := m.renderLines()
				if len(lines) > 5 || len(lines) == 0 {
					t.Fatal("unbounded", len(lines))
				}
				markers := 0
				for _, line := range lines {
					if line.cursor >= 0 {
						markers++
					}
					if !utf8.ValidString(line.text) || gotui.StringWidth(line.text) > width {
						t.Fatalf("width%d overflow %q", width, line.text)
					}
				}
				if markers != 1 {
					t.Fatalf("cursor%d markers%d: %#v", cursor, markers, lines)
				}
				if m.Text() != original || m.cursorPos != cursor || m.undoText != "previous" || m.yankText != "yank" {
					t.Fatal("editor state changed")
				}
			}
			m.cursorPos = 0 // the draft's start (Home is line start)
			m.markDirty()
			if m.scrollRow == 0 {
				t.Fatal("expected old viewport until render")
			}
			m.renderLines()
			if m.scrollRow != 0 {
				t.Fatal("home not shown")
			}
			m.cursorPos = len([]rune(m.text))
			m.markDirty()
			m.renderLines()
			if m.scrollRow == 0 {
				t.Fatal("end not shown")
			}
			m.maxLines = 2
			lines := m.renderLines()
			if len(lines) != 2 || lines[1].cursor < 0 {
				t.Fatal("shrunk viewport lost cursor", lines)
			}
			m.SetText("")
			if lines = m.renderLines(); len(lines) != 1 || lines[0].text != "" || lines[0].cursor != 0 {
				t.Fatal("idle height", lines)
			}
		})
	}
}

func TestEditorViewportCellWrappingAndIndependentState(t *testing.T) {
	m := newMultilineInput(6, "", nil, nil)
	m.SetText("中🙂e\u0301")
	lines := m.renderLines()
	if len(lines) != 1 || lines[0].text != "中🙂e\u0301" {
		t.Fatal(lines)
	}
	m.width = 5
	m.Focus()
	m.cursorPos = 3
	lines = m.renderLines()
	if m.Text() != "中🙂e\u0301" || m.cursorPos != 3 {
		t.Fatal("split draft")
	}
	// Pi wraps between CJK characters and highlights the whole cluster
	// containing the cursor instead of inserting a marker.
	if len(lines) != 2 || lines[0].text != "中" || lines[1].text != "🙂e\u0301" || lines[1].text[lines[1].cursor:lines[1].cursorEnd] != "e\u0301" {
		t.Fatalf("cursor split combining cluster %#v", lines)
	}
	for _, line := range lines {
		if gotui.StringWidth(line.text) > 5 {
			t.Fatal("wide overflow")
		}
	}
	m.SetText("first\r\nlast")
	m.maxLines = 1
	lines = m.renderLines()
	if lines[0].text != "last" || lines[0].cursor != 4 || lines[0].cursorEnd != 4 {
		t.Fatal("CRLF", lines)
	}
	// Independent search and composer instances cannot share scroll positions.
	other := newMultilineInput(5, "", nil, nil)
	other.maxLines = 1
	other.Focus()
	other.SetText("find")
	other.renderLines()
	if other.scrollRow != 0 || m.scrollRow == 0 {
		t.Fatal("shared viewport")
	}
}

func TestEditorViewportBudgetAndMenuReserve(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {100, 22}, {140, 36}, {30, 10}} {
		c, _, _ := modelTestChat(t)
		c.outputWidth, c.outputHeight = size[0], size[1]
		c.input.SetText(strings.Repeat("long draft\n", 80))
		c.input.width = size[0]
		pad := 0
		if size[0] >= 80 && size[1] >= 20 {
			pad = 1
		}
		footer := len(c.footerLines(size[0]))
		c.boundEditor(size[1], pad, footer, 0, 0, false)
		if len(c.input.renderLines()) > max(5, size[1]*3/10) {
			t.Fatal("budget")
		}
		c.openModelMenu()
		height := c.modelMenuHeight()
		c.boundEditor(size[1], pad, footer, 0, height, false)
		// Pi's model selector replaces the editor: spacer + selector + footer
		// must fit the screen.
		if size[1] >= 18 && 2*pad+footer+1+height > size[1] {
			t.Fatal("menu/editor hides transcript", size, height, c.input.maxLines)
		}
		c.closeModelMenu()
		c.boundEditor(size[1], 0, footer, 3, 0, true)
		if 2+footer+3+len(c.input.renderLines()) >= size[1] && size[1] >= 18 {
			t.Fatal("regular history lost")
		}
	}
}
