package tui

import (
	"fmt"
	"slices"
	"strconv"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
)

// /settings is Pi's SettingsSelectorComponent over pi-tui's SettingsList
// (settings-selector.js, settings-list.js; settings_list.go), with the Pi
// settings gi implements. Enter or Space cycles the selected value or opens
// its submenu (Theme), typing searches the labels, Escape or Ctrl+C closes.
// Golden: scripts/golden-settings.mjs.

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
	treeFilter := c.cfg.TreeFilterMode
	if !slices.Contains(treeFilterModes, treeFilter) {
		treeFilter = "default"
	}
	theme := c.cfg.Theme
	if theme == "" {
		theme = systemThemeName
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
		{id: "tree-filter-mode", label: "Tree filter mode", description: "Default filter when opening /tree", value: treeFilter, values: treeFilterModes},
		{id: "tui-mode", label: "TUI mode", description: "Interface layout; regular mode uses the terminal's normal scrollback", value: mode, values: []string{"regular", "fullscreen"}},
		{id: "fullscreen-wheel-scroll-lines", label: "Fullscreen wheel scrolling", description: "Lines per mouse-wheel event in fullscreen mode; 'auto' speeds up fast wheel spins where the terminal does not", value: wheel, values: wheelValues},
		{id: "theme", label: "Theme", description: "Color theme for the interface", value: theme, submenu: func(current string, done func(*string)) settingsComponent {
			return newThemeSubmenu(current, piTerminal.scheme, availablePiThemes(), c.previewTheme, done)
		}},
	}
}

// previewTheme is Pi's ThemeController.preview: the setting's theme for the
// terminal's scheme, shown at once and not saved.
func (c *chatTUI) previewTheme(setting string) {
	if name := resolveThemeSetting(setting, piTerminal.scheme); name != "" {
		_ = setPiTheme(name)
		c.watchActiveTheme()
		c.invalidateFooter()
		c.markDirty()
	}
}

func (c *chatTUI) autoCompaction() bool {
	if c.engine != nil {
		return c.engine.CompactionPolicy().Enabled
	}
	return c.cfg.Compaction.Enabled
}

func (c *chatTUI) openSettingsMenu() {
	c.settingsList = &settingsListState{items: c.piSettingItems(), search: true, onChange: c.applySetting, onCancel: func() {
		c.settingsList = nil
		c.closeModelMenu()
	}}
	c.openMenuKind("settings")
}

func (c *chatTUI) settingsKeys() gotui.KeyMap {
	s := c.settingsList
	return settingsListKeys(s.handle, c.markDirty)
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
	case "tree-filter-mode":
		c.cfg.TreeFilterMode = value
		err = config.PersistTreeFilterMode(root, value)
	case "tui-mode":
		c.cfg.TUIMode = value
		err = config.PersistTUIMode(root, value)
		if err == nil {
			c.appendTranscript(fmt.Sprintf("TUI mode: %s (applies when gi next starts)", value))
		}
	case "theme":
		c.cfg.Theme = value
		err = config.PersistTheme(root, value)
		if _, loadErr := applyPiThemeSetting(value); loadErr != nil {
			c.appendTranscript(fmt.Sprintf("error: Failed to load theme %q: %v\nFell back to the system theme.", resolveThemeSetting(value, piTerminal.scheme), loadErr))
		}
		c.watchActiveTheme()
		c.invalidateFooter()
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

// piSettingsRows is the selector: a border, the SettingsList (or its open
// submenu) and a border.
func (c *chatTUI) piSettingsRows(width int) spanRows {
	rows := spanRows{piRule(width, piBorder)}
	rows = append(rows, c.settingsList.rows(width)...)
	return append(rows, piRule(width, piBorder))
}

func restyle(spans []gotui.TextSpan, style gotui.Style) []gotui.TextSpan {
	out := make([]gotui.TextSpan, len(spans))
	for i, s := range spans {
		out[i] = gotui.TextSpan{Text: s.Text, Style: style}
	}
	return out
}
