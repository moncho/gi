package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
	gotui "github.com/grindlemire/go-tui"
)

type multilineInput struct {
	app              *gotui.App
	width            int
	maxLines         int // zero leaves standalone inputs unbounded
	scrollRow        int
	totalRows        int
	layoutCache      *editorLayoutCache // immutable, one entry per input snapshot
	border           gotui.BorderStyle
	placeholder      string
	placeholderStyle gotui.Style
	textStyle        gotui.Style
	autoFocus        bool
	suspended        bool
	onSubmit         func(string)
	onFollowUp       func(string)
	onShiftEnter     func()
	onNewline        func()
	onRestoreQueued  func()
	onEscape         func() bool
	onTranscriptTop  func()
	onTranscriptEnd  func()
	onComplete       func(string, int) (string, int, bool)
	// interceptKey lets an open autocomplete list take Tab/Enter/Escape
	// before the editor's own bindings (Pi's Editor autocomplete mode).
	interceptKey func(gotui.Key) bool
	onChange     func(string)
	onEdit       func()
	text         string
	cursorPos    int
	undoText     string
	undoCursor   int
	hasUndo      bool
	yankText     string
	focused      bool
}

func newMultilineInput(width int, placeholder string, onSubmit func(string), onChange func(string)) *multilineInput {
	return &multilineInput{
		width:            width,
		border:           gotui.BorderNone,
		placeholder:      placeholder,
		placeholderStyle: gotui.NewStyle().Dim(),
		textStyle:        gotui.NewStyle(),
		autoFocus:        true,
		onSubmit:         onSubmit,
		onChange:         onChange,
	}
}

func (m *multilineInput) BindApp(app *gotui.App) { m.app = app }
func (m *multilineInput) Text() string           { return m.text }
func (m *multilineInput) SetText(s string) {
	m.text = s
	m.cursorPos = utf8.RuneCountInString(s)
	m.notifyChanged()
}
func (m *multilineInput) Clear()            { m.SetText("") }
func (m *multilineInput) IsFocusable() bool { return !m.suspended }
func (m *multilineInput) IsTabStop() bool   { return !m.suspended }
func (m *multilineInput) IsFocused() bool   { return m.focused }
func (m *multilineInput) Focus() {
	m.focused = true
}
func (m *multilineInput) Blur() { m.focused = false }

// Watchers: none. The cursor is steady like Pi's (reverse video, no
// blink); a blink timer re-rendered the whole UI twice a second when idle.
func (m *multilineInput) Watchers() []gotui.Watcher { return nil }

func (m *multilineInput) KeyMap() gotui.KeyMap {
	if m.suspended {
		return nil
	}
	return gotui.KeyMap{
		gotui.OnFocused(gotui.AnyRune, m.insertRune),
		gotui.OnFocused(gotui.KeyBackspace, func(ke gotui.KeyEvent) { m.backspace() }),
		gotui.OnFocused(gotui.KeyDelete, func(ke gotui.KeyEvent) { m.delete() }),
		gotui.OnFocused(gotui.KeyLeft, func(ke gotui.KeyEvent) { m.moveLeft() }),
		gotui.OnFocused(gotui.KeyLeft.Alt(), func(ke gotui.KeyEvent) { m.moveWordLeft() }),
		gotui.OnFocused(gotui.KeyRight, func(ke gotui.KeyEvent) { m.moveRight() }),
		gotui.OnFocused(gotui.KeyRight.Alt(), func(ke gotui.KeyEvent) { m.moveWordRight() }),
		gotui.OnFocused(gotui.KeyHome, func(ke gotui.KeyEvent) {
			if m.onTranscriptTop != nil {
				m.onTranscriptTop()
			} else {
				m.moveHome()
			}
		}),
		gotui.OnFocused(gotui.KeyEnd, func(ke gotui.KeyEvent) {
			if m.onTranscriptEnd != nil {
				m.onTranscriptEnd()
			} else {
				m.moveEnd()
			}
		}),
		gotui.OnFocused(gotui.KeyHome.Ctrl(), func(ke gotui.KeyEvent) { m.moveHome() }),
		gotui.OnFocused(gotui.KeyEnd.Ctrl(), func(ke gotui.KeyEvent) { m.moveEnd() }),
		gotui.OnFocused(gotui.KeyCtrlA, func(ke gotui.KeyEvent) { m.moveHome() }),
		gotui.OnFocused(gotui.KeyCtrlE, func(ke gotui.KeyEvent) { m.moveEnd() }),
		gotui.OnFocused(gotui.KeyCtrlU, func(ke gotui.KeyEvent) { m.deleteToLineStart() }),
		gotui.OnFocused(gotui.KeyCtrlK, func(ke gotui.KeyEvent) { m.deleteToLineEnd() }),
		gotui.OnFocused(gotui.KeyCtrlW, func(ke gotui.KeyEvent) { m.deleteWordBackward() }),
		gotui.OnFocused(gotui.KeyBackspace.Alt(), func(ke gotui.KeyEvent) { m.deleteWordBackward() }),
		gotui.OnFocused(gotui.KeyDelete.Alt(), func(ke gotui.KeyEvent) { m.deleteWordForward() }),
		gotui.OnFocused(gotui.KeyCtrlZ, func(ke gotui.KeyEvent) { m.undo() }),
		gotui.OnFocused(gotui.KeyCtrlY, func(ke gotui.KeyEvent) { m.yank() }),
		gotui.OnFocused(gotui.KeyTab, func(ke gotui.KeyEvent) {
			if m.interceptKey != nil && m.interceptKey(gotui.KeyTab) {
				return
			}
			m.complete()
		}),
		gotui.OnFocused(gotui.KeyEnter, func(ke gotui.KeyEvent) {
			if m.interceptKey != nil && m.interceptKey(gotui.KeyEnter) {
				return
			}
			m.enter(ke)
		}),
		gotui.OnFocused(gotui.KeyEnter.Shift(), m.enter),
		gotui.OnFocused(gotui.KeyCtrlJ, func(ke gotui.KeyEvent) {
			if m.onNewline != nil {
				m.onNewline()
			} else {
				m.insertLiteral('\n')
			}
		}),
		gotui.OnFocused(gotui.KeyEnter.Alt(), m.enter),
		gotui.OnFocused(gotui.KeyUp.Alt(), func(ke gotui.KeyEvent) {
			if m.onRestoreQueued != nil {
				m.onRestoreQueued()
			}
		}),
		gotui.OnFocused(gotui.KeyEscape, func(_ gotui.KeyEvent) {
			if m.interceptKey != nil && m.interceptKey(gotui.KeyEscape) {
				return
			}
			if m.onEscape != nil {
				m.onEscape()
			}
			// Pi's editor retains focus after an idle Escape. Selection and
			// compaction handlers may consume the key, but no fallback blurs
			// the composer or discards its draft.
		}),
	}
}

func (m *multilineInput) Render(app *gotui.App) *gotui.Element {
	lines := m.renderLines()
	help := m.helpLine()
	if help != "" {
		lines = append(lines, renderedLine{text: help, placeholder: true, cursor: -1})
	}
	totalHeight := len(lines)
	if totalHeight < 1 {
		totalHeight = 1
	}
	if m.border != gotui.BorderNone {
		totalHeight += 2
	}
	root := gotui.New(
		gotui.WithDirection(gotui.Column),
		gotui.WithWidth(m.width),
		gotui.WithHeight(totalHeight),
		gotui.WithFocusable(!m.suspended),
		gotui.WithAutoFocus(m.autoFocus && !m.suspended),
		gotui.WithBorder(m.border),
	)
	root.SetOnFocus(func(e *gotui.Element) { m.Focus() })
	root.SetOnBlur(func(e *gotui.Element) { m.Blur() })
	for _, line := range lines {
		style := m.textStyle
		// The empty editor is just Pi's reverse-video cursor cell, not dim text.
		if line.placeholder && line.cursor < 0 {
			style = m.placeholderStyle
		}
		root.AddChild(renderEditorLine(line, style))
	}
	return root
}

// renderEditorLine draws Pi's fake cursor: the grapheme under the cursor in
// reverse video, or a reverse-video space at the end of the line. The draft
// text itself is never shifted by a cursor glyph.
func renderEditorLine(line renderedLine, style gotui.Style) *gotui.Element {
	options := []gotui.Option{gotui.WithHeight(1), gotui.WithWrap(false)}
	if line.cursor < 0 {
		return gotui.New(append(options, gotui.WithText(line.text), gotui.WithTextStyle(style))...)
	}
	before, under, after := line.text[:line.cursor], line.text[line.cursor:line.cursorEnd], line.text[line.cursorEnd:]
	if under == "" {
		under = " "
	}
	spans := make([]gotui.TextSpan, 0, 3)
	if before != "" {
		spans = append(spans, gotui.TextSpan{Text: before, Style: style})
	}
	spans = append(spans, gotui.TextSpan{Text: under, Style: style.Reverse()})
	if after != "" {
		spans = append(spans, gotui.TextSpan{Text: after, Style: style})
	}
	return gotui.New(append(options, gotui.WithRichText(spans...))...)
}

type renderedLine struct {
	text        string
	placeholder bool
	// cursor/cursorEnd are byte offsets of the reverse-video cursor grapheme
	// in text; cursor < 0 means no cursor, cursor == cursorEnd a trailing cell.
	cursor, cursorEnd int
}

// displayText is the row as painted, including a trailing cursor cell.
func (l renderedLine) displayText() string {
	if l.cursor >= 0 && l.cursor == l.cursorEnd {
		return l.text + " "
	}
	return l.text
}

type editorLayoutKey struct {
	text          string
	width, cursor int
	focused       bool
}
type editorLayoutCache struct {
	key       editorLayoutKey
	lines     []renderedLine
	cursorRow int
}

// editorLayoutWidth mirrors Pi's Editor with paddingX 0: one column is
// reserved so a cursor at the end of a full row still fits.
func editorLayoutWidth(width int) int {
	return max(1, width-1)
}

func (m *multilineInput) renderLines() []renderedLine {
	visibleWidth := m.width
	if m.border != gotui.BorderNone {
		visibleWidth -= 2
	}
	visibleWidth = editorLayoutWidth(visibleWidth)
	key := editorLayoutKey{text: m.text, width: visibleWidth, cursor: m.cursorPos, focused: m.focused}
	if m.text == "" {
		m.layoutCache = nil
		m.scrollRow = 0
		m.totalRows = 1
		if m.focused {
			return []renderedLine{{placeholder: true, cursor: 0, cursorEnd: 0}}
		}
		return []renderedLine{{placeholder: true, cursor: -1}}
	}
	cached := m.layoutCache
	if cached == nil || cached.key != key {
		lines, cursorRow := m.layoutLines(visibleWidth)
		cached = &editorLayoutCache{key: key, lines: lines, cursorRow: cursorRow}
		m.layoutCache = cached
	}
	lines, cursorRow := cached.lines, cached.cursorRow
	m.totalRows = len(lines)
	limit := m.maxLines
	if limit <= 0 || len(lines) <= limit {
		m.scrollRow = 0
		return lines[:len(lines):len(lines)]
	}
	if cursorRow < m.scrollRow {
		m.scrollRow = cursorRow
	}
	if cursorRow >= m.scrollRow+limit {
		m.scrollRow = cursorRow - limit + 1
	}
	m.scrollRow = max(0, min(m.scrollRow, len(lines)-limit))
	// Limit capacity so Render's optional append cannot overwrite cached rows.
	return lines[m.scrollRow : m.scrollRow+limit : m.scrollRow+limit]
}

// hiddenRows reports rows above and below the last rendered viewport, for
// Pi's "↑ N more" / "↓ N more" editor borders.
func (m *multilineInput) hiddenRows() (above, below int) {
	visible := m.totalRows - m.scrollRow
	if m.maxLines > 0 {
		visible = min(visible, m.maxLines)
	}
	return m.scrollRow, max(0, m.totalRows-m.scrollRow-visible)
}

type editorGrapheme struct {
	text      string
	width     int
	runeStart int
	runeCount int
}

// layoutLines ports Pi's Editor.layoutText/wordWrapLine: logical lines wrap at
// word boundaries (or between CJK characters), falling back to grapheme
// breaks for words longer than the row. The cursor is a rune offset.
func (m *multilineInput) layoutLines(visibleWidth int) ([]renderedLine, int) {
	cursorPos := max(0, min(m.cursorPos, utf8.RuneCountInString(m.text)))
	lines := []renderedLine{}
	cursorRow := -1
	var logical []editorGrapheme
	lineStart := 0
	runeOffset := 0
	flushLogical := func(lineEnd int) {
		hasCursor := cursorPos >= lineStart && cursorPos <= lineEnd && cursorRow < 0
		// Grapheme index holding the cursor; len(logical) means end of line.
		cursorIndex := len(logical)
		if hasCursor {
			for i, g := range logical {
				if cursorPos < g.runeStart+g.runeCount {
					cursorIndex = i
					break
				}
			}
		}
		chunks := editorWordWrap(logical, visibleWidth)
		for ci, chunk := range chunks {
			inChunk := false
			if hasCursor {
				if ci == len(chunks)-1 {
					inChunk = cursorIndex >= chunk[0]
				} else {
					inChunk = cursorIndex >= chunk[0] && cursorIndex < chunk[1]
				}
			}
			var b strings.Builder
			line := renderedLine{cursor: -1}
			for i := chunk[0]; i < chunk[1]; i++ {
				if inChunk && m.focused && i == cursorIndex {
					line.cursor = b.Len()
				}
				b.WriteString(logical[i].text)
				if inChunk && m.focused && i == cursorIndex {
					line.cursorEnd = b.Len()
				}
			}
			line.text = b.String()
			if inChunk {
				cursorRow = len(lines)
				if m.focused && cursorIndex >= chunk[1] {
					line.cursor, line.cursorEnd = len(line.text), len(line.text)
				}
			}
			lines = append(lines, line)
		}
		logical = logical[:0]
	}
	it := graphemes.FromString(m.text)
	for it.Next() {
		cluster := it.Value()
		count := utf8.RuneCountInString(cluster)
		if cluster == "\n" || cluster == "\r\n" {
			flushLogical(runeOffset)
			runeOffset += count
			lineStart = runeOffset
			continue
		}
		width := gotui.StringWidth(cluster)
		// A one-column terminal cannot paint a two-cell glyph. Substitute only
		// its display cell; the draft and logical cursor remain byte-identical.
		if width > visibleWidth {
			cluster, width = "�", 1
		}
		logical = append(logical, editorGrapheme{text: cluster, width: width, runeStart: runeOffset, runeCount: count})
		runeOffset += count
	}
	flushLogical(runeOffset)
	if cursorRow < 0 {
		cursorRow = len(lines) - 1
	}
	return lines, cursorRow
}

// editorWordWrap returns [start,end) grapheme ranges, as Pi's wordWrapLine.
func editorWordWrap(segments []editorGrapheme, maxWidth int) [][2]int {
	total := 0
	for _, g := range segments {
		total += g.width
	}
	if len(segments) == 0 || total <= maxWidth {
		return [][2]int{{0, len(segments)}}
	}
	var chunks [][2]int
	currentWidth, chunkStart := 0, 0
	wrapOppIndex, wrapOppWidth := -1, 0
	for i, g := range segments {
		isWs := isEditorWhitespace(g.text)
		if currentWidth+g.width > maxWidth {
			if wrapOppIndex >= 0 && currentWidth-wrapOppWidth+g.width <= maxWidth {
				chunks = append(chunks, [2]int{chunkStart, wrapOppIndex})
				chunkStart = wrapOppIndex
				currentWidth -= wrapOppWidth
			} else if chunkStart < i {
				chunks = append(chunks, [2]int{chunkStart, i})
				chunkStart = i
				currentWidth = 0
			}
			wrapOppIndex = -1
		}
		currentWidth += g.width
		if i+1 < len(segments) {
			next := segments[i+1]
			nextWs := isEditorWhitespace(next.text)
			if isWs && !nextWs {
				wrapOppIndex, wrapOppWidth = i+1, currentWidth
			} else if !isWs && !nextWs && (isEditorCJK(g.text) || isEditorCJK(next.text)) {
				wrapOppIndex, wrapOppWidth = i+1, currentWidth
			}
		}
	}
	return append(chunks, [2]int{chunkStart, len(segments)})
}

func isEditorWhitespace(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsSpace(r)
}

func isEditorCJK(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) ||
		(r >= 0x3000 && r <= 0x303f) || (r >= 0xff00 && r <= 0xffef)
}

func (m *multilineInput) helpLine() string {
	return ""
}

func (m *multilineInput) insertRune(ke gotui.KeyEvent) {
	m.snapshotUndo()
	runes := []rune(m.text)
	pos := m.clampCursor()
	runes = append(runes[:pos], append([]rune{ke.Rune}, runes[pos:]...)...)
	m.text = string(runes)
	m.cursorPos = pos + 1
	m.notifyEdited()
}

func (m *multilineInput) backspace() {
	runes := []rune(m.text)
	pos := m.clampCursor()
	if pos == 0 || len(runes) == 0 {
		return
	}
	m.snapshotUndo()
	m.yankText = string(runes[pos-1 : pos])
	runes = append(runes[:pos-1], runes[pos:]...)
	m.text = string(runes)
	m.cursorPos = pos - 1
	m.notifyEdited()
}

func (m *multilineInput) delete() {
	runes := []rune(m.text)
	pos := m.clampCursor()
	if pos >= len(runes) {
		return
	}
	m.snapshotUndo()
	m.yankText = string(runes[pos : pos+1])
	runes = append(runes[:pos], runes[pos+1:]...)
	m.text = string(runes)
	m.notifyEdited()
}

func (m *multilineInput) moveLeft() {
	if m.cursorPos > 0 {
		m.cursorPos--
		m.markDirty()
	}
}
func (m *multilineInput) moveRight() {
	if m.cursorPos < utf8.RuneCountInString(m.text) {
		m.cursorPos++
		m.markDirty()
	}
}
func (m *multilineInput) moveHome() { m.cursorPos = 0; m.markDirty() }
func (m *multilineInput) moveEnd()  { m.cursorPos = utf8.RuneCountInString(m.text); m.markDirty() }

func (m *multilineInput) moveWordLeft() {
	runes := []rune(m.text)
	pos := m.clampCursor()
	for pos > 0 && isWordSpace(runes[pos-1]) {
		pos--
	}
	for pos > 0 && !isWordSpace(runes[pos-1]) {
		pos--
	}
	m.cursorPos = pos
	m.markDirty()
}

func (m *multilineInput) moveWordRight() {
	runes := []rune(m.text)
	pos := m.clampCursor()
	for pos < len(runes) && !isWordSpace(runes[pos]) {
		pos++
	}
	for pos < len(runes) && isWordSpace(runes[pos]) {
		pos++
	}
	m.cursorPos = pos
	m.markDirty()
}

func (m *multilineInput) deleteWordBackward() {
	runes := []rune(m.text)
	end := m.clampCursor()
	start := end
	for start > 0 && isWordSpace(runes[start-1]) {
		start--
	}
	for start > 0 && !isWordSpace(runes[start-1]) {
		start--
	}
	if start == end {
		return
	}
	m.snapshotUndo()
	m.yankText = string(runes[start:end])
	m.text = string(append(runes[:start], runes[end:]...))
	m.cursorPos = start
	m.notifyEdited()
}

func (m *multilineInput) deleteWordForward() {
	runes := []rune(m.text)
	start := m.clampCursor()
	end := start
	for end < len(runes) && isWordSpace(runes[end]) {
		end++
	}
	for end < len(runes) && !isWordSpace(runes[end]) {
		end++
	}
	if start == end {
		return
	}
	m.snapshotUndo()
	m.yankText = string(runes[start:end])
	m.text = string(append(runes[:start], runes[end:]...))
	m.notifyEdited()
}

func (m *multilineInput) deleteToLineStart() {
	runes := []rune(m.text)
	pos := m.clampCursor()
	if pos == 0 {
		return
	}
	m.snapshotUndo()
	m.yankText = string(runes[:pos])
	m.text = string(runes[pos:])
	m.cursorPos = 0
	m.notifyEdited()
}

func (m *multilineInput) deleteToLineEnd() {
	runes := []rune(m.text)
	pos := m.clampCursor()
	if pos >= len(runes) {
		return
	}
	m.snapshotUndo()
	m.yankText = string(runes[pos:])
	m.text = string(runes[:pos])
	m.notifyEdited()
}

func isWordSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

func (m *multilineInput) complete() {
	if m.onComplete == nil {
		return
	}
	text, cursor, ok := m.onComplete(m.text, m.clampCursor())
	if !ok {
		return
	}
	m.snapshotUndo()
	m.text = text
	m.cursorPos = cursor
	m.notifyEdited()
}

func (m *multilineInput) enter(ke gotui.KeyEvent) {
	if ke.Mod&gotui.ModShift != 0 {
		if m.onShiftEnter != nil {
			m.onShiftEnter()
			return
		}
		m.insertLiteral('\n')
		return
	}
	if ke.Mod&gotui.ModAlt != 0 && m.onFollowUp != nil {
		m.onFollowUp(m.text)
		return
	}
	if m.onSubmit != nil {
		m.onSubmit(m.text)
	}
}

func (m *multilineInput) insertLiteral(r rune) {
	m.snapshotUndo()
	runes := []rune(m.text)
	pos := m.clampCursor()
	runes = append(runes[:pos], append([]rune{r}, runes[pos:]...)...)
	m.text = string(runes)
	m.cursorPos = pos + 1
	m.notifyEdited()
}

func (m *multilineInput) snapshotUndo() {
	m.undoText = m.text
	m.undoCursor = m.clampCursor()
	m.hasUndo = true
}

func (m *multilineInput) undo() {
	if !m.hasUndo {
		return
	}
	m.text, m.undoText = m.undoText, m.text
	m.cursorPos, m.undoCursor = m.undoCursor, m.clampCursor()
	m.notifyEdited()
}

func (m *multilineInput) yank() {
	if m.yankText == "" {
		return
	}
	m.snapshotUndo()
	runes := []rune(m.text)
	pos := m.clampCursor()
	yankRunes := []rune(m.yankText)
	runes = append(runes[:pos], append(yankRunes, runes[pos:]...)...)
	m.text = string(runes)
	m.cursorPos = pos + len(yankRunes)
	m.notifyEdited()
}

func (m *multilineInput) clampCursor() int {
	count := utf8.RuneCountInString(m.text)
	if m.cursorPos < 0 {
		return 0
	}
	if m.cursorPos > count {
		return count
	}
	return m.cursorPos
}

func (m *multilineInput) notifyEdited() {
	if m.onEdit != nil {
		m.onEdit()
	}
	m.notifyChanged()
}

func (m *multilineInput) notifyChanged() {
	if m.onChange != nil {
		m.onChange(m.text)
	}
	m.markDirty()
}

func (m *multilineInput) markDirty() {
	if m.app != nil {
		m.app.MarkDirty()
	}
}
