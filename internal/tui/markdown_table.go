package tui

import (
	"strings"

	gotui "github.com/grindlemire/go-tui"
)

// Match Pi's width-aware table allocation. Widths are terminal columns, never
// rune counts or the byte length of our private inline-style delimiters.
func tableColumnWidths(headers []string, rows [][]string, width int) []int {
	n := len(headers)
	available := width - (3*n + 1)
	if n == 0 || available < n {
		return nil
	}
	natural, minimum := make([]int, n), make([]int, n)
	measure := func(cells []string) {
		for i := 0; i < n && i < len(cells); i++ {
			plain := stripMarkdownInlineStyleMarkers(cells[i])
			natural[i] = max(natural[i], gotui.StringWidth(plain))
			for _, word := range strings.Fields(plain) {
				minimum[i] = max(minimum[i], min(30, gotui.StringWidth(word)))
			}
		}
	}
	measure(headers)
	for _, row := range rows {
		measure(row)
	}
	sumNatural, sumMin := 0, 0
	for i := range natural {
		natural[i] = max(1, natural[i])
		minimum[i] = max(1, minimum[i])
		sumNatural += natural[i]
		sumMin += minimum[i]
	}
	if sumNatural <= available {
		return natural
	}
	widths := make([]int, n)
	if sumMin >= available {
		scale := float64(available) / float64(sumMin)
		for i := range widths {
			widths[i] = max(1, int(float64(minimum[i])*scale))
		}
	} else {
		remaining := available - sumMin
		flex := 0
		for i := range widths {
			flex += natural[i] - minimum[i]
		}
		for i := range widths {
			widths[i] = minimum[i]
			if flex > 0 {
				widths[i] += int(float64(natural[i]-minimum[i]) / float64(flex) * float64(remaining))
			}
		}
	}
	used := 0
	for _, w := range widths {
		used += w
	}
	// Very narrow tables may have more minimum-one columns than the floored
	// proportional allocation expects. Preserve one per cell while fitting.
	for used > available {
		for i := len(widths) - 1; i >= 0 && used > available; i-- {
			if widths[i] > 1 {
				widths[i]--
				used--
			}
		}
	}
	for i := 0; used < available; i = (i + 1) % n {
		widths[i]++
		used++
	}
	return widths
}

type tableGlyph struct {
	text  string
	width int
	code  bool
}

func wrapTableCell(text string, width int) []string {
	var glyphs []tableGlyph
	for _, segment := range parseTUIInlineSegments(text) {
		// Use the renderer's cluster boundaries as well as its cell widths.
		// UAX #29 and go-tui disagree for some ZWJ sequences; measuring
		// separately segmented glyphs can shift every following grid border.
		for rest := segment.Text; rest != ""; {
			g, width, size := gotui.NextCluster(rest)
			if size == 0 {
				break
			}
			glyphs = append(glyphs, tableGlyph{g, width, segment.Code})
			rest = rest[size:]
		}
	}
	if len(glyphs) == 0 {
		return []string{""}
	}
	var lines []string
	for len(glyphs) > 0 {
		used, end, lastSpace := 0, 0, -1
		for end < len(glyphs) && used+glyphs[end].width <= width {
			if glyphs[end].text == " " {
				lastSpace = end
			}
			used += glyphs[end].width
			end++
		}
		if end == 0 { // A two-column grapheme cannot fit a one-column cell.
			lines = append(lines, "�")
			glyphs = glyphs[1:]
			continue
		}
		cut, next := end, end
		if end < len(glyphs) && glyphs[end].text == " " {
			next = end + 1 // exact-fitting words need not be shortened at an earlier space
		} else if end < len(glyphs) && lastSpace > 0 {
			cut, next = lastSpace, lastSpace+1
		}
		var b strings.Builder
		inCode := false
		for _, g := range glyphs[:cut] {
			if g.code != inCode {
				if g.code {
					b.WriteString(markdownInlineCodeStart)
				} else {
					b.WriteString(markdownInlineCodeEnd)
				}
				inCode = g.code
			}
			b.WriteString(g.text)
		}
		if inCode {
			b.WriteString(markdownInlineCodeEnd)
		}
		lines = append(lines, b.String())
		glyphs = glyphs[next:]
	}
	return lines
}

func (m *markdownProjector) renderTableGrid(headers []string, rows [][]string) []string {
	widths := tableColumnWidths(headers, rows, m.width)
	if widths == nil { // No room for grid structure: preserve every cell as text.
		var out []string
		for _, row := range append([][]string{headers}, rows...) {
			out = append(out, wrapTableCell(strings.Join(row, " | "), max(1, m.width))...)
		}
		return out
	}
	border := func(left, middle, right string) string {
		parts := make([]string, len(widths))
		for i, w := range widths {
			parts[i] = strings.Repeat("─", w+2)
		}
		return left + strings.Join(parts, middle) + right
	}
	renderRow := func(cells []string) []string {
		wrapped := make([][]string, len(widths))
		height := 1
		for i, w := range widths {
			value := ""
			if i < len(cells) {
				value = cells[i]
			}
			wrapped[i] = wrapTableCell(value, w)
			height = max(height, len(wrapped[i]))
		}
		out := make([]string, height)
		for line := 0; line < height; line++ {
			parts := make([]string, len(widths))
			for i, w := range widths {
				value := ""
				if line < len(wrapped[i]) {
					value = wrapped[i][line]
				}
				parts[i] = " " + value + strings.Repeat(" ", max(0, w-gotui.StringWidth(stripMarkdownInlineStyleMarkers(value)))) + " "
			}
			out[line] = "│" + strings.Join(parts, "│") + "│"
		}
		return out
	}
	out := []string{border("┌", "┬", "┐")}
	out = append(out, renderRow(headers)...)
	out = append(out, border("├", "┼", "┤"))
	for i, row := range rows {
		out = append(out, renderRow(row)...)
		if i < len(rows)-1 {
			out = append(out, border("├", "┼", "┤"))
		}
	}
	return append(out, border("└", "┴", "┘"))
}
