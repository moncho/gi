// Package plan ports Piclaw's plan-sidebar Markdown and mutation rules.
package plan

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var ErrInvalid = errors.New("invalid plan")

const MaxBytes = 256 << 10
const DefaultMarkdown = "- [ ] Update this plan thoroughly with ongoing work\n- [ ] Clarify the current objective\n- [ ] Do the next concrete step\n- [ ] Verify the result\n- [ ] Report progress and next step"

type Item struct {
	Step   string `json:"step"`
	Status string `json:"status"`
}
type Parsed struct {
	Explanation *string `json:"explanation"`
	Plan        []Item  `json:"plan"`
}
type Edit struct {
	Operation  string  `json:"operation,omitempty"`
	OldText    *string `json:"oldText,omitempty"`
	NewText    *string `json:"newText,omitempty"`
	AnchorText *string `json:"anchorText,omitempty"`
	Text       *string `json:"text,omitempty"`
}
type Patch struct {
	Operation string  `json:"operation"`
	Index     *int    `json:"index,omitempty"`
	Match     string  `json:"match,omitempty"`
	Step      *string `json:"step,omitempty"`
	Status    *string `json:"status,omitempty"`
	Position  string  `json:"position,omitempty"`
	Before    string  `json:"before,omitempty"`
	After     string  `json:"after,omitempty"`
}
type Mutation struct {
	Action      string  `json:"action"`
	Markdown    *string `json:"markdown,omitempty"`
	Explanation *string `json:"explanation,omitempty"`
	Plan        []Item  `json:"plan,omitempty"`
	Edits       []Edit  `json:"edits,omitempty"`
	Patches     []Patch `json:"patches,omitempty"`
	ChatJID     string  `json:"chat_jid,omitempty"`
}

var checklist = regexp.MustCompile(`^(\s*(?:[-*+]|[0-9]+[.)])\s+)\[([ xX-])\](\s*)(.*)$`)
var whitespace = regexp.MustCompile(`\s+`)

func step(s string) string        { return whitespace.ReplaceAllString(strings.TrimSpace(s), " ") }
func lineEndings(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
func markerStatus(marker string) string {
	switch strings.ToLower(marker) {
	case "x":
		return "completed"
	case "-":
		return "in_progress"
	default:
		return "pending"
	}
}
func statusMarker(status string) (string, error) {
	switch status {
	case "pending":
		return " ", nil
	case "in_progress":
		return "-", nil
	case "completed":
		return "x", nil
	default:
		return "", fmt.Errorf("invalid plan status %q", status)
	}
}
func Normalize(markdown string) (string, error) {
	if len(markdown) > MaxBytes {
		return "", errors.New("plan exceeds 256 KiB")
	}
	lines := strings.Split(lineEndings(markdown), "\n")
	active := 0
	for i, line := range lines {
		m := checklist.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		status := markerStatus(m[2])
		if status == "in_progress" {
			active++
		}
		marker, _ := statusMarker(status)
		spacing := m[3]
		if spacing == "" {
			spacing = " "
		}
		lines[i] = m[1] + "[" + marker + "]" + spacing + m[4]
	}
	if active > 1 {
		return "", errors.New("Plan Markdown can contain at most one in-progress checklist item (`[-]`).")
	}
	return strings.Join(lines, "\n"), nil
}
func Parse(markdown string) Parsed {
	out := Parsed{Plan: []Item{}}
	var notes []string
	sawTask := false
	for _, line := range strings.Split(lineEndings(markdown), "\n") {
		if m := checklist.FindStringSubmatch(line); m != nil {
			sawTask = true
			out.Plan = append(out.Plan, Item{step(m[4]), markerStatus(m[2])})
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !sawTask && strings.HasPrefix(trimmed, ">") {
			notes = append(notes, strings.TrimPrefix(strings.TrimPrefix(trimmed, ">"), " "))
		}
	}
	if note := strings.TrimSpace(strings.Join(notes, "\n")); note != "" {
		out.Explanation = &note
	}
	return out
}
func Render(explanation *string, items []Item) (string, error) {
	var lines []string
	active := 0
	if explanation != nil && strings.TrimSpace(*explanation) != "" {
		for _, line := range strings.Split(lineEndings(strings.TrimSpace(*explanation)), "\n") {
			lines = append(lines, strings.TrimRight("> "+strings.TrimSpace(line), " "))
		}
		lines = append(lines, "")
	}
	for i, item := range items {
		text := step(item.Step)
		if text == "" {
			return "", fmt.Errorf("plan update item %d requires a non-empty step", i+1)
		}
		marker, err := statusMarker(item.Status)
		if err != nil {
			return "", err
		}
		if item.Status == "in_progress" {
			active++
		}
		lines = append(lines, "- ["+marker+"] "+text)
	}
	if active > 1 {
		return "", errors.New("plan update accepts at most one in_progress step")
	}
	return Normalize(strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n"))
}
func textValue(primary, fallback *string, field string, allowEmpty bool) (string, error) {
	value := primary
	if value == nil {
		value = fallback
	}
	if value == nil || (!allowEmpty && *value == "") {
		return "", fmt.Errorf("plan edit %s must be a non-empty string", field)
	}
	return *value, nil
}
func EditMarkdown(markdown string, edits []Edit) (string, error) {
	if len(edits) == 0 || len(edits) > 100 {
		return "", errors.New("plan edit requires 1-100 edit blocks")
	}
	markdown = lineEndings(markdown)
	type span struct {
		from, to, order int
		text            string
	}
	ranges := []span{}
	for i, edit := range edits {
		op := edit.Operation
		if op == "" && edit.OldText != nil && edit.NewText != nil {
			op = "replace"
		}
		switch op {
		case "append", "prepend":
			text, err := textValue(edit.Text, edit.NewText, "text", false)
			if err != nil {
				return "", err
			}
			at := 0
			if op == "append" {
				at = len(markdown)
			}
			ranges = append(ranges, span{at, at, i, text})
			continue
		case "replace", "delete", "insert_after", "insert_before":
		default:
			return "", errors.New("invalid plan edit operation")
		}
		field := "anchorText"
		if op == "replace" || op == "delete" {
			field = "oldText"
		}
		anchor, err := textValue(edit.AnchorText, edit.OldText, field, false)
		if err != nil {
			return "", err
		}
		if strings.Count(markdown, anchor) != 1 {
			return "", fmt.Errorf("Plan edit %s must match exactly once", field)
		}
		from := strings.Index(markdown, anchor)
		to := from + len(anchor)
		replacement := ""
		if op == "replace" {
			replacement, err = textValue(edit.NewText, nil, "newText", true)
		} else if op != "delete" {
			replacement, err = textValue(edit.Text, edit.NewText, "text", false)
			if op == "insert_before" {
				to = from
			} else {
				from = to
			}
		}
		if err != nil {
			return "", err
		}
		ranges = append(ranges, span{from, to, i, replacement})
	}
	sort.SliceStable(ranges, func(i, j int) bool { return ranges[i].from < ranges[j].from })
	var out strings.Builder
	cursor := 0
	for _, r := range ranges {
		if r.from < cursor {
			return "", errors.New("Plan edit blocks must not overlap")
		}
		out.WriteString(markdown[cursor:r.from])
		out.WriteString(r.text)
		cursor = r.to
	}
	out.WriteString(markdown[cursor:])
	return Normalize(out.String())
}
func target(items []Item, index *int, match string) (int, error) {
	if index != nil {
		if *index < 1 || *index > len(items) {
			return 0, fmt.Errorf("plan patch index %d is out of range", *index)
		}
		return *index - 1, nil
	}
	match = step(match)
	if match == "" {
		return 0, errors.New("plan patch needs index or match")
	}
	found := -1
	for i, item := range items {
		if strings.Contains(item.Step, match) {
			if found != -1 {
				return 0, errors.New("plan patch match must identify exactly one item")
			}
			found = i
		}
	}
	if found == -1 {
		return 0, errors.New("plan patch match must identify exactly one item")
	}
	return found, nil
}
func PatchMarkdown(markdown string, patches []Patch) (string, error) {
	if len(patches) == 0 || len(patches) > 100 {
		return "", errors.New("plan patch requires 1-100 patch blocks")
	}
	parsed := Parse(markdown)
	items := parsed.Plan
	for _, p := range patches {
		if p.Position != "" && p.Position != "start" && p.Position != "end" {
			return "", errors.New("plan patch position must be start or end")
		}
		switch p.Operation {
		case "add":
			if p.Step == nil || step(*p.Step) == "" {
				return "", errors.New("plan patch add requires step")
			}
			status := "pending"
			if p.Status != nil {
				status = *p.Status
			}
			if _, err := statusMarker(status); err != nil {
				return "", err
			}
			at := len(items)
			var err error
			switch {
			case p.Position == "start":
				at = 0
			case p.Position == "end":
			case step(p.Before) != "":
				at, err = target(items, nil, p.Before)
			case step(p.After) != "":
				at, err = target(items, nil, p.After)
				at++
			}
			if err != nil {
				return "", err
			}
			items = append(items, Item{})
			copy(items[at+1:], items[at:])
			items[at] = Item{step(*p.Step), status}
		case "update", "remove":
			at, err := target(items, p.Index, p.Match)
			if err != nil {
				return "", err
			}
			if p.Operation == "remove" {
				items = append(items[:at], items[at+1:]...)
				continue
			}
			if p.Step != nil {
				if step(*p.Step) == "" {
					return "", errors.New("plan patch update step must not be empty")
				}
				items[at].Step = step(*p.Step)
			}
			if p.Status != nil {
				if _, err = statusMarker(*p.Status); err != nil {
					return "", err
				}
				items[at].Status = *p.Status
			}
		default:
			return "", errors.New("plan patch requires add, update, or remove")
		}
	}
	return Render(parsed.Explanation, items)
}
func Apply(markdown string, m Mutation) (string, error) {
	switch m.Action {
	case "write":
		if m.Markdown == nil {
			return "", errors.New("plan action=write requires a markdown string")
		}
		return Normalize(*m.Markdown)
	case "reset":
		return DefaultMarkdown, nil
	case "update":
		if m.Plan == nil {
			return "", errors.New("plan action=update requires a plan array")
		}
		return Render(m.Explanation, m.Plan)
	case "edit":
		return EditMarkdown(markdown, m.Edits)
	case "patch":
		return PatchMarkdown(markdown, m.Patches)
	default:
		return "", fmt.Errorf("invalid plan mutation action %q", m.Action)
	}
}
