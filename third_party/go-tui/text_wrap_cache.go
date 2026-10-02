package tui

// textWrapCache keeps an element's last wrap: layout (HeightForWidth,
// text-wrapping recomputation) and rendering each wrapped the same text at
// the same width, so one frame wrapped every visible text up to three
// times (gi). Setting the text resets the cache; callers must not modify
// the returned lines.
type textWrapCache struct {
	// Unwrapped content width is independent of layout width, padding and style.
	// Text setters already reset this cache.
	intrinsicWidth int
	intrinsicValid bool
	spansWidth     int
	spans          [][]TextSpan
	textWidth      int
	text           []string
	// Grapheme clusters of each wrapped span line for drawing, valid for
	// clustersBase (the render context's base style).
	clusters     [][]styledCluster
	clustersBase Style
}

// lineClusters returns the clusters of wrapped line i, segmenting it once
// per wrap and base style.
func (c *textWrapCache) lineClusters(lines [][]TextSpan, i int, base Style) []styledCluster {
	if c == nil {
		return segmentLineClusters(lines[i], base)
	}
	if len(c.clusters) != len(lines) || c.clustersBase != base {
		c.clusters = make([][]styledCluster, len(lines))
		c.clustersBase = base
	}
	if c.clusters[i] == nil {
		c.clusters[i] = segmentLineClusters(lines[i], base)
	}
	return c.clusters[i]
}

func (e *Element) wrappedSpans(width int) [][]TextSpan {
	if e.wrapCache.spans != nil && e.wrapCache.spansWidth == width {
		return e.wrapCache.spans
	}
	lines := wrapSpans(e.richText, width)
	e.wrapCache.spans, e.wrapCache.spansWidth = lines, width
	e.wrapCache.clusters = nil
	return lines
}

func (e *Element) wrappedText(width int) []string {
	if e.wrapCache.text != nil && e.wrapCache.textWidth == width {
		return e.wrapCache.text
	}
	lines := wrapText(e.text, width)
	e.wrapCache.text, e.wrapCache.textWidth = lines, width
	return lines
}

// spanLinesCache returns the element's wrap cache when lines are its cached
// wrap or unwrapped line, so their clusters can be reused.
func (e *Element) spanLinesCache(lines [][]TextSpan) *textWrapCache {
	if len(lines) > 0 && len(e.wrapCache.spans) == len(lines) && &e.wrapCache.spans[0] == &lines[0] {
		return &e.wrapCache
	}
	return nil
}

// contentIntrinsicWidth measures immutable text once per assignment. Layout
// repeatedly asks this for every child, including off-screen table rows.
func (e *Element) contentIntrinsicWidth() int {
	if !e.wrapCache.intrinsicValid {
		if len(e.richText) > 0 {
			e.wrapCache.intrinsicWidth = richTextWidth(e.richText)
		} else {
			e.wrapCache.intrinsicWidth = stringWidth(e.text)
		}
		e.wrapCache.intrinsicValid = true
	}
	return e.wrapCache.intrinsicWidth
}

// unwrappedSpans shares the one-entry wrap/cluster cache. The -1 sentinel
// cannot alias a real wrap width. Unwrapped table rows otherwise resegment
// their Unicode clusters on every scroll frame.
func (e *Element) unwrappedSpans() [][]TextSpan {
	if e.wrapCache.spans != nil && e.wrapCache.spansWidth == -1 {
		return e.wrapCache.spans
	}
	e.wrapCache.spans = [][]TextSpan{e.richText}
	e.wrapCache.spansWidth = -1
	e.wrapCache.clusters = nil
	return e.wrapCache.spans
}
