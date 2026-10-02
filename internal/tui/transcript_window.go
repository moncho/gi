package tui

import (
	"hash/fnv"
	"strconv"

	gotui "github.com/grindlemire/go-tui"
)

// Transcript windowing (#34): a frame lays out only the transcript blocks
// that intersect the viewport. The rest are fixed-height spacers sized from
// cached block heights, so the scroll container's content height, scroll
// offsets and "stick to bottom" behave exactly as with every block laid
// out, while a frame costs what the screen shows instead of the whole
// session.

// transcriptWindowMargin is how many extra rows above and below the
// viewport are laid out, so small scrolls stay smooth.
const transcriptWindowMargin = 8

// blockHeightKey identifies a block's rendered height: its content, the
// spacing context (previous kind), width and theme.
func blockHeightKey(block transcriptRenderableBlock, previousKind string, width int) uint64 {
	h := fnv.New64a()
	write := func(s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	write(block.Kind)
	write(block.Key)
	write(block.Header)
	write(block.Subheader)
	for _, line := range block.Body {
		write(line)
	}
	write(block.MarkdownSource)
	write(block.Footer)
	write(block.Status)
	write(block.ToolPath)
	write(block.ToolArg)
	write(block.StartedAt)
	write(block.EndedAt)
	if block.ToolContent != nil {
		write(*block.ToolContent)
	}
	write(strconv.FormatBool(block.Expanded) + strconv.FormatBool(block.Expandable) + strconv.FormatBool(block.Selected) +
		strconv.FormatBool(block.Static) + strconv.FormatBool(block.PreviewTail) + strconv.Itoa(block.PreviewLimit))
	write(previousKind)
	write(strconv.Itoa(width))
	write(piActiveTheme)
	return h.Sum64()
}

// transcriptBlockHeight returns a block's rendered height at width, cached
// unless the block is running (spinner and elapsed time change over time).
func (c *chatTUI) transcriptBlockHeight(block transcriptRenderableBlock, previousKind string, width int) int {
	cacheable := block.Status != "running" && block.Kind != "thinking_indicator"
	var key uint64
	if cacheable {
		key = blockHeightKey(block, previousKind, width)
		if h, ok := c.blockHeights[key]; ok {
			return h
		}
	}
	// Measuring builds the block; it must not leave hit targets for blocks
	// that are not on screen.
	refs := c.transcriptBlockRefs
	h := c.renderTranscriptBlockAfter(block, previousKind).HeightForWidth(width)
	c.transcriptBlockRefs = refs
	if cacheable {
		if c.blockHeights == nil || len(c.blockHeights) > 8192 {
			c.blockHeights = map[uint64]int{}
		}
		c.blockHeights[key] = h
	}
	return h
}

// addTranscriptWindow adds the blocks that intersect the viewport to the
// transcript container, with spacers standing in for the rest.
func (c *chatTUI) addTranscriptWindow(transcript *gotui.Element, blocks []transcriptRenderableBlock, width, viewport int) {
	type placed struct {
		block    transcriptRenderableBlock
		previous string
		top, h   int
	}
	items := make([]placed, 0, len(blocks))
	total := 0
	previousKind := ""
	for _, block := range blocks {
		h := c.transcriptBlockHeight(block, previousKind, width)
		items = append(items, placed{block: block, previous: previousKind, top: total, h: h})
		total += h
		if block.Kind != "thinking_indicator" {
			previousKind = block.Kind
		}
	}
	offset := c.transcriptScroll
	if c.stickToBottom {
		offset = total - viewport
	}
	offset = max(0, min(offset, max(0, total-viewport)))
	from, to := offset-transcriptWindowMargin, offset+viewport+transcriptWindowMargin
	spacer := func(rows int) {
		if rows > 0 {
			transcript.AddChild(gotui.New(gotui.WithWidthPercent(100), gotui.WithHeight(rows)))
		}
	}
	pending := 0 // rows of skipped blocks not yet emitted as a spacer
	for _, it := range items {
		if it.top+it.h <= from || it.top >= to {
			pending += it.h
			continue
		}
		spacer(pending)
		pending = 0
		transcript.AddChild(c.renderTranscriptBlockAfter(it.block, it.previous))
	}
	spacer(pending)
}
