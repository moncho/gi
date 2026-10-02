package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	gotui "github.com/grindlemire/go-tui"

	"github.com/rcarmo/gi/internal/textdiff"
	"github.com/rcarmo/gi/internal/tools"
)

// Pi's read, write and edit renderers (core/tools/renderers/{read,write,
// edit}.js, modes/interactive/components/diff.js) and the word wrapping of
// pi-tui's Text (wrapTextWithAnsi), which lays their rows out.

// styledCell is one grapheme cluster with its style.
type styledCell struct {
	text  string
	width int
	style gotui.Style
}

func spansToCells(spans []gotui.TextSpan) []styledCell {
	var cells []styledCell
	for _, span := range spans {
		for text := span.Text; text != ""; {
			cluster, width, size := gotui.NextCluster(text)
			if size == 0 {
				break
			}
			text = text[size:]
			cells = append(cells, styledCell{cluster, width, span.Style})
		}
	}
	return cells
}

func cellsToSpans(cells []styledCell) []gotui.TextSpan {
	var spans []gotui.TextSpan
	for _, c := range cells {
		if n := len(spans); n > 0 && spans[n-1].Style == c.style {
			spans[n-1].Text += c.text
		} else {
			spans = append(spans, gotui.TextSpan{Text: c.text, Style: c.style})
		}
	}
	return spans
}

func isJSWhitespace(s string) bool {
	for _, r := range s {
		if !(unicode.IsSpace(r) || r == 0xfeff) {
			return false
		}
	}
	return true
}

func isCJKBreak(cluster string) bool {
	for _, r := range cluster {
		return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Bopomofo)
	}
	return false
}

func cellsWidth(cells []styledCell) int {
	n := 0
	for _, c := range cells {
		n += c.width
	}
	return n
}

func trimEndCells(cells []styledCell) []styledCell {
	for len(cells) > 0 && isJSWhitespace(cells[len(cells)-1].text) {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// piWrapLine is pi-tui's wrapSingleLine on styled text: a line that fits is
// kept as is; otherwise it breaks between space runs and words (each CJK
// character a word of its own), never starts a line with the whitespace it
// broke at, splits words wider than the line by grapheme, and trims each
// wrapped line's end.
func piWrapLine(spans []gotui.TextSpan, width int) [][]gotui.TextSpan {
	width = max(1, width)
	cells := spansToCells(spans)
	if cellsWidth(cells) <= width {
		return [][]gotui.TextSpan{spans}
	}
	for i := range cells {
		// A cluster wider than the line cannot be shown (pi-tui would
		// overflow the terminal): stand in a replacement character.
		if cells[i].width > width {
			cells[i].text, cells[i].width = "\uFFFD", 1
		}
	}
	var tokens [][]styledCell
	var current []styledCell
	kind := ""
	for _, c := range cells {
		if c.text != " " && isCJKBreak(c.text) {
			if len(current) > 0 {
				tokens, current = append(tokens, current), nil
			}
			tokens = append(tokens, []styledCell{c})
			kind = ""
			continue
		}
		k := "word"
		if c.text == " " {
			k = "space"
		}
		if len(current) > 0 && kind != k {
			tokens, current = append(tokens, current), nil
		}
		kind = k
		current = append(current, c)
	}
	if len(current) > 0 {
		tokens = append(tokens, current)
	}
	var lines [][]styledCell
	var line []styledCell
	lineWidth := 0
	for _, token := range tokens {
		tokenWidth := cellsWidth(token)
		whitespace := true
		for _, c := range token {
			if !isJSWhitespace(c.text) {
				whitespace = false
				break
			}
		}
		if tokenWidth > width && !whitespace {
			if len(line) > 0 {
				lines = append(lines, line)
				line, lineWidth = nil, 0
			}
			var piece []styledCell
			pieceWidth := 0
			for _, c := range token {
				if pieceWidth+c.width > width {
					lines = append(lines, piece)
					piece, pieceWidth = nil, 0
				}
				piece = append(piece, c)
				pieceWidth += c.width
			}
			line, lineWidth = piece, pieceWidth
			continue
		}
		if lineWidth+tokenWidth > width && lineWidth > 0 {
			lines = append(lines, trimEndCells(line))
			if whitespace {
				line, lineWidth = nil, 0
			} else {
				line, lineWidth = append([]styledCell(nil), token...), tokenWidth
			}
			continue
		}
		line = append(line, token...)
		lineWidth += tokenWidth
	}
	if len(line) > 0 {
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return [][]gotui.TextSpan{nil}
	}
	out := make([][]gotui.TextSpan, len(lines))
	for i, l := range lines {
		out[i] = cellsToSpans(trimEndCells(l))
	}
	return out
}

// piTextRows lays out one logical line as Pi's Text would, one row per
// wrapped line.
func piTextRows(spans []gotui.TextSpan, width int) []*gotui.Element {
	var rows []*gotui.Element
	for _, line := range piWrapLine(spans, width) {
		var linked []gotui.TextSpan
		for _, span := range line {
			linked = append(linked, transcriptLinkSpans(span.Text, span.Style)...)
		}
		rows = append(rows, gotui.New(gotui.WithWidthPercent(100), gotui.WithHeight(1), gotui.WithWrap(false), gotui.WithRichText(linked...)))
	}
	return rows
}

// shortenToolPath is Pi's shortenPath: the home directory becomes "~".
func shortenToolPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func numericArg(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i, true
		}
	}
	return 0, false
}

// readLineRange is Pi's formatReadLineRange (":start-end", ":start").
func readLineRange(args map[string]any) string {
	offset, hasOffset := numericArg(args["offset"])
	limit, hasLimit := numericArg(args["limit"])
	if !hasOffset && !hasLimit {
		return ""
	}
	start := 1
	if hasOffset {
		start = offset
	}
	if hasLimit && start+limit-1 != 0 {
		return fmt.Sprintf(":%d-%d", start, start+limit-1)
	}
	return fmt.Sprintf(":%d", start)
}

var compactReadResourceNames = map[string]bool{"AGENTS.override.md": true, "AGENTS.md": true, "AGENTS.MD": true, "CLAUDE.md": true, "CLAUDE.MD": true}

// compactReadClassification is Pi's getCompactReadClassification. Pi's own
// docs become gi's shipped reference tree (vfs://reference/...).
func compactReadClassification(rawPath, workspaceRoot string) (kind, label string) {
	if rawPath == "" {
		return "", ""
	}
	if rest, ok := strings.CutPrefix(rawPath, "vfs://reference/"); ok {
		return "docs", rest
	}
	if strings.Contains(rawPath, "://") {
		return "", ""
	}
	abs := rawPath
	if strings.HasPrefix(abs, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			abs = filepath.Join(home, abs[2:])
		}
	}
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(workspaceRoot, abs)
	}
	abs = filepath.Clean(abs)
	name := filepath.Base(abs)
	if name == "SKILL.md" {
		label := filepath.Base(filepath.Dir(abs))
		if label == "" || label == "." || label == string(filepath.Separator) {
			label = name
		}
		return "skill", label
	}
	if compactReadResourceNames[name] {
		label := abs
		if rel, err := filepath.Rel(filepath.Clean(workspaceRoot), abs); err == nil && filepath.IsLocal(rel) {
			label = rel
		}
		return "resource", filepath.ToSlash(label)
	}
	return "", ""
}

// fileToolCallSpans is the call line of Pi's read, write and edit
// renderers: compact for skills, resources and docs while collapsed.
func (c *chatTUI) fileToolCallSpans(block transcriptRenderableBlock) []gotui.TextSpan {
	title := piFg(piToolTitle).Bold()
	rangeSpan := gotui.TextSpan{Text: block.ToolRange, Style: piFg(piWarning)}
	if block.Header == "read" && !block.Expanded {
		if kind, label := compactReadClassification(block.ToolPath, c.cfg.WorkspaceRoot); kind != "" {
			hint := gotui.TextSpan{Text: " (" + piExpandKey + " to expand)", Style: piFg(piDim)}
			if kind == "skill" {
				return []gotui.TextSpan{{Text: "[skill] ", Style: piFg(piCustomLabel).Bold()}, {Text: label, Style: piFg(piCustomText)}, rangeSpan, hint}
			}
			return []gotui.TextSpan{{Text: "read " + kind, Style: title}, {Text: " "}, {Text: label, Style: piFg(piAccent)}, rangeSpan, hint}
		}
	}
	path := gotui.TextSpan{Text: "...", Style: piFg(piToolOutput)}
	if raw := block.ToolPath; raw != "" {
		path = gotui.TextSpan{Text: shortenToolPath(raw), Style: piFg(piAccent)}
	} else if block.ToolArg != "" { // no path argument recorded: the call's text
		path = gotui.TextSpan{Text: block.ToolArg, Style: piFg(piAccent)}
	}
	spans := []gotui.TextSpan{{Text: block.Header, Style: title}, {Text: " "}, path}
	if block.Header == "read" {
		spans = append(spans, rangeSpan)
	}
	return spans
}

// readTruncationNotice is the warning Pi's read renderer adds from
// details.truncation.
func readTruncationNotice(details any) string {
	d, _ := details.(map[string]any)
	t, _ := d["truncation"].(map[string]any)
	if truncated, _ := t["truncated"].(bool); !truncated {
		return ""
	}
	maxBytes, ok := numericArg(t["maxBytes"])
	if !ok {
		maxBytes = tools.ReadMaxBytes
	}
	maxLines, ok := numericArg(t["maxLines"])
	if !ok {
		maxLines = tools.ReadMaxLines
	}
	outputLines, _ := numericArg(t["outputLines"])
	totalLines, _ := numericArg(t["totalLines"])
	switch {
	case t["firstLineExceedsLimit"] == true:
		return fmt.Sprintf("[First line exceeds %s limit]", tools.FormatSize(maxBytes))
	case t["truncatedBy"] == "lines":
		return fmt.Sprintf("[Truncated: showing %d of %d lines (%d line limit)]", outputLines, totalLines, maxLines)
	}
	return fmt.Sprintf("[Truncated: %d lines shown (%s limit)]", outputLines, tools.FormatSize(maxBytes))
}

// setFileToolDetails records what Pi's renderers take from a result.
func setFileToolDetails(meta *transcriptBlockMeta, details any, resultText string) {
	switch meta.Title {
	case "read":
		meta.ToolNotice = readTruncationNotice(details)
	case "edit":
		d, _ := details.(map[string]any)
		if diff, ok := d["diff"].(string); ok && meta.Status == "ok" {
			meta.EditDiff, meta.EditError = &diff, ""
		} else if meta.Status != "ok" && meta.EditDiff == nil && meta.EditError == "" {
			// No live preview (e.g. a resumed session): Pi recomputes it from
			// the file, which yields the same error the edit failed with.
			meta.EditError = resultText
		}
	}
}

// setEditPreview computes Pi's edit preview once the call's arguments are
// complete: the diff the edit would make, or the error it would fail with.
func (c *chatTUI) setEditPreview(meta *transcriptBlockMeta, args any) {
	if meta.Title != "edit" || meta.EditDiff != nil || meta.EditError != "" {
		return
	}
	m, _ := args.(map[string]any)
	path, edits, ok := tools.EditArguments(m)
	if !ok || path == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	diff, _, err := tools.EditPreview(ctx, c.cfg.WorkspaceRoot, c.store, path, edits)
	if err != nil {
		meta.EditError = err.Error()
		return
	}
	meta.EditDiff = &diff
}

var diffLinePattern = regexp.MustCompile(`^([+\-\s])(\s*\d*)\s(.*)$`)

func replaceTabs(s string) string { return strings.ReplaceAll(s, "\t", "   ") }

// intraLineDiff is Pi's renderIntraLineDiff: word changes in inverse, the
// first change's leading whitespace left plain.
func intraLineDiff(oldContent, newContent string, removed, added gotui.Style) (oldSpans, newSpans []gotui.TextSpan) {
	firstRemoved, firstAdded := true, true
	changed := func(value string, first *bool, base gotui.Style, out *[]gotui.TextSpan) {
		if *first {
			lead := value[:len(value)-len(strings.TrimLeftFunc(value, unicode.IsSpace))]
			*out = append(*out, gotui.TextSpan{Text: lead, Style: base})
			value = value[len(lead):]
			*first = false
		}
		if value != "" {
			*out = append(*out, gotui.TextSpan{Text: value, Style: base.Reverse()})
		}
	}
	for _, part := range textdiff.Words(oldContent, newContent) {
		switch {
		case part.Removed:
			changed(part.Value, &firstRemoved, removed, &oldSpans)
		case part.Added:
			changed(part.Value, &firstAdded, added, &newSpans)
		default:
			oldSpans = append(oldSpans, gotui.TextSpan{Text: part.Value, Style: removed})
			newSpans = append(newSpans, gotui.TextSpan{Text: part.Value, Style: added})
		}
	}
	return oldSpans, newSpans
}

// renderDiffLines is Pi's renderDiff: one span list per diff line.
func renderDiffLines(diff string) [][]gotui.TextSpan {
	removedStyle, addedStyle, contextStyle := piFg(piDiffRemoved), piFg(piDiffAdded), piFg(piDiffContext)
	lines := strings.Split(diff, "\n")
	var out [][]gotui.TextSpan
	type numbered struct{ num, content string }
	for i := 0; i < len(lines); {
		m := diffLinePattern.FindStringSubmatch(lines[i])
		if m == nil {
			out = append(out, []gotui.TextSpan{{Text: lines[i], Style: contextStyle}})
			i++
			continue
		}
		switch m[1] {
		case "-":
			var removed, added []numbered
			for i < len(lines) {
				p := diffLinePattern.FindStringSubmatch(lines[i])
				if p == nil || p[1] != "-" {
					break
				}
				removed = append(removed, numbered{p[2], p[3]})
				i++
			}
			for i < len(lines) {
				p := diffLinePattern.FindStringSubmatch(lines[i])
				if p == nil || p[1] != "+" {
					break
				}
				added = append(added, numbered{p[2], p[3]})
				i++
			}
			if len(removed) == 1 && len(added) == 1 {
				oldSpans, newSpans := intraLineDiff(replaceTabs(removed[0].content), replaceTabs(added[0].content), removedStyle, addedStyle)
				out = append(out, append([]gotui.TextSpan{{Text: "-" + removed[0].num + " ", Style: removedStyle}}, oldSpans...))
				out = append(out, append([]gotui.TextSpan{{Text: "+" + added[0].num + " ", Style: addedStyle}}, newSpans...))
				continue
			}
			for _, r := range removed {
				out = append(out, []gotui.TextSpan{{Text: "-" + r.num + " " + replaceTabs(r.content), Style: removedStyle}})
			}
			for _, a := range added {
				out = append(out, []gotui.TextSpan{{Text: "+" + a.num + " " + replaceTabs(a.content), Style: addedStyle}})
			}
		case "+":
			out = append(out, []gotui.TextSpan{{Text: "+" + m[2] + " " + replaceTabs(m[3]), Style: addedStyle}})
			i++
		default:
			out = append(out, []gotui.TextSpan{{Text: " " + m[2] + " " + replaceTabs(m[3]), Style: contextStyle}})
			i++
		}
	}
	return out
}

// editBandStatus is Pi's getEditHeaderBg: a preview decides the colour
// (diff: success, error: error); without one, the settled status does.
func editBandStatus(block transcriptRenderableBlock) string {
	switch {
	case block.EditDiff != nil:
		return "ok"
	case block.EditError != "":
		return "error"
	case block.Status == "error" || block.Status == "failed" || block.Status == "skipped":
		return "error"
	}
	return "running"
}

// renderEditBlock fills Pi's edit box: the call, then the diff (or the
// preview's error) after a blank line.
func (c *chatTUI) renderEditBlock(block transcriptRenderableBlock, container *gotui.Element) {
	width := c.transcriptBlockContentWidth("tool")
	switch {
	case block.EditDiff != nil:
		container.AddChild(blankRow())
		for _, line := range renderDiffLines(*block.EditDiff) {
			for _, row := range piTextRows(line, width) {
				container.AddChild(row)
			}
		}
	case block.EditError != "":
		container.AddChild(blankRow())
		for _, line := range strings.Split(block.EditError, "\n") {
			for _, row := range piTextRows([]gotui.TextSpan{{Text: line, Style: piFg(piError)}}, width) {
				container.AddChild(row)
			}
		}
	}
}

// editResultBelow is Pi's edit renderResult: an error the preview did not
// already show, below the box, outside its background.
func (c *chatTUI) editResultBelow(block transcriptRenderableBlock) *gotui.Element {
	if block.Header != "edit" || block.Kind != "tool" || block.Status != "error" && block.Status != "failed" && block.Status != "skipped" {
		return nil
	}
	text := strings.TrimSpace(strings.Join(block.Body, "\n"))
	if text == "" || text == block.EditError {
		return nil
	}
	below := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100), gotui.WithPaddingTRBL(0, 1, 0, 1))
	below.AddChild(blankRow())
	for _, line := range strings.Split(text, "\n") {
		for _, row := range piTextRows([]gotui.TextSpan{{Text: line, Style: piFg(piError)}}, max(1, c.currentContentWidth()-2)) {
			below.AddChild(row)
		}
	}
	return below
}
