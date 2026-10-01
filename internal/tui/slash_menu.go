package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	gotui "github.com/grindlemire/go-tui"
)

// Pi's editor opens a slash-command autocomplete list when "/" is typed at
// the start of the message (pi-tui Editor + CombinedAutocompleteProvider +
// SelectList). It is rendered under the editor's bottom border, fuzzy filters
// as the command name is typed, closes at the first space, and:
//
//	Up/Down  move the selection (wrapping)
//	Tab      complete to "/name "
//	Enter    complete to "/name " and submit
//	Escape   close the list

const (
	slashMaxVisible     = 5  // Editor autocompleteMaxVisible default
	slashMinPrimaryCol  = 12 // SLASH_COMMAND_SELECT_LIST_LAYOUT
	slashMaxPrimaryCol  = 32
	slashPrimaryGap     = 2
	slashMinDescription = 10
)

type slashItem struct {
	name, description string
}

type slashMenu struct {
	active   bool
	items    []slashItem
	selected int
}

var slashArgHint = regexp.MustCompile(`^/([^\s]+)\s*(.*)$`)

// slashCommandItems lists commands as Pi's provider does: built-ins in
// catalogue order, then extension commands, then skills ("skill:name").
// Descriptions are "<hint> — <description>" when a command takes arguments.
func (c *chatTUI) slashCommandItems() []slashItem {
	items := make([]slashItem, 0, len(tuiCommands)+8)
	seen := map[string]bool{}
	add := func(name, hint, desc string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		full := desc
		if hint = strings.TrimSpace(hint); hint != "" {
			if desc != "" {
				full = hint + " — " + desc
			} else {
				full = hint
			}
		}
		items = append(items, slashItem{name: name, description: full})
	}
	for _, cmd := range tuiCommands {
		m := slashArgHint.FindStringSubmatch(cmd.name)
		if m == nil || strings.HasPrefix(m[1], "skill:") {
			continue
		}
		add(m[1], m[2], cmd.hint)
	}
	if c.engine != nil {
		for _, cmd := range c.engine.ExtensionCommandInfos() {
			add(cmd.Name, "", strings.TrimSpace(cmd.Description))
		}
	}
	for _, skill := range c.cfg.Discovery.Skills {
		add("skill:"+skill.Name, "", strings.TrimSpace(skill.Description))
	}
	return items
}

// piFuzzyMatch ports pi-tui fuzzy.js fuzzyMatch (lower score is better).
func piFuzzyMatch(query, text string) (bool, float64) {
	q, t := []rune(strings.ToLower(query)), []rune(strings.ToLower(text))
	match := func(q []rune) (bool, float64) {
		if len(q) == 0 {
			return true, 0
		}
		if len(q) > len(t) {
			return false, 0
		}
		score, last, consecutive, qi := 0.0, -1, 0, 0
		for qi < len(q) {
			i := -1
			for j := last + 1; j < len(t); j++ {
				if t[j] == q[qi] {
					i = j
					break
				}
			}
			if i == -1 {
				break
			}
			boundary := i == 0 || strings.ContainsRune(" \t\n-_./:", t[i-1])
			if last == i-1 {
				consecutive++
				score -= float64(consecutive * 5)
			} else {
				consecutive = 0
				if last >= 0 {
					score += float64((i - last - 1) * 2)
				}
			}
			if boundary {
				score -= 10
			}
			score += float64(i) * 0.1
			last = i
			qi++
		}
		if qi < len(q) {
			return false, 0
		}
		if string(q) == string(t) {
			score -= 100
		}
		return true, score
	}
	if ok, score := match(q); ok {
		return true, score
	}
	if m := regexp.MustCompile(`^([a-z]+)([0-9]+)$`).FindStringSubmatch(string(q)); m != nil {
		if ok, score := match([]rune(m[2] + m[1])); ok {
			return true, score + 5
		}
	} else if m := regexp.MustCompile(`^([0-9]+)([a-z]+)$`).FindStringSubmatch(string(q)); m != nil {
		if ok, score := match([]rune(m[2] + m[1])); ok {
			return true, score + 5
		}
	}
	return false, 0
}

// piFuzzyFilter ports fuzzyFilter: every whitespace/slash token must match;
// results are ordered by total score (stable for ties).
func piFuzzyFilter(items []slashItem, query string, text func(slashItem) string) []slashItem {
	tokens := strings.FieldsFunc(strings.TrimSpace(query), func(r rune) bool { return r == '/' || r == ' ' || r == '\t' })
	if len(tokens) == 0 {
		return items
	}
	type scored struct {
		item  slashItem
		score float64
	}
	var results []scored
	for _, item := range items {
		total, all := 0.0, true
		for _, token := range tokens {
			ok, score := piFuzzyMatch(token, text(item))
			if !ok {
				all = false
				break
			}
			total += score
		}
		if all {
			results = append(results, scored{item, total})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].score < results[j].score })
	out := make([]slashItem, len(results))
	for i, r := range results {
		out[i] = r.item
	}
	return out
}

func (c *chatTUI) slashMatches(prefix string) []slashItem {
	items := c.slashCommandItems()
	bare := piFuzzyFilter(items, prefix, func(it slashItem) string { return strings.TrimPrefix(it.name, "skill:") })
	inBare := map[string]bool{}
	for _, it := range bare {
		inBare[it.name] = true
	}
	var skillsOnly []slashItem
	for _, it := range items {
		if strings.HasPrefix(it.name, "skill:") && !inBare[it.name] {
			skillsOnly = append(skillsOnly, it)
		}
	}
	return append(bare, piFuzzyFilter(skillsOnly, prefix, func(it slashItem) string { return it.name })...)
}

// slashPrefix returns the command name typed so far, when the draft is a
// single-line slash command with no argument yet. Leading whitespace is
// allowed, as in Pi (pi-tui: textBeforeCursor.trimStart()).
func slashPrefix(text string, cursor int) (string, bool) {
	runes := []rune(text)
	cursor = max(0, min(cursor, len(runes)))
	before := strings.TrimLeft(string(runes[:cursor]), " \t")
	if !strings.HasPrefix(before, "/") || strings.ContainsAny(before, " \t\n") || strings.Contains(text, "\n") {
		return "", false
	}
	return strings.TrimPrefix(before, "/"), true
}

// slashLead is the whitespace before the command, kept when a completion is
// applied (Pi's beforePrefix).
func slashLead(text string) string {
	return text[:len(text)-len(strings.TrimLeft(text, " \t"))]
}

// updateSlashMenu follows Pi: typing "/" as the first character opens the
// list; later edits refilter it; a space, leaving the slash prefix, or no
// matches closes it.
func (c *chatTUI) updateSlashMenu(previous string) {
	if c.input == nil {
		return
	}
	text := c.input.Text()
	prefix, ok := slashPrefix(text, c.input.cursorPos)
	if !c.slash.active {
		// Pi opens the list when "/" is typed at the start of the message,
		// after nothing but whitespace.
		if !ok || strings.TrimLeft(text, " \t") != "/" || strings.TrimSpace(previous) != "" {
			return
		}
	}
	if !ok {
		c.slash = slashMenu{}
		return
	}
	items := c.slashMatches(prefix)
	if len(items) == 0 {
		c.slash = slashMenu{}
		return
	}
	c.slash = slashMenu{active: true, items: items}
}

func (c *chatTUI) moveSlashSelection(delta int) {
	n := len(c.slash.items)
	if n == 0 {
		return
	}
	c.slash.selected = ((c.slash.selected+delta)%n + n) % n
	c.markDirty()
}

// applySlashSelection replaces the command prefix with "/name " (Pi's
// applyCompletion) and optionally submits it.
func (c *chatTUI) applySlashSelection(submit bool) {
	if !c.slash.active || len(c.slash.items) == 0 {
		return
	}
	item := c.slash.items[c.slash.selected]
	runes := []rune(c.input.Text())
	cursor := max(0, min(c.input.cursorPos, len(runes)))
	after := string(runes[cursor:])
	c.slash = slashMenu{}
	c.input.snapshotUndo()
	lead := slashLead(c.input.text)
	c.input.text = lead + "/" + item.name + " " + after
	c.input.cursorPos = len([]rune(lead)) + len([]rune(item.name)) + 2
	c.input.notifyChanged()
	if submit {
		c.input.enter(gotui.KeyEvent{Key: gotui.KeyEnter})
	}
	c.markDirty()
}

func (c *chatTUI) closeSlashMenu() {
	c.slash = slashMenu{}
	c.markDirty()
}

func (c *chatTUI) markDirty() {
	if c.app != nil {
		c.app.MarkDirty()
	}
}

// handleSlashKey is the editor's interceptKey hook.
func (c *chatTUI) handleSlashKey(key gotui.Key) bool {
	if !c.slash.active {
		return false
	}
	switch key {
	case gotui.KeyTab:
		c.applySlashSelection(false)
	case gotui.KeyEnter:
		c.applySlashSelection(true)
	case gotui.KeyEscape:
		c.closeSlashMenu()
	default:
		return false
	}
	return true
}

func (c *chatTUI) slashMenuKeys() gotui.KeyMap {
	if !c.slash.active {
		return nil
	}
	return gotui.KeyMap{
		gotui.OnPreemptStop(gotui.KeyUp, func(gotui.KeyEvent) { c.moveSlashSelection(-1) }),
		gotui.OnPreemptStop(gotui.KeyDown, func(gotui.KeyEvent) { c.moveSlashSelection(1) }),
		gotui.OnPreemptStop(gotui.KeyTab, func(gotui.KeyEvent) { c.applySlashSelection(false) }),
		gotui.OnPreemptStop(gotui.KeyEnter, func(gotui.KeyEvent) { c.applySlashSelection(true) }),
		gotui.OnPreemptStop(gotui.KeyEscape, func(gotui.KeyEvent) { c.closeSlashMenu() }),
	}
}

func (c *chatTUI) slashVisibleRange() (int, int) {
	n := len(c.slash.items)
	start := max(0, min(c.slash.selected-slashMaxVisible/2, n-slashMaxVisible))
	return start, min(start+slashMaxVisible, n)
}

func (c *chatTUI) slashMenuHeight() int {
	if !c.slash.active {
		return 0
	}
	start, end := c.slashVisibleRange()
	rows := end - start
	if start > 0 || end < len(c.slash.items) {
		rows++
	}
	return rows
}

// slashMenuRows ports SelectList.render/renderItem for the slash layout.
func (c *chatTUI) slashMenuRows(width int) [][]gotui.TextSpan {
	if !c.slash.active {
		return nil
	}
	widest := 0
	for _, it := range c.slash.items {
		widest = max(widest, gotui.StringWidth(it.name)+slashPrimaryGap)
	}
	primary := max(slashMinPrimaryCol, min(widest, slashMaxPrimaryCol))
	accent, muted := piFg(piAccent), piFg(piMuted)
	start, end := c.slashVisibleRange()
	var rows [][]gotui.TextSpan
	for i := start; i < end; i++ {
		it := c.slash.items[i]
		selected := i == c.slash.selected
		prefix := "  "
		if selected {
			prefix = "→ "
		}
		desc := strings.Join(strings.Fields(it.description), " ")
		if desc != "" && width > 40 {
			col := max(1, min(primary, width-2-4))
			value := truncateCells(it.name, max(1, col-slashPrimaryGap))
			spacing := strings.Repeat(" ", max(1, col-gotui.StringWidth(value)))
			remaining := width - (2 + gotui.StringWidth(value) + len(spacing)) - 2
			if remaining > slashMinDescription {
				desc = truncateCells(desc, remaining)
				if selected {
					rows = append(rows, []gotui.TextSpan{{Text: prefix + value + spacing + desc, Style: accent}})
				} else {
					rows = append(rows, []gotui.TextSpan{{Text: prefix + value}, {Text: spacing + desc, Style: muted}})
				}
				continue
			}
		}
		value := truncateCells(it.name, max(1, width-2-2))
		if selected {
			rows = append(rows, []gotui.TextSpan{{Text: prefix + value, Style: accent}})
		} else {
			rows = append(rows, []gotui.TextSpan{{Text: prefix + value}})
		}
	}
	if start > 0 || end < len(c.slash.items) {
		rows = append(rows, []gotui.TextSpan{{Text: truncateCells(fmt.Sprintf("  (%d/%d)", c.slash.selected+1, len(c.slash.items)), max(0, width-2)), Style: muted}})
	}
	return rows
}

func (c *chatTUI) renderSlashMenu(width int) *gotui.Element {
	rows := c.slashMenuRows(width)
	block := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidthPercent(100), gotui.WithHeight(len(rows)))
	for _, spans := range rows {
		block.AddChild(gotui.New(gotui.WithWidthPercent(100), gotui.WithHeight(1), gotui.WithWrap(false), gotui.WithRichText(spans...)))
	}
	return block
}
