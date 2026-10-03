package tui

import (
	"fmt"
	"strings"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/inference"
	"github.com/rcarmo/gi/internal/store"
)

// Pi-style layouts for the thinking selector (ThinkingSelectorComponent),
// the session selector (SessionSelectorComponent) and its actions list.
// Selection, filtering and gi's session-ownership checks are unchanged; only
// the presentation follows Pi.

var piThinkingDescriptions = map[string]string{
	"off":     "No reasoning",
	"minimal": "Very brief reasoning (~1k tokens)",
	"low":     "Light reasoning (~2k tokens)",
	"medium":  "Moderate reasoning (~8k tokens)",
	"high":    "Deep reasoning (~16k tokens)",
	"xhigh":   "Extra-high reasoning (~32k tokens)",
	"max":     "Maximum reasoning",
}

type spanRows = [][]gotui.TextSpan

func piRule(width int, color gotui.Color) []gotui.TextSpan {
	return []gotui.TextSpan{{Text: strings.Repeat("─", max(1, width)), Style: piFg(color)}}
}

func piSearchRow(query string, width int) []gotui.TextSpan {
	return []gotui.TextSpan{{Text: "> " + truncateCells(query, max(0, width-3))}, {Text: " ", Style: gotui.NewStyle().Reverse()}}
}

// piSelectRows renders SelectList rows: "→ " + primary column + muted
// description, the selected row wholly in accent.
func piSelectRows(labels, descriptions []string, selected, width, maxVisible int) spanRows {
	n := len(labels)
	if n == 0 {
		return spanRows{{{Text: "  No matching items", Style: piFg(piMuted)}}}
	}
	widest := 0
	for _, l := range labels {
		widest = max(widest, gotui.StringWidth(l)+slashPrimaryGap)
	}
	primary := max(slashMinPrimaryCol, min(widest, slashMaxPrimaryCol))
	start := max(0, min(selected-maxVisible/2, n-maxVisible))
	end := min(start+maxVisible, n)
	var rows spanRows
	for i := start; i < end; i++ {
		prefix := "  "
		if i == selected {
			prefix = "→ "
		}
		desc := ""
		if i < len(descriptions) {
			desc = descriptions[i]
		}
		value := truncateCells(labels[i], max(1, primary-slashPrimaryGap))
		if desc != "" && width > 40 {
			spacing := strings.Repeat(" ", max(1, primary-gotui.StringWidth(value)))
			if remaining := width - 2 - gotui.StringWidth(value) - len(spacing) - 2; remaining > slashMinDescription {
				desc = truncateCells(desc, remaining)
				if i == selected {
					rows = append(rows, []gotui.TextSpan{{Text: prefix + value + spacing + desc, Style: piFg(piAccent)}})
				} else {
					rows = append(rows, []gotui.TextSpan{{Text: prefix + value}, {Text: spacing + desc, Style: piFg(piMuted)}})
				}
				continue
			}
		}
		if i == selected {
			rows = append(rows, []gotui.TextSpan{{Text: prefix + value, Style: piFg(piAccent)}})
		} else {
			rows = append(rows, []gotui.TextSpan{{Text: prefix + value}})
		}
	}
	if start > 0 || end < n {
		rows = append(rows, []gotui.TextSpan{{Text: fmt.Sprintf("  (%d/%d)", selected+1, n), Style: piFg(piMuted)}})
	}
	return rows
}

// ----- thinking selector -----

func (c *chatTUI) thinkingMenuLevels() []string {
	levels := inference.ThinkingLevels(c.sessionModelLabel())
	if len(levels) == 0 {
		levels = []string{"low", "medium", "high"}
	}
	return levels
}

func (c *chatTUI) piThinkingSelectorRows(width int) spanRows {
	current := c.effectiveThinking(c.cfg.DefaultProvider, c.cfg.DefaultModel, c.cfg.DefaultThinkingLevel)
	saved := strings.ToLower(strings.TrimSpace(config.Load(c.cfg.WorkspaceRoot).DefaultThinkingLevel))
	labels := make([]string, len(c.modelMenuChoices))
	descs := make([]string, len(c.modelMenuChoices))
	for i, level := range c.modelMenuChoices {
		mark := "  "
		if level == current {
			mark = "✓ "
		}
		labels[i] = mark + level
		descs[i] = piThinkingDescriptions[level]
		if level == saved {
			descs[i] += " · default"
		}
	}
	rows := spanRows{piRule(width, piBorder), {}, {{Text: "Thinking Level"}}, {},
		{{Text: truncateCells(defaultPiKeys.display("Shift+Tab")+" cycles thinking levels in-session", width)}}, {},
		piSearchRow(c.modelMenuQuery, width), {}}
	rows = append(rows, piSelectRows(labels, descs, c.modelMenuSelected, width, max(1, len(labels)))...)
	if c.modelMenuError != "" {
		rows = append(rows, []gotui.TextSpan{{Text: truncateCells("  "+c.modelMenuError, width), Style: piFg(piError)}})
	}
	return append(rows, spanRows{{},
		{{Text: truncateCells("  Enter to select · Ctrl+S to set as default · Escape/Ctrl+C to cancel", width), Style: piFg(piDim)}},
		piRule(width, piBorder)}...)
}

// acceptThinkingMenuAsDefault is Pi's Ctrl+S in the thinking selector.
func (c *chatTUI) acceptThinkingMenuAsDefault() {
	if c.modelMenuKind != "thinking" || c.modelMenuSelected < 0 || c.modelMenuSelected >= len(c.modelMenuChoices) {
		return
	}
	level := c.modelMenuChoices[c.modelMenuSelected]
	c.acceptModelMenuSelection()
	if err := config.PersistModelSelection(c.cfg.WorkspaceRoot, c.cfg.DefaultProvider, c.cfg.DefaultModel, level, c.cfg.EnabledModels); err != nil {
		c.appendTranscript(fmt.Sprintf("warn: failed to save default thinking level: %v", err))
	}
}

// ----- session selector -----

type sessionPickerRow struct {
	title, right string
	current      bool
	named        bool
}

func (c *chatTUI) sessionPickerRowFor(sess *store.Session) sessionPickerRow {
	title := strings.TrimSpace(sess.Title)
	named := title != "" && !strings.HasPrefix(title, "@")
	if title == "" {
		title = sess.ID
	}
	status, _ := sess.State["status"].(string)
	if status == "" {
		status = "idle"
	}
	if archived, _ := sess.State["archived_at"].(string); archived != "" {
		status = "archived"
	}
	if pinned, _ := sess.State["pinned"].(bool); pinned {
		status += " · pinned"
	}
	right := fmt.Sprintf("@%s · %s", c.agentIDForSession(sess), status)
	if age := formatSessionAge(sess.UpdatedAt); age != "" {
		right += " " + age
	}
	return sessionPickerRow{title: title, right: right, current: sess.ID == c.sessionID, named: named}
}

// formatSessionAge mirrors Pi's compact session dates.
func formatSessionAge(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(ts))
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	return t.Format("2006-01-02")
}

func (c *chatTUI) piSessionSelectorRows(width int) spanRows {
	muted, accent, dim := piFg(piMuted), piFg(piAccent), piFg(piDim)
	title := "Resume Session (All)"
	right := "◉ All  Sort: Recent"
	left := truncateCells(title, max(0, width-gotui.StringWidth(right)-1))
	header := []gotui.TextSpan{{Text: left, Style: gotui.NewStyle().Bold()}}
	if pad := width - gotui.StringWidth(left) - gotui.StringWidth(right); pad >= 1 {
		header = append(header, gotui.TextSpan{Text: strings.Repeat(" ", pad)}, gotui.TextSpan{Text: "◉ All", Style: accent}, gotui.TextSpan{Text: "  Sort: ", Style: muted}, gotui.TextSpan{Text: "Recent", Style: accent})
	}
	hint := clipSpans([]gotui.TextSpan{{Text: "enter", Style: dim}, {Text: " select", Style: muted}, {Text: " · ", Style: muted}, {Text: "→", Style: dim}, {Text: " actions", Style: muted}, {Text: " · ", Style: muted}, {Text: "esc", Style: dim}, {Text: " cancel", Style: muted}}, width)
	rows := spanRows{{}, piRule(width, piAccent), {}, header, hint, {}, piSearchRow(c.modelMenuQuery, width), {}}
	n := len(c.modelMenuChoices)
	switch {
	case c.modelMenuError != "":
		rows = append(rows, []gotui.TextSpan{{Text: truncateCells("  "+c.modelMenuError, width), Style: piFg(piError)}})
	case n == 0:
		rows = append(rows, []gotui.TextSpan{{Text: "  No sessions found", Style: muted}})
	default:
		visible := c.modelMenuVisibleRows()
		start := max(0, min(c.modelMenuSelected-visible/2, n-visible))
		end := min(start+visible, n)
		for i := start; i < end; i++ {
			row := c.modelMenuSessionRows[c.modelMenuChoices[i]]
			selected := i == c.modelMenuSelected
			cursor := gotui.TextSpan{Text: "  "}
			if selected {
				cursor = gotui.TextSpan{Text: "› ", Style: accent}
			}
			msgStyle := gotui.NewStyle()
			switch {
			case row.current:
				msgStyle = accent
			case row.named:
				msgStyle = piFg(piWarning)
			}
			if selected {
				msgStyle = msgStyle.Bold()
			}
			avail := max(10, width-2-gotui.StringWidth(row.right)-2)
			msg := truncateWithEllipsis(row.title, avail, "…")
			spacing := max(1, width-2-gotui.StringWidth(msg)-gotui.StringWidth(row.right))
			line := []gotui.TextSpan{cursor, {Text: msg, Style: msgStyle}, {Text: strings.Repeat(" ", spacing)}, {Text: row.right, Style: dim}}
			line = clipSpans(line, width)
			if selected {
				for j := range line {
					line[j].Style = line[j].Style.Background(piUserBg) // selectedBg
				}
			}
			rows = append(rows, line)
		}
		if start > 0 || end < n {
			rows = append(rows, []gotui.TextSpan{{Text: fmt.Sprintf("  (%d/%d)", c.modelMenuSelected+1, n), Style: muted}})
		}
	}
	return append(rows, spanRows{{}, piRule(width, piAccent)}...)
}

// ----- session actions -----

func (c *chatTUI) piSessionActionRows(width int) spanRows {
	rows := spanRows{{}, piRule(width, piAccent), {},
		{{Text: truncateCells("Session actions", width), Style: gotui.NewStyle().Bold()}},
		{{Text: truncateCells(c.sessionActions.targetLabel, width), Style: piFg(piMuted)}}, {}}
	if c.modelMenuError != "" {
		rows = append(rows, []gotui.TextSpan{{Text: truncateCells("  "+c.modelMenuError, width), Style: piFg(piError)}})
	}
	rows = append(rows, piSelectRows(c.modelMenuChoices, nil, c.modelMenuSelected, width, max(1, c.modelMenuVisibleRows()))...)
	return append(rows, spanRows{{},
		{{Text: truncateCells("  Enter to select · Escape to go back", width), Style: piFg(piDim)}},
		piRule(width, piAccent)}...)
}

func renderSpanRows(rows spanRows) *gotui.Element {
	block := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100), gotui.WithHeight(len(rows)))
	for _, spans := range rows {
		block.AddChild(gotui.New(gotui.WithWidthPercent(100), gotui.WithHeight(1), gotui.WithWrap(false), gotui.WithRichText(spans...)))
	}
	return block
}

// piMenuRows returns the Pi layout for selector kinds that have one.
func (c *chatTUI) piMenuRows(width int) (spanRows, bool) {
	switch c.modelMenuKind {
	case "thinking":
		return c.piThinkingSelectorRows(width), true
	case "session":
		return c.piSessionSelectorRows(width), true
	case "session-actions":
		return c.piSessionActionRows(width), true
	case "fork":
		return c.piForkSelectorRows(width), true
	case "select":
		return c.piSelectDialogRows(width), true
	case "mcp-manager":
		if c.mcpManager != nil {
			return c.piMCPManagerRows(width), true
		}
	case "scoped-models":
		if c.scopedModels != nil {
			return c.piScopedModelsRows(width), true
		}
	case "auth-selector":
		if c.authSelector != nil {
			return c.piAuthSelectorRows(width), true
		}
	case "login-dialog":
		if c.loginDialog != nil {
			return c.piLoginDialogRows(width), true
		}
	case "settings":
		if c.settingsList != nil {
			return c.piSettingsRows(width), true
		}
	case "tree":
		if c.treeSelector != nil {
			return c.treeSelector.list.selectorRows(width), true
		}
	case "editor-dialog":
		if c.editorDialog != nil {
			return c.editorDialog.rows(width), true
		}
	}
	return nil, false
}
