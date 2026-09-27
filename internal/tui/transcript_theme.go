package tui

import gotui "github.com/grindlemire/go-tui"

// The user message alone gets a background; transcript output stays on the
// terminal background. Keep the user color independent of the ANSI palette.
var (
	piUserBg = gotui.RGBColor(0x34, 0x35, 0x41)
	piText   = gotui.RGBColor(0xd4, 0xd4, 0xd4)
	piError  = gotui.RGBColor(0xf4, 0x87, 0x71)
)

func transcriptBand(kind, status string) (gotui.Color, bool) {
	if kind == "user" {
		return piUserBg, true
	}
	return gotui.Color{}, false
}

func applyTranscriptBand(element *gotui.Element, block transcriptRenderableBlock) {
	if bg, ok := transcriptBand(block.Kind, block.Status); ok {
		style := gotui.NewStyle().Background(bg)
		element.SetBackground(&style)
	}
}

// The fullscreen transcript navigates rendered rows, including wrapped and
// expanded output. The fallback is only for tests/before the first layout.
func (c *chatTUI) transcriptMaxScroll() int {
	if c.transcriptRef != nil && c.transcriptRef.El() != nil {
		_, maxY := c.transcriptRef.El().MaxScroll()
		return maxY
	}
	return max(0, len(c.visibleTranscript())-c.transcriptViewportHeight())
}

func (c *chatTUI) toggleToolOutput() {
	blocks := c.buildTranscriptRenderableBlocks(c.visibleTranscript())
	expand := false
	for _, b := range blocks {
		if (b.Kind == "tool" || b.Kind == "bash" || b.Kind == "local") && b.Expandable && !b.Expanded {
			expand = true
			break
		}
	}
	for _, b := range blocks {
		if (b.Kind == "tool" || b.Kind == "bash" || b.Kind == "local") && b.Expandable {
			c.transcriptExpanded[b.Key] = expand
		}
	}
	if c.app != nil {
		c.app.MarkDirty()
	}
}

func (c *chatTUI) setTranscriptPosition(row int) {
	c.transcriptScroll = max(0, row)
	if c.transcriptRef != nil && c.transcriptRef.El() != nil {
		c.transcriptRef.El().ScrollTo(0, c.transcriptScroll)
	}
}

// Only user messages have a padded background. All other transcript content
// has a separating blank row, but no boxed padding or background.
func transcriptSpacing(kind string) (separator, vertical, horizontal int) {
	switch kind {
	case "user":
		return 0, 1, 1
	case "assistant":
		return 1, 0, 1
	case "tool", "bash", "local", "error", "thought", "thinking", "thinking_indicator", "hook", "route", "dispatcher", "subturn", "compact":
		return 1, 0, 1
	default:
		return 0, 0, 0
	}
}

func padTranscriptBlock(content *gotui.Element, block transcriptRenderableBlock) *gotui.Element {
	separator, vertical, horizontal := transcriptSpacing(block.Kind)
	if separator == 0 && vertical == 0 && horizontal == 0 {
		return content
	}
	band := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100), gotui.WithPaddingTRBL(vertical, horizontal, vertical, horizontal))
	band.AddChild(content)
	applyTranscriptBand(band, block)
	if separator == 0 {
		return band
	}
	wrapper := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100))
	wrapper.AddChild(gotui.New(gotui.WithWidthPercent(100), gotui.WithHeight(separator)))
	wrapper.AddChild(band)
	return wrapper
}

// Pi inserts a Spacer(1) before a subsequent user message. An assistant
// already has a leading spacer; only add one after it when the next block
// does not provide its own separation. Keep it outside the next message band.
func assistantGapBefore(previousKind, nextKind string) int {
	separator, _, _ := transcriptSpacing(nextKind)
	if previousKind == "assistant" && separator == 0 {
		return 1
	}
	return 0
}

func (c *chatTUI) renderTranscriptBlockAfter(block transcriptRenderableBlock, previousKind string) *gotui.Element {
	content := c.renderTranscriptBlock(block)
	if assistantGapBefore(previousKind, block.Kind) == 0 {
		return content
	}
	wrapper := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100))
	wrapper.AddChild(gotui.New(gotui.WithWidthPercent(100), gotui.WithHeight(1)))
	wrapper.AddChild(content)
	return wrapper
}
