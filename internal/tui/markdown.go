package tui

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"

	gotui "github.com/grindlemire/go-tui"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

var tuiMarkdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

const (
	markdownInlineCodeStart = "\x00gi-code-start\x00"
	markdownInlineCodeEnd   = "\x00gi-code-end\x00"
)

func looksLikeMarkdown(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	markers := []string{"# ", "## ", "### ", "- ", "* ", "1. ", "```", "|", "> ", "**", "__", "`"}
	for _, marker := range markers {
		if strings.Contains(trimmed, marker) {
			return true
		}
	}
	return strings.Contains(trimmed, "\n")
}

// Table-bearing messages retain source in an invisible transcript metadata row
// so terminal resize can allocate columns again instead of wrapping old borders.
func renderChatMarkdown(role, prefix, markdown string, width int) []string {
	if (role != "user" && role != "assistant") || !strings.Contains(markdown, "|") {
		return renderMarkdownTranscript(prefix, markdown, width)
	}
	source := []byte(markdown)
	root := tuiMarkdown.Parser().Parse(text.NewReader(source))
	hasTable := false
	_ = gast.Walk(root, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if _, ok := n.(*extast.Table); ok && entering {
			hasTable = true
			return gast.WalkStop, nil
		}
		return gast.WalkContinue, nil
	})
	if !hasTable {
		return projectMarkdownTranscript(prefix, source, root, width)
	}
	body := projectMarkdownTranscript("", source, root, max(1, width-1))
	out := []string{encodeTranscriptBlockMarker(transcriptBlockMeta{Key: "markdown-table", Kind: role, MarkdownSource: markdown})}
	for _, line := range body {
		out = append(out, "│ "+line)
	}
	return out
}

func renderMarkdownTranscript(prefix, markdown string, width int) []string {
	source := []byte(markdown)
	root := tuiMarkdown.Parser().Parse(text.NewReader(source))
	return projectMarkdownTranscript(prefix, source, root, width)
}

func projectMarkdownTranscript(prefix string, source []byte, root gast.Node, width int) []string {
	contentWidth := max(1, width-utf8.RuneCountInString(prefix))
	renderer := &markdownProjector{source: source, width: contentWidth}
	body := renderer.renderBlocks(root, 0)
	if len(body) == 0 {
		body = []string{""}
	}
	indent := strings.Repeat(" ", utf8.RuneCountInString(prefix))
	out := make([]string, 0, len(body))
	for i, line := range body {
		if i == 0 {
			out = append(out, prefix+line)
		} else {
			out = append(out, indent+line)
		}
	}
	return out
}

type markdownProjector struct {
	source []byte
	width  int
}

func (m *markdownProjector) renderBlocks(node gast.Node, depth int) []string {
	var lines []string
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch n := child.(type) {
		case *gast.Heading:
			text := strings.TrimSpace(m.renderInlineChildren(n))
			if text == "" {
				continue
			}
			lines = append(lines, mdStyled("heading", upperOutsideMarkers(text)))
			underline := strings.Repeat("=", min(max(markdownRenderedWidth(text), 3), m.width))
			if n.Level > 1 {
				underline = strings.Repeat("-", min(max(markdownRenderedWidth(text), 3), m.width))
			}
			lines = append(lines, mdStyled("heading", underline), "")
		case *gast.Paragraph:
			text := strings.TrimSpace(m.renderInlineChildren(n))
			if text != "" {
				lines = append(lines, wrapParagraph(text, m.width)...)
				lines = append(lines, "")
			}
		case *gast.TextBlock:
			text := strings.TrimSpace(m.renderInlineChildren(n))
			if text != "" {
				lines = append(lines, wrapParagraph(text, m.width)...)
			}
		case *gast.FencedCodeBlock:
			lang := strings.TrimSpace(string(n.Language(m.source)))
			lines = append(lines, m.renderCodeBlock(n.Lines(), lang)...)
		case *gast.CodeBlock:
			lines = append(lines, m.renderCodeBlock(n.Lines(), "")...)
		case *gast.Blockquote:
			quoted := m.renderBlocks(n, depth+1)
			for _, line := range quoted {
				if line == "" {
					lines = append(lines, "")
					continue
				}
				lines = append(lines, styleLinePrefix(wrapWithPrefix(mdStyled("quote", line), m.width, "> "), "> ", "quoteborder")...)
			}
			lines = append(lines, "")
		case *gast.List:
			itemIndex := n.Start
			if itemIndex == 0 {
				itemIndex = 1
			}
			for item := n.FirstChild(); item != nil; item = item.NextSibling() {
				itemLines := m.renderBlocks(item, depth+1)
				itemLines = trimBlankEdges(itemLines)
				if len(itemLines) == 0 {
					continue
				}
				prefix := "• "
				if n.IsOrdered() {
					prefix = strconv.Itoa(itemIndex) + ". "
					itemIndex++
				}
				lines = append(lines, styleLinePrefix(wrapWithPrefix(itemLines[0], m.width, prefix), prefix, "bullet")...)
				contPrefix := strings.Repeat(" ", utf8.RuneCountInString(prefix))
				for _, extra := range itemLines[1:] {
					if extra == "" {
						continue
					}
					lines = append(lines, wrapWithPrefix(extra, m.width, contPrefix)...)
				}
			}
			lines = append(lines, "")
		case *extast.Table:
			lines = append(lines, m.renderTable(n)...)
			lines = append(lines, "")
		case *gast.ThematicBreak:
			lines = append(lines, mdStyled("hr", strings.Repeat("-", min(max(10, m.width/2), m.width))), "")
		default:
			if child.HasChildren() {
				lines = append(lines, m.renderBlocks(child, depth+1)...)
			}
		}
	}
	return trimBlankEdges(lines)
}

func (m *markdownProjector) renderTable(table *extast.Table) []string {
	var headers []string
	var rows [][]string
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		var cells []string
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cells = append(cells, strings.TrimSpace(m.renderInlineChildren(cell)))
		}
		if len(cells) == 0 {
			continue
		}
		if row.Kind() == extast.KindTableHeader {
			headers = cells
			continue
		}
		rows = append(rows, cells)
	}
	if len(headers) == 0 && len(rows) > 0 {
		headers = make([]string, len(rows[0]))
		for i := range headers {
			headers[i] = "Col" + strconv.Itoa(i+1)
		}
	}
	return m.renderTableGrid(headers, rows)
}

func (m *markdownProjector) renderInlineChildren(node gast.Node) string {
	var b strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		b.WriteString(m.renderInline(child))
	}
	return b.String()
}

func (m *markdownProjector) renderInline(node gast.Node) string {
	switch n := node.(type) {
	case *gast.Text:
		text := string(n.Text(m.source))
		if n.HardLineBreak() {
			return text + "\n"
		}
		if n.SoftLineBreak() {
			return text + " "
		}
		return text
	case *gast.String:
		return string(n.Value)
	case *gast.CodeSpan:
		return markdownInlineCodeStart + m.renderInlineChildren(n) + markdownInlineCodeEnd
	case *gast.Emphasis:
		if n.Level >= 2 {
			return mdStyled("bold", m.renderInlineChildren(n))
		}
		return mdStyled("italic", m.renderInlineChildren(n))
	case *extast.Strikethrough:
		return mdStyled("strike", m.renderInlineChildren(n))
	case *extast.TaskCheckBox:
		if n.IsChecked {
			return "☑ "
		}
		return "☐ "
	case *gast.Link:
		label := strings.TrimSpace(m.renderInlineChildren(n))
		dest := string(n.Destination)
		if label == "" || label == dest {
			return mdStyled("link", dest)
		}
		return mdStyled("link", label) + " " + mdStyled("url", "("+dest+")")
	case *gast.AutoLink:
		return mdStyled("link", string(n.URL(m.source)))
	default:
		return m.renderInlineChildren(node)
	}
}

func wrapParagraph(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		words := markdownParagraphTokens(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := words[0]
		currentWidth := markdownRenderedWidth(current)
		for _, word := range words[1:] {
			wordWidth := markdownRenderedWidth(word)
			if currentWidth+1+wordWidth <= width {
				current += " " + word
				currentWidth += 1 + wordWidth
				continue
			}
			lines = append(lines, current)
			// Keep a long source token intact so go-tui owns its soft wraps.
			// Pre-splitting it here makes cross-row transcript search lose
			// the source paragraph boundary.
			current = word
			currentWidth = wordWidth
		}
		lines = append(lines, current)
	}
	return balanceStyleMarkers(lines)
}

// markdownParagraphTokens keeps inline-code spans and their adjacent punctuation
// in the same word. A style boundary is not a whitespace boundary: adding a
// separator at `code`, or between `code` and punctuation, changes the message.
func markdownParagraphTokens(text string) []string {
	var tokens []string
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			tokens = append(tokens, word.String())
			word.Reset()
		}
	}
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], markdownInlineCodeStart) {
			end := strings.Index(text[i+len(markdownInlineCodeStart):], markdownInlineCodeEnd)
			if end >= 0 {
				endPos := i + len(markdownInlineCodeStart) + end + len(markdownInlineCodeEnd)
				word.WriteString(text[i:endPos])
				i = endPos
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		if isMarkdownTokenSpace(r) {
			flush()
		} else {
			word.WriteString(text[i : i+size])
		}
		i += size
	}
	flush()
	return tokens
}

func isMarkdownTokenSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func markdownRenderedWidth(s string) int {
	return utf8.RuneCountInString(stripMarkdownInlineStyleMarkers(s))
}

func stripMarkdownInlineStyleMarkers(s string) string {
	if !strings.Contains(s, "\x00gi-") {
		return s
	}
	return markdownMarkerPattern.ReplaceAllString(s, "")
}

// styleCodeLines marks fenced/indented code lines (after their indent).
func styleCodeLines(lines []string) []string {
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" {
			continue
		}
		lines[i] = line[:len(line)-len(trimmed)] + mdStyled("codeblock", trimmed)
	}
	return lines
}

// styleLinePrefix colours the list bullet or quote marker on the first line
// (or every line, for quotes) produced by wrapWithPrefix.
func styleLinePrefix(lines []string, prefix, style string) []string {
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) && (i == 0 || style == "quoteborder") {
			mark := strings.TrimRight(prefix, " ")
			lines[i] = mdStyled(style, mark) + line[len(mark):]
		}
	}
	return lines
}

func wrapWithPrefix(text string, width int, prefix string) []string {
	prefixWidth := utf8.RuneCountInString(prefix)
	contentWidth := width - prefixWidth
	if contentWidth < 8 {
		contentWidth = 8
	}
	wrapped := wrapParagraph(text, contentWidth)
	out := make([]string, 0, len(wrapped))
	for i, line := range wrapped {
		if i == 0 {
			out = append(out, prefix+line)
		} else {
			out = append(out, strings.Repeat(" ", prefixWidth)+line)
		}
	}
	return out
}

func wrapPreformattedWithPrefix(text string, width int, prefix string) []string {
	prefixWidth := markdownRenderedWidth(prefix)
	contentWidth := max(2, width-prefixWidth)
	if text == "" {
		return []string{prefix}
	}
	var out []string
	var line strings.Builder
	used := 0
	for text != "" {
		cluster, cells, size := gotui.NextCluster(text)
		if size == 0 {
			break
		}
		text = text[size:]
		if used > 0 && used+cells > contentWidth {
			out = append(out, prefix+line.String())
			prefix = strings.Repeat(" ", prefixWidth)
			line.Reset()
			used = 0
		}
		line.WriteString(cluster)
		used += cells
	}
	return append(out, prefix+line.String())
}

func wrapLongRunes(word string, width int) []string {
	runes := []rune(word)
	if len(runes) == 0 {
		return []string{""}
	}
	var out []string
	for len(runes) > width {
		out = append(out, string(runes[:width]))
		runes = runes[width:]
	}
	out = append(out, string(runes))
	return out
}

func trimBlankEdges(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return nil
	}
	trimmed := append([]string(nil), lines[start:end]...)
	// collapse repeated blank lines inside the slice
	var out []string
	blank := false
	for _, line := range trimmed {
		isBlank := strings.TrimSpace(line) == ""
		if isBlank && blank {
			continue
		}
		out = append(out, line)
		blank = isBlank
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func markdownToPlain(text string) string {
	var buf bytes.Buffer
	if err := tuiMarkdown.Convert([]byte(text), &buf); err != nil {
		return text
	}
	return buf.String()
}

// piCodeBlockIndent is pi-tui Markdown's default codeBlockIndent.
const piCodeBlockIndent = "  "

// renderCodeBlock follows pi-tui Markdown: a ```lang border, the code
// indented and syntax highlighted when the language is known (plain
// mdCodeBlock otherwise), a closing ``` border, then a blank line.
func (m *markdownProjector) renderCodeBlock(segments *text.Segments, lang string) []string {
	var src strings.Builder
	for i := 0; i < segments.Len(); i++ {
		segment := segments.At(i)
		src.WriteString(strings.TrimRight(string(segment.Value(m.source)), "\r\n"))
		if i < segments.Len()-1 {
			src.WriteByte('\n')
		}
	}
	code := src.String()
	lines := []string{mdStyled("codeborder", "```"+lang)}
	if highlighted, ok := highlightCodeLines(code, lang); ok {
		for _, segs := range highlighted {
			lines = append(lines, wrapHighlightedLine(segs, m.width, piCodeBlockIndent)...)
		}
	} else {
		for _, codeLine := range strings.Split(code, "\n") {
			if codeLine == "" {
				lines = append(lines, piCodeBlockIndent)
				continue
			}
			lines = append(lines, styleCodeLines(wrapPreformattedWithPrefix(codeLine, m.width, piCodeBlockIndent))...)
		}
	}
	return append(lines, mdStyled("codeborder", "```"), "")
}
