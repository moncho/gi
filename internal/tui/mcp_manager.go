package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	gotui "github.com/grindlemire/go-tui"
	gimcp "github.com/rcarmo/gi/internal/mcp"
)

// Pi's /mcp manager (pi-coding-agent extensions/mcp/ui.js McpManagerView and
// the manage loop in index.js): the server list, servers needing attention
// first; a server's actions (sign in, tools, reconnect, sign out, exposure,
// enable/disable); its tools; and its exposure. Menus are rebuilt from the
// engine's state on every frame, so they follow servers as they connect.
// Changes are saved to the mcp.json that defines the server.

const mcpManagerMaxVisible = 12 // ui.js MAX_VISIBLE_ITEMS

// mcpExposureDescriptions is Pi's EXPOSURE_DESCRIPTIONS, in Pi's order.
var mcpExposureDescriptions = []struct{ name, description string }{
	{gimcp.ExposureCodemode, "called from codemode scripts, which find them with searchTools()"},
	{gimcp.ExposureDeferred, "not declared until tool_search loads them, then called directly; no codemode needed"},
	{gimcp.ExposureDirect, "declared to the model like built-in tools"},
}

// mcpMenu is the value ui.menu builds: a titled list, or the empty text.
type mcpMenu struct {
	title, details, errorText, empty string
	confirmLabel, cancelLabel        string
	items                            []slashItem // name: label, value, description
	selected                         string      // preferred value when nothing is selected yet
}

type mcpManagerState struct {
	screen   string            // "servers", "server", "tools", "exposure"
	server   string            // the server of the inner screens
	selected map[string]string // screen -> selected value, kept across rebuilds (Pi)
	messages map[string]string // server -> last action's message (Pi's server.message)
	status   [2]string         // title and text of Pi's status screen while an action runs
}

func (c *chatTUI) openMCPManager() {
	c.mcpManager = &mcpManagerState{screen: "servers", selected: map[string]string{}, messages: map[string]string{}}
	c.modelMenuOpen = true
	c.modelMenuKind = "mcp-manager"
	c.modelMenuError = ""
	c.inputActive = false
	if c.engine != nil {
		c.engine.SetMCPChangeListener(func() { c.runOnUI(func() {}) })
	}
	if c.app != nil {
		c.app.BlurFocused()
		c.app.MarkDirty()
	}
}

func (c *chatTUI) closeMCPManager() {
	c.closeModelMenu() // also clears the manager state
}

func mcpStatusOf(statuses []gimcp.Status, name string) (gimcp.Status, bool) {
	for _, st := range statuses {
		if st.Name == name {
			return st, true
		}
	}
	return gimcp.Status{}, false
}

// mcpAttentionRank is Pi's attentionRank: servers that need the user first.
func mcpAttentionRank(st gimcp.Status) int {
	switch st.State {
	case gimcp.StateDisabled:
		return 5
	case gimcp.StateNeedsAuth:
		return 0
	case gimcp.StateFailed:
		return 1
	case gimcp.StateDisconnected:
		return 2
	case gimcp.StateConnected:
		return 4
	}
	return 3
}

func mcpExposureOf(st gimcp.Status) string {
	if st.Exposure == "" {
		return gimcp.ExposureCodemode
	}
	return st.Exposure
}

// mcpManagerMenu builds the current screen's menu (Pi's serversMenu,
// serverMenu, showTools and chooseExposure).
func (c *chatTUI) mcpManagerMenu() mcpMenu {
	m := c.mcpManager
	statuses, errs := c.engine.MCPStatus()
	switch m.screen {
	case "server":
		return c.mcpServerMenu(statuses, m.server)
	case "tools":
		st, _ := mcpStatusOf(statuses, m.server)
		exposure := mcpExposureOf(st)
		details := "Exposure " + exposure + ": "
		if exposure == gimcp.ExposureHidden {
			details += "unreachable"
		} else {
			for _, e := range mcpExposureDescriptions {
				if e.name == exposure {
					details += e.description
				}
			}
		}
		if cfg, ok := c.engine.MCPServerConfig(m.server); ok && len(cfg.ToolExposure) > 0 {
			details += "\nSome tools override it with toolExposure."
		}
		menu := mcpMenu{title: "Tools of " + m.server, details: details, empty: "The server offers no tools.", confirmLabel: "back", cancelLabel: "back"}
		for _, tool := range c.engine.MCPServerTools(m.server) {
			description := strings.SplitN(tool.Description, "\n", 2)[0]
			if tool.Exposure != exposure {
				description = "[" + tool.Exposure + "] " + description
			}
			menu.items = append(menu.items, slashItem{name: tool.Name, value: tool.Name, description: description})
		}
		return menu
	case "exposure":
		st, _ := mcpStatusOf(statuses, m.server)
		current := mcpExposureOf(st)
		menu := mcpMenu{title: "Exposure of " + m.server, details: "Saved to " + st.Source + ".", selected: current, confirmLabel: "save", cancelLabel: "back"}
		for _, e := range mcpExposureDescriptions {
			mark := "  "
			if e.name == current {
				mark = "✓ "
			}
			menu.items = append(menu.items, slashItem{name: mark + e.name, value: e.name, description: e.description})
		}
		return menu
	}
	var notices []string
	for _, err := range errs {
		notices = append(notices, "config: "+err.Error())
	}
	sorted := append([]gimcp.Status(nil), statuses...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if a, b := mcpAttentionRank(sorted[i]), mcpAttentionRank(sorted[j]); a != b {
			return a < b
		}
		return sorted[i].Name < sorted[j].Name
	})
	menu := mcpMenu{title: "MCP servers", errorText: strings.Join(notices, "\n"),
		empty:        "No MCP servers configured. Add them to " + gimcp.UserConfigPath() + " or .pi/mcp.json.",
		confirmLabel: "manage", cancelLabel: "close"}
	for _, st := range sorted {
		scope := st.Scope
		if scope == "" {
			scope = st.Source
		}
		menu.items = append(menu.items, slashItem{name: st.Name, value: st.Name,
			description: gimcp.DescribeState(st) + " · " + mcpExposureOf(st) + " · " + scope})
	}
	return menu
}

func (c *chatTUI) mcpServerMenu(statuses []gimcp.Status, name string) mcpMenu {
	st, ok := mcpStatusOf(statuses, name)
	if !ok {
		return mcpMenu{title: name, empty: "This server is no longer configured.", cancelLabel: "back"}
	}
	saved := "saved to mcp.json"
	if st.Scope != "" {
		saved = "saved to the " + st.Scope + " mcp.json"
	}
	var items []slashItem
	add := func(value, label, description string) {
		items = append(items, slashItem{name: label, value: value, description: description})
	}
	if st.State == gimcp.StateDisabled {
		add("enable", "Enable", saved)
	} else {
		if st.State == gimcp.StateNeedsAuth {
			add("signin", "Sign in", "opens the browser")
		}
		if st.State == gimcp.StateConnected {
			add("tools", "Tools", fmt.Sprintf("%d offered", st.Tools))
		}
		switch st.State {
		case gimcp.StateFailed, gimcp.StateDisconnected, gimcp.StateConnected, gimcp.StateNeedsAuth:
			add("reconnect", "Reconnect", "")
		}
		if st.State == gimcp.StateConnected && c.engine.MCPUsesOAuth(name) {
			add("signout", "Sign out", "deletes the stored credentials")
		}
		add("exposure", "Exposure", mcpExposureOf(st))
		add("disable", "Disable", saved)
	}
	scope := st.Scope
	if scope == "" {
		scope = "config"
	}
	state := gimcp.DescribeState(st)
	if st.State == gimcp.StateFailed {
		state = "failed" // Pi's describeState(server, false)
	}
	var errLines []string
	if msg := c.mcpManager.messages[name]; msg != "" {
		errLines = append(errLines, msg)
	}
	if st.State != gimcp.StateConnected && st.Error != "" {
		errLines = append(errLines, st.Error)
	}
	menu := mcpMenu{title: "MCP server " + name, details: strings.Join([]string{st.Endpoint, scope + ": " + st.Source, "State: " + state}, "\n"),
		errorText: strings.Join(errLines, "\n"), items: items, confirmLabel: "select", cancelLabel: "back"}
	if len(items) > 0 {
		menu.selected = items[0].value
	}
	return menu
}

// mcpManagerSelection returns the selected index, keeping the selected value
// across rebuilds as Pi's menu does.
func (c *chatTUI) mcpManagerSelection(menu mcpMenu) int {
	wanted, ok := c.mcpManager.selected[c.mcpManager.screen]
	if !ok {
		wanted = menu.selected
	}
	for i, item := range menu.items {
		if item.value == wanted {
			return i
		}
	}
	return 0
}

func (c *chatTUI) mcpManagerKeys() gotui.KeyMap {
	move := func(delta int) {
		if c.mcpManager.status[0] != "" {
			return
		}
		menu := c.mcpManagerMenu()
		n := len(menu.items)
		if n == 0 {
			return
		}
		i := (c.mcpManagerSelection(menu) + delta + n) % n // SelectList wraps
		c.mcpManager.selected[c.mcpManager.screen] = menu.items[i].value
		c.markDirty()
	}
	return gotui.KeyMap{
		gotui.OnPreemptStop(gotui.KeyUp, func(gotui.KeyEvent) { move(-1) }),
		gotui.OnPreemptStop(gotui.KeyDown, func(gotui.KeyEvent) { move(1) }),
		gotui.OnPreemptStop(gotui.KeyEnter, func(gotui.KeyEvent) { c.mcpManagerConfirm() }),
		gotui.OnPreemptStop(gotui.KeyEscape, func(gotui.KeyEvent) { c.mcpManagerBack() }),
		gotui.OnPreemptStop(gotui.KeyCtrlC, func(gotui.KeyEvent) { c.mcpManagerBack() }),
	}
}

// mcpManagerBack is Pi's cancel: the inner screens return to the server,
// the server to the list, and the list closes the manager.
func (c *chatTUI) mcpManagerBack() {
	m := c.mcpManager
	if m.status[0] != "" {
		return // Pi's status screen takes no input
	}
	switch m.screen {
	case "servers":
		c.closeMCPManager()
		return
	case "server":
		m.screen = "servers"
	default:
		m.screen = "server"
	}
	c.markDirty()
}

func (c *chatTUI) mcpManagerConfirm() {
	m := c.mcpManager
	if m.status[0] != "" {
		return
	}
	menu := c.mcpManagerMenu()
	if len(menu.items) == 0 {
		return // an empty menu only cancels
	}
	value := menu.items[c.mcpManagerSelection(menu)].value
	switch m.screen {
	case "servers":
		m.screen, m.server = "server", value
		delete(m.selected, "server")
	case "tools":
		m.screen = "server"
	case "exposure":
		m.screen = "server"
		st, _ := mcpStatusOf(func() []gimcp.Status { s, _ := c.engine.MCPStatus(); return s }(), m.server)
		if value != mcpExposureOf(st) {
			c.runMCPManagerAction(m.server, "exposure:"+value)
		}
	case "server":
		c.runMCPManagerAction(m.server, value)
	}
	c.markDirty()
}

// runMCPManagerAction is Pi's runAction. Slow actions show Pi's status
// screen and run in the background, then return to the server's menu with
// the action's message, if any.
func (c *chatTUI) runMCPManagerAction(name, action string) {
	m := c.mcpManager
	delete(m.messages, name)
	switch action {
	case "tools":
		m.screen = "tools"
		delete(m.selected, "tools")
		return
	case "exposure":
		m.screen = "exposure"
		delete(m.selected, "exposure")
		return
	case "signin":
		// Pi signs in inside the manager; gi uses its in-session sign-in
		// (link, browser, pasted redirect URL in the editor).
		c.closeMCPManager()
		c.appendTranscript(c.startMCPSignIn(name)...)
		return
	}
	status := map[string]string{"reconnect": "Reconnecting…", "enable": "Connecting…", "disable": "Disconnecting…"}[action]
	if status != "" {
		m.status = [2]string{"MCP server " + name, status}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		var err error
		switch {
		case action == "reconnect":
			_ = c.engine.MCPReconnect(ctx, name) // a failure shows as the server's state and error
		case action == "signout":
			c.engine.MCPSignOut(ctx, name)
		case action == "enable" || action == "disable":
			err = c.engine.MCPSetEnabled(ctx, name, action == "enable")
		case strings.HasPrefix(action, "exposure:"):
			err = c.engine.MCPSetExposure(ctx, name, strings.TrimPrefix(action, "exposure:"))
		}
		c.runOnUI(func() {
			if c.mcpManager != m {
				return
			}
			m.status = [2]string{}
			if err != nil {
				m.messages[name] = err.Error()
			}
		})
	}()
}

// mcpFrame ports ui.js frame(): accent borders around an accent title, the
// body and a dim footer. Text lines have one column of padding and wrap.
func mcpFrame(width int, title string, body spanRows, footer []gotui.TextSpan) spanRows {
	rows := spanRows{piRule(width, piAccent)}
	rows = append(rows, piPaddedText(width, gotui.TextSpan{Text: title, Style: piFg(piAccent).Bold()})...)
	rows = append(rows, body...)
	if footer != nil {
		rows = append(rows, nil)
		rows = append(rows, piPaddedText(width, footer...)...)
	}
	return append(rows, piRule(width, piAccent))
}

// piPaddedText is a pi-tui Text with paddingX 1: each line of the text wraps
// to the width less the padding.
func piPaddedText(width int, spans ...gotui.TextSpan) spanRows {
	var lines [][]gotui.TextSpan
	var line []gotui.TextSpan
	for _, span := range spans {
		parts := strings.Split(span.Text, "\n")
		for i, part := range parts {
			if i > 0 {
				lines, line = append(lines, line), nil
			}
			if part != "" {
				line = append(line, gotui.TextSpan{Text: part, Style: span.Style})
			}
		}
	}
	lines = append(lines, line)
	var rows spanRows
	for _, l := range lines {
		for _, wrapped := range piWrapLine(l, max(1, width-2)) {
			rows = append(rows, append([]gotui.TextSpan{{Text: " "}}, wrapped...))
		}
	}
	return rows
}

func mcpFooter(confirm, cancel string) []gotui.TextSpan {
	spans := append(keyHintSpans("enter", confirm), gotui.TextSpan{Text: " • ", Style: piFg(piDim)})
	return append(spans, keyHintSpans("escape/ctrl+c", cancel)...)
}

// piMCPManagerRows renders the manager's current screen.
func (c *chatTUI) piMCPManagerRows(width int) spanRows {
	m := c.mcpManager
	if m.status[0] != "" {
		return mcpStatusRows(width, m.status[0], m.status[1])
	}
	menu := c.mcpManagerMenu()
	return mcpMenuRows(width, menu, c.mcpManagerSelection(menu))
}

// mcpStatusRows is McpManagerView.status: a title and a muted message.
func mcpStatusRows(width int, title, message string) spanRows {
	return mcpFrame(width, title, append(spanRows{nil}, piPaddedText(width, gotui.TextSpan{Text: message, Style: piFg(piMuted)})...), nil)
}

// mcpMenuRows is McpManagerView.menu's rendering of one menu.
func mcpMenuRows(width int, menu mcpMenu, selected int) spanRows {
	var body spanRows
	if menu.details != "" {
		body = append(body, piPaddedText(width, gotui.TextSpan{Text: menu.details, Style: piFg(piMuted)})...)
	}
	if menu.errorText != "" {
		body = append(body, piPaddedText(width, gotui.TextSpan{Text: menu.errorText, Style: piFg(piError)})...)
	}
	body = append(body, nil)
	if len(menu.items) == 0 {
		empty := menu.empty
		if empty == "" {
			empty = "Nothing to show."
		}
		body = append(body, piPaddedText(width, gotui.TextSpan{Text: empty, Style: piFg(piMuted)})...)
		return mcpFrame(width, menu.title, body, keyHintSpans("escape/ctrl+c", menu.cancelLabel))
	}
	body = append(body, selectListRows(menu.items, selected, width, min(len(menu.items), mcpManagerMaxVisible), 32, 32)...)
	return mcpFrame(width, menu.title, body, mcpFooter(menu.confirmLabel, menu.cancelLabel))
}

// selectListRows ports pi-tui SelectList.render with a primary column
// between minPrimary and maxPrimary cells.
func selectListRows(items []slashItem, selected, width, maxVisible, minPrimary, maxPrimary int) spanRows {
	n := len(items)
	if n == 0 {
		return spanRows{{{Text: "  No matching commands", Style: piFg(piMuted)}}}
	}
	widest := 0
	for _, it := range items {
		widest = max(widest, gotui.StringWidth(it.name)+slashPrimaryGap)
	}
	primary := max(minPrimary, min(widest, maxPrimary))
	start := max(0, min(selected-maxVisible/2, n-maxVisible))
	end := min(start+maxVisible, n)
	var rows spanRows
	for i := start; i < end; i++ {
		prefix := "  "
		if i == selected {
			prefix = "→ "
		}
		desc := strings.Join(strings.Fields(items[i].description), " ")
		if desc != "" && width > 40 {
			col := max(1, min(primary, width-2-4))
			value := truncateCells(items[i].name, max(1, col-slashPrimaryGap))
			spacing := strings.Repeat(" ", max(1, col-gotui.StringWidth(value)))
			if remaining := width - (2 + gotui.StringWidth(value) + len(spacing)) - 2; remaining > slashMinDescription {
				desc = truncateCells(desc, remaining)
				if i == selected {
					rows = append(rows, []gotui.TextSpan{{Text: prefix + value + spacing + desc, Style: piFg(piAccent)}})
				} else {
					rows = append(rows, []gotui.TextSpan{{Text: prefix + value}, {Text: spacing + desc, Style: piFg(piMuted)}})
				}
				continue
			}
		}
		value := truncateCells(items[i].name, max(1, width-2-2))
		if i == selected {
			rows = append(rows, []gotui.TextSpan{{Text: prefix + value, Style: piFg(piAccent)}})
		} else {
			rows = append(rows, []gotui.TextSpan{{Text: prefix + value}})
		}
	}
	if start > 0 || end < n {
		rows = append(rows, []gotui.TextSpan{{Text: truncateCells(fmt.Sprintf("  (%d/%d)", selected+1, n), max(0, width-2)), Style: piFg(piMuted)}})
	}
	return rows
}
