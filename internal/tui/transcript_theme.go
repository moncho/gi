package tui

import (
	"strings"

	gotui "github.com/grindlemire/go-tui"
)

// Pi's built-in dark theme (theme/dark.json), resolved to truecolor by Pi's
// own okhsl conversion. Keep these values in sync with Pi, not the ANSI
// palette, so gi and Pi render the same colors in the same terminal.
// The user message alone gets a background; transcript output stays on the
// terminal background.
var (
	piText            = gotui.RGBColor(222, 224, 225)
	piMuted           = gotui.RGBColor(157, 165, 169)
	piDim             = gotui.RGBColor(126, 136, 142)
	piAccent          = gotui.RGBColor(167, 152, 215)
	piError           = gotui.RGBColor(234, 127, 129)
	piWarning         = gotui.RGBColor(205, 154, 34)
	piSuccess         = gotui.RGBColor(104, 183, 141)
	piThinkingText    = gotui.RGBColor(150, 160, 164)
	piUserBg          = gotui.RGBColor(33, 59, 73)
	piMdCode          = piAccent
	piBashMode        = gotui.RGBColor(94, 178, 134)
	piBorderMuted     = gotui.RGBColor(118, 129, 134)
	piThinkingOff     = gotui.RGBColor(108, 118, 123)
	piThinkingMinimal = gotui.RGBColor(104, 128, 141)
	piThinkingLow     = gotui.RGBColor(84, 137, 164)
	piThinkingMedium  = gotui.RGBColor(97, 133, 204)
	piThinkingHigh    = gotui.RGBColor(151, 118, 229)
	piThinkingXhigh   = gotui.RGBColor(222, 84, 193)
	piThinkingMax     = gotui.RGBColor(254, 84, 98)
)

func piFg(c gotui.Color) gotui.Style { return gotui.NewStyle().Foreground(c) }

// Pi colors the editor border (and the embedded working status) by thinking
// level, or with bashMode while the draft is a `!` shell command.
func piThinkingBorderColor(level string) gotui.Color {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "minimal":
		return piThinkingMinimal
	case "low":
		return piThinkingLow
	case "medium":
		return piThinkingMedium
	case "high":
		return piThinkingHigh
	case "xhigh":
		return piThinkingXhigh
	case "max":
		return piThinkingMax
	}
	return piThinkingOff
}

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
	case "tool", "bash", "local", "error", "thought", "thinking", "hook", "route", "dispatcher", "subturn", "compact":
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
	if block.Kind == "thinking_indicator" {
		return c.renderTranscriptBlock(block)
	}
	content := c.renderTranscriptBlock(block)
	if assistantGapBefore(previousKind, block.Kind) == 0 {
		return content
	}
	wrapper := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100))
	wrapper.AddChild(gotui.New(gotui.WithWidthPercent(100), gotui.WithHeight(1)))
	wrapper.AddChild(content)
	return wrapper
}
