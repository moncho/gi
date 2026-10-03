package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	gotui "github.com/grindlemire/go-tui"
)

// Pi's editor paste handling (pi-tui components/editor.js handlePaste):
// bracketed pastes are inserted as one edit with line endings normalised,
// tabs as four spaces and control characters dropped. A paste of more than
// 10 lines or 1000 characters becomes a marker ("[paste #1 +123 lines]",
// "[paste #2 1234 chars]") that the cursor and deletions treat as one unit
// and that is expanded to the pasted text on submit.

var (
	pasteMarkerPattern = regexp.MustCompile(`\[paste #(\d+)( (\+\d+ lines|\d+ chars))?\]`)
	csiUCtrlPattern    = regexp.MustCompile(`\x1b\[(\d+);5u`)
)

// normalizePastedText is Pi's normalizeText plus handlePaste's filtering.
func normalizePastedText(text string) string {
	// tmux popups may re-encode control bytes as CSI-u Ctrl+letter.
	text = csiUCtrlPattern.ReplaceAllStringFunc(text, func(m string) string {
		cp, _ := strconv.Atoi(csiUCtrlPattern.FindStringSubmatch(m)[1])
		switch {
		case cp >= 97 && cp <= 122:
			return string(rune(cp - 96))
		case cp >= 65 && cp <= 90:
			return string(rune(cp - 64))
		}
		return m
	})
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	text = strings.ReplaceAll(text, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r == '\n' || r >= 32 {
			return r
		}
		return -1
	}, text)
}

func isWordRune(r rune) bool {
	return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// paste is Pi's handlePaste.
func (m *multilineInput) paste(raw string) {
	text := normalizePastedText(raw)
	if text == "" {
		return
	}
	m.snapshotUndo()
	runes := []rune(m.text)
	pos := m.clampCursor()
	// A pasted path right after a word character gets a separating space.
	if strings.ContainsAny(text[:1], "/~.") && pos > 0 && isWordRune(runes[pos-1]) {
		text = " " + text
	}
	lines := strings.Count(text, "\n") + 1
	chars := utf8.RuneCountInString(text)
	if lines > 10 || chars > 1000 {
		if m.pastes == nil {
			m.pastes = map[int]string{}
		}
		m.pasteCounter++
		m.pastes[m.pasteCounter] = text
		if lines > 10 {
			text = fmt.Sprintf("[paste #%d +%d lines]", m.pasteCounter, lines)
		} else {
			text = fmt.Sprintf("[paste #%d %d chars]", m.pasteCounter, chars)
		}
	}
	inserted := []rune(text)
	m.text = string(append(runes[:pos], append(inserted, runes[pos:]...)...))
	m.cursorPos = pos + len(inserted)
	m.notifyEdited()
}

// ExpandedText is the text with paste markers replaced by their content
// (Pi's getExpandedText); it is what gets submitted.
func (m *multilineInput) ExpandedText() string {
	if len(m.pastes) == 0 {
		return m.text
	}
	return pasteMarkerPattern.ReplaceAllStringFunc(m.text, func(marker string) string {
		id, _ := strconv.Atoi(pasteMarkerPattern.FindStringSubmatch(marker)[1])
		if content, ok := m.pastes[id]; ok {
			return content
		}
		return marker
	})
}

// expandedCursor maps the cursor into ExpandedText.
func (m *multilineInput) expandedCursor() int {
	pos := m.clampCursor()
	if len(m.pastes) == 0 {
		return pos
	}
	prefix := string([]rune(m.text)[:pos])
	expanded := (&multilineInput{text: prefix, pastes: m.pastes}).ExpandedText()
	return utf8.RuneCountInString(expanded)
}

// markerSpans are the rune spans of valid paste markers.
func (m *multilineInput) markerSpans() [][2]int {
	if len(m.pastes) == 0 {
		return nil
	}
	var spans [][2]int
	for _, loc := range pasteMarkerPattern.FindAllStringSubmatchIndex(m.text, -1) {
		id, _ := strconv.Atoi(m.text[loc[2]:loc[3]])
		if _, ok := m.pastes[id]; !ok {
			continue
		}
		start := utf8.RuneCountInString(m.text[:loc[0]])
		spans = append(spans, [2]int{start, start + utf8.RuneCountInString(m.text[loc[0]:loc[1]])})
	}
	return spans
}

// markerEndingAt / markerStartingAt find a marker adjacent to the cursor.
func (m *multilineInput) markerEndingAt(pos int) (int, bool) {
	for _, s := range m.markerSpans() {
		if s[1] == pos {
			return s[0], true
		}
	}
	return 0, false
}

func (m *multilineInput) markerStartingAt(pos int) (int, bool) {
	for _, s := range m.markerSpans() {
		if s[0] == pos {
			return s[1], true
		}
	}
	return 0, false
}

// removePaste drops a deleted marker's paste and renumbers the higher ones,
// in the registry and in the text, as Pi's handleBackspace does.
func (m *multilineInput) removePaste(marker string) {
	sub := pasteMarkerPattern.FindStringSubmatch(marker)
	if sub == nil {
		return
	}
	target, _ := strconv.Atoi(sub[1])
	if _, ok := m.pastes[target]; !ok {
		return
	}
	delete(m.pastes, target)
	m.pasteCounter--
	renumbered := map[int]string{}
	for id, content := range m.pastes {
		if id > target {
			id--
		}
		renumbered[id] = content
	}
	m.pastes = renumbered
	m.text = pasteMarkerPattern.ReplaceAllStringFunc(m.text, func(s string) string {
		g := pasteMarkerPattern.FindStringSubmatch(s)
		id, _ := strconv.Atoi(g[1])
		if id <= target {
			return s
		}
		return fmt.Sprintf("[paste #%d%s]", id-1, g[2])
	})
}

// clearPastes forgets pasted content (new text, submit).
func (m *multilineInput) clearPastes() {
	m.pastes, m.pasteCounter = nil, 0
}

func copyPastes(p map[int]string) map[int]string {
	if p == nil {
		return nil
	}
	out := make(map[int]string, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}

// handlePaste routes a bracketed paste to the editor when it has focus;
// elsewhere (menus, selectors) the paste replays as keystrokes.
func (c *chatTUI) handlePaste(e gotui.PasteEvent) bool {
	if c.input == nil || c.input.suspended || !c.input.focused {
		return false
	}
	c.input.paste(e.Text)
	return true
}
