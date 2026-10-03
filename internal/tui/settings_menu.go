package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
)

// /settings is Pi's SettingsSelectorComponent over pi-tui's SettingsList
// (settings-selector.js, settings-list.js), with the Pi settings gi
// implements. Enter or Space cycles the selected value, typing searches the
// labels, Escape or Ctrl+C closes. Golden: scripts/golden-settings.mjs.

const settingsMaxVisible = 10

type settingItem struct {
	id, label, description, value string
	values                        []string
}

type settingsListState struct {
	items    []settingItem
	query    string
	selected int
}

func (s *settingsListState) display() []settingItem {
	if s.query == "" {
		return s.items
	}
	wrapped := make([]slashItem, len(s.items))
	for i, it := range s.items {
		wrapped[i] = slashItem{value: strconv.Itoa(i), description: it.label}
	}
	var out []settingItem
	for _, w := range piFuzzyFilter(wrapped, s.query, func(it slashItem) string { return it.description }) {
		i, _ := strconv.Atoi(w.value)
		out = append(out, s.items[i])
	}
	return out
}

// piSettingItems are Pi's items, in Pi's order, for the settings gi has.
func (c *chatTUI) piSettingItems() []settingItem {
	quiet := c.cfg.QuietStartup
	if quiet == "" {
		quiet = "false"
	}
	mode := "fullscreen"
	if c.regularMode {
		mode = "regular"
	}
	wheel := "auto"
	if c.cfg.TUIWheelScrollLines > 0 {
		wheel = strconv.Itoa(c.cfg.TUIWheelScrollLines)
	}
	wheelValues := []string{"auto"}
	lines := []int{1, 2, 3, 5, 10}
	if n := c.cfg.TUIWheelScrollLines; n > 0 && !slices.Contains(lines, n) {
		lines = append(lines, n)
		slices.Sort(lines)
	}
	for _, n := range lines {
		wheelValues = append(wheelValues, strconv.Itoa(n))
	}
	return []settingItem{
		{id: "autocompact", label: "Auto-compact", description: "Automatically compact context when it gets too large", value: strconv.FormatBool(c.autoCompaction()), values: []string{"true", "false"}},
		{id: "hide-thinking", label: "Hide thinking", description: "Hide thinking blocks in assistant responses", value: strconv.FormatBool(c.cfg.HideThinkingBlock), values: []string{"true", "false"}},
		{id: "quiet-startup", label: "Quiet startup", description: "Disable verbose printing at startup (header: keep only the startup header)", value: quiet, values: []string{"true", "header", "false"}},
		{id: "tui-mode", label: "TUI mode", description: "Interface layout; regular mode uses the terminal's normal scrollback", value: mode, values: []string{"regular", "fullscreen"}},
		{id: "fullscreen-wheel-scroll-lines", label: "Fullscreen wheel scrolling", description: "Lines per mouse-wheel event in fullscreen mode; 'auto' speeds up fast wheel spins where the terminal does not", value: wheel, values: wheelValues},
	}
}

func (c *chatTUI) autoCompaction() bool {
	if c.engine != nil {
		return c.engine.CompactionPolicy().Enabled
	}
	return c.cfg.Compaction.Enabled
}

func (c *chatTUI) openSettingsMenu() {
	c.settingsList = &settingsListState{items: c.piSettingItems()}
	c.openMenuKind("settings")
}

func (c *chatTUI) settingsKeys() gotui.KeyMap {
	s := c.settingsList
	move := func(delta int) {
		if n := len(s.display()); n > 0 {
			s.selected = (s.selected + delta + n) % n
			c.markDirty()
		}
	}
	activate := func() {
		items := s.display()
		if s.selected >= len(items) {
			return
		}
		id := items[s.selected].id
		for i := range s.items {
			if it := &s.items[i]; it.id == id && len(it.values) > 0 {
				it.value = it.values[(slices.Index(it.values, it.value)+1)%len(it.values)]
				c.applySetting(it.id, it.value)
			}
		}
		c.markDirty()
	}
	search := func(query string) {
		s.query, s.selected = query, 0
		c.markDirty()
	}
	cancel := func(gotui.KeyEvent) {
		c.settingsList = nil
		c.closeModelMenu()
	}
	return gotui.KeyMap{
		gotui.OnPreemptStop(gotui.KeyUp, func(gotui.KeyEvent) { move(-1) }),
		gotui.OnPreemptStop(gotui.KeyDown, func(gotui.KeyEvent) { move(1) }),
		gotui.OnPreemptStop(gotui.KeyEnter, func(gotui.KeyEvent) { activate() }),
		gotui.OnPreemptStop(gotui.KeyEscape, cancel),
		gotui.OnPreemptStop(gotui.KeyCtrlC, cancel),
		gotui.OnPreemptStop(gotui.KeyBackspace, func(gotui.KeyEvent) {
			if r := []rune(s.query); len(r) > 0 {
				search(string(r[:len(r)-1]))
			}
		}),
		gotui.OnFocused(gotui.AnyRune, func(ke gotui.KeyEvent) {
			if ke.Rune == ' ' && s.query == "" {
				activate() // Pi: Space changes the value unless searching
				return
			}
			search(s.query + string(ke.Rune))
		}),
	}
}

// applySetting is Pi's onChange: applied at once and saved to settings.
func (c *chatTUI) applySetting(id, value string) {
	root := c.cfg.WorkspaceRoot
	var err error
	switch id {
	case "autocompact":
		enabled := value == "true"
		c.cfg.Compaction.Enabled = enabled
		if c.engine != nil {
			c.engine.SetAutoCompaction(enabled)
		}
		err = config.PersistCompactionEnabled(root, enabled)
	case "hide-thinking":
		c.cfg.HideThinkingBlock = value == "true"
		err = config.PersistHideThinkingBlock(root, c.cfg.HideThinkingBlock)
	case "quiet-startup":
		c.cfg.QuietStartup = value
		if value == "false" {
			c.cfg.QuietStartup = ""
		}
		err = config.PersistQuietStartup(root, value)
	case "tui-mode":
		c.cfg.TUIMode = value
		err = config.PersistTUIMode(root, value)
		if err == nil {
			c.appendTranscript(fmt.Sprintf("TUI mode: %s (applies when gi next starts)", value))
		}
	case "fullscreen-wheel-scroll-lines":
		lines, _ := strconv.Atoi(value) // "auto" is 0
		c.cfg.TUIWheelScrollLines = lines
		c.wheel.configure(lines)
		err = config.PersistWheelScrollLines(root, lines)
	}
	if err != nil {
		c.appendTranscript(fmt.Sprintf("warn: could not save %s: %v", id, err))
	}
}

// piSettingsRows is the selector: a border, SettingsList's main list and a
// border.
func (c *chatTUI) piSettingsRows(width int) spanRows {
	s := c.settingsList
	rows := spanRows{piRule(width, piBorder), piSearchRow(s.query, width), nil}
	hint := func() {
		rows = append(rows, nil, piTruncate([]gotui.TextSpan{{Text: "  Type to search · Enter/Space to change · Esc to cancel", Style: piFg(piDim)}}, width))
	}
	items := s.display()
	if len(items) == 0 {
		rows = append(rows, piTruncate([]gotui.TextSpan{{Text: "  No matching settings", Style: piFg(piDim)}}, width))
		hint()
		return append(rows, piRule(width, piBorder))
	}
	labelWidth := 0
	for _, it := range s.items {
		labelWidth = max(labelWidth, gotui.StringWidth(it.label))
	}
	labelWidth = min(36, labelWidth)
	start := max(0, min(s.selected-settingsMaxVisible/2, len(items)-settingsMaxVisible))
	end := min(start+settingsMaxVisible, len(items))
	for i := start; i < end; i++ {
		it := items[i]
		prefix, labelStyle, valueStyle := gotui.TextSpan{Text: "  "}, gotui.NewStyle(), piFg(piMuted)
		if i == s.selected {
			prefix, labelStyle, valueStyle = gotui.TextSpan{Text: "→ ", Style: piFg(piAccent)}, piFg(piAccent), piFg(piAccent)
		}
		label := it.label + strings.Repeat(" ", max(0, labelWidth-gotui.StringWidth(it.label)))
		value := truncateCells(it.value, width-2-labelWidth-2-2)
		rows = append(rows, piTruncate([]gotui.TextSpan{prefix, {Text: label, Style: labelStyle}, {Text: "  "}, {Text: value, Style: valueStyle}}, width))
	}
	if start > 0 || end < len(items) {
		rows = append(rows, []gotui.TextSpan{{Text: truncateCells(fmt.Sprintf("  (%d/%d)", s.selected+1, len(items)), width-2), Style: piFg(piDim)}})
	}
	if description := items[s.selected].description; description != "" {
		rows = append(rows, nil)
		for _, line := range piWrapLine([]gotui.TextSpan{{Text: description}}, max(1, width-4)) {
			rows = append(rows, append([]gotui.TextSpan{{Text: "  ", Style: piFg(piDim)}}, restyle(line, piFg(piDim))...))
		}
	}
	hint()
	return append(rows, piRule(width, piBorder))
}

func restyle(spans []gotui.TextSpan, style gotui.Style) []gotui.TextSpan {
	out := make([]gotui.TextSpan, len(spans))
	for i, s := range spans {
		out[i] = gotui.TextSpan{Text: s.Text, Style: style}
	}
	return out
}
