package tui

// textWrapCache keeps an element's last wrap: layout (HeightForWidth,
// text-wrapping recomputation) and rendering each wrapped the same text at
// the same width, so one frame wrapped every visible text up to three
// times (gi). Setting the text resets the cache; callers must not modify
// the returned lines.
type textWrapCache struct {
	spansWidth int
	spans      [][]TextSpan
	textWidth  int
	text       []string
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
// wrap (not an unwrapped single line), so their clusters can be reused.
func (e *Element) spanLinesCache(lines [][]TextSpan) *textWrapCache {
	if len(lines) > 0 && len(e.wrapCache.spans) == len(lines) && &e.wrapCache.spans[0] == &lines[0] {
		return &e.wrapCache
	}
	return nil
}
