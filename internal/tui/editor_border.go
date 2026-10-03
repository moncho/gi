package tui

import (
	"fmt"
	"strings"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/inference"
)

// Pi's CustomEditor draws horizontal borders above and below the draft, in the
// thinking-level border color (bashMode for `!` commands). While the agent is
// working, the status indicator is embedded in the top border:
//
//	── ⠋ Working ───────────────
//
// and editor overflow is reported as a centered "↑ N more" / "↓ N more" label.

const piWorkingMessage = "Working"

func (c *chatTUI) editorBorderColor() gotui.Color {
	if c.input != nil && strings.HasPrefix(strings.TrimLeft(c.input.Text(), " \t\r\n"), "!") {
		return piBashMode
	}
	return piThinkingBorderColor(c.effectiveThinking(c.cfg.DefaultProvider, c.cfg.DefaultModel, c.cfg.DefaultThinkingLevel))
}

// effectiveThinking mirrors Pi's session thinking level: the selected level,
// else the configured default, else Pi's "medium"; "off" for models the
// registry knows do not reason. "" means the model does not reason at all.
func (c *chatTUI) effectiveThinking(provider, model, level string) string {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" {
		level = strings.ToLower(strings.TrimSpace(c.cfg.DefaultThinkingLevel))
	}
	if level == "" {
		level = "medium"
	}
	id := strings.TrimSpace(model)
	if provider = strings.TrimSpace(provider); provider != "" && !strings.HasPrefix(id, provider+"/") && !strings.Contains(id, "/") {
		id = provider + "/" + id
	}
	// The border asks every frame; a model lookup copies the provider's
	// whole catalogue in go-ai, so the answer is kept until the model or
	// level changes.
	key := id + "\x00" + level
	if c.thinkingMemoKey == key {
		return c.thinkingMemoValue
	}
	value := level
	if effective, known := inference.EffectiveThinking(id, level); known {
		value = effective // clamped like Pi; "" for non-reasoning models
	}
	c.thinkingMemoKey, c.thinkingMemoValue = key, value
	return value
}

// editorStatus returns Pi's active status indicator (spinner frame, message and
// their colors), or ok=false while idle.
func (c *chatTUI) editorStatus(now time.Time) (frame, message string, spinner, text gotui.Style, ok bool) {
	border := piFg(c.editorBorderColor())
	switch {
	case c.compaction.active:
		return brailleSpinnerFrame(now), "Compacting context... (Esc to cancel)", piFg(piAccent), piFg(piMuted), true
	case c.branchSummaryCancel != nil:
		return brailleSpinnerFrame(now), "Summarizing branch... (Esc to cancel)", piFg(piAccent), piFg(piMuted), true
	case c.running:
		return brailleSpinnerFrame(now), piWorkingMessage, border, border, true
	}
	return "", "", gotui.Style{}, gotui.Style{}, false
}

func piScrollBorder(direction string, hidden, width int) string {
	width = max(0, width)
	label := fmt.Sprintf(" %s %d more ", direction, hidden)
	labelWidth := gotui.StringWidth(label)
	if labelWidth+2 <= width {
		left := (width - labelWidth) / 2
		return strings.Repeat("─", left) + label + strings.Repeat("─", width-left-labelWidth)
	}
	indicator := fmt.Sprintf("─── %s %d more ", direction, hidden)
	if rest := width - gotui.StringWidth(indicator); rest >= 0 {
		return indicator + strings.Repeat("─", rest)
	}
	ellipsis := "..."[:min(3, width)]
	return truncateCells(indicator, width-len(ellipsis)) + ellipsis
}

func truncateCells(s string, width int) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := gotui.StringWidth(string(r))
		if used+w > width {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String()
}

// editorTopBorderSpans ports CustomEditor.renderTopBorder.
func (c *chatTUI) editorTopBorderSpans(width, hidden int, now time.Time) []gotui.TextSpan {
	border := piFg(c.editorBorderColor())
	plain := func() []gotui.TextSpan {
		if hidden > 0 {
			return []gotui.TextSpan{{Text: piScrollBorder("↑", hidden, width), Style: border}}
		}
		return []gotui.TextSpan{{Text: strings.Repeat("─", max(0, width)), Style: border}}
	}
	frame, message, spinnerStyle, messageStyle, ok := c.editorStatus(now)
	if !ok || width <= 0 {
		return plain()
	}
	// Loader text is "<frame> <message>", truncated to width-5 cells.
	status := []gotui.TextSpan{{Text: frame, Style: spinnerStyle}}
	if avail := max(1, width-5) - gotui.StringWidth(frame) - 1; avail > 0 {
		status = append(status, gotui.TextSpan{Text: " " + truncateCells(message, avail), Style: messageStyle})
	}
	statusWidth := spansWidth(status)
	spinnerOnly := []gotui.TextSpan{{Text: truncateCells(frame, width), Style: spinnerStyle}}
	overflow := ""
	if hidden > 0 {
		overflow = fmt.Sprintf(" ↑ %d more ", hidden)
	}
	overflowWidth := gotui.StringWidth(overflow)
	overflowStart := (width - overflowWidth) / 2
	canFit := func() bool {
		return overflow != "" && overflowWidth+2 <= width && overflowStart-(3+statusWidth+1) >= 1
	}
	if overflow != "" && !canFit() {
		status, statusWidth = spinnerOnly, spansWidth(spinnerOnly)
	}
	join := func(prefix string, mid []gotui.TextSpan, suffix string) []gotui.TextSpan {
		out := []gotui.TextSpan{{Text: prefix, Style: border}}
		out = append(out, mid...)
		return append(out, gotui.TextSpan{Text: suffix, Style: border})
	}
	if canFit() {
		left := 3 + statusWidth + 1
		return join("── ", status, " "+strings.Repeat("─", overflowStart-left)+overflow+strings.Repeat("─", width-overflowStart-overflowWidth))
	}
	if width >= statusWidth+5 {
		return join("── ", status, " "+strings.Repeat("─", width-statusWidth-4))
	}
	status, statusWidth = spinnerOnly, spansWidth(spinnerOnly)
	prefix := min(3, max(0, width-statusWidth))
	return join(strings.Repeat("─", prefix), status, strings.Repeat("─", max(0, width-prefix-statusWidth)))
}

func spansWidth(spans []gotui.TextSpan) int {
	w := 0
	for _, s := range spans {
		w += gotui.StringWidth(s.Text)
	}
	return w
}

func (c *chatTUI) editorBottomBorderText(width, hidden int) string {
	if hidden > 0 {
		return piScrollBorder("↓", hidden, width)
	}
	return strings.Repeat("─", max(0, width))
}

func borderElement(spans []gotui.TextSpan) *gotui.Element {
	return gotui.New(gotui.WithWidthPercent(100), gotui.WithHeight(1), gotui.WithWrap(false), gotui.WithRichText(spans...))
}

func (c *chatTUI) renderEditorTopBorder(input *multilineInput, width int) *gotui.Element {
	above, _ := input.hiddenRows()
	return borderElement(c.editorTopBorderSpans(width, above, time.Now()))
}

func (c *chatTUI) renderEditorBottomBorder(input *multilineInput, width int) *gotui.Element {
	_, below := input.hiddenRows()
	return borderElement([]gotui.TextSpan{{Text: c.editorBottomBorderText(width, below), Style: piFg(c.editorBorderColor())}})
}
