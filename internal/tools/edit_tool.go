package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	goai "github.com/rcarmo/go-ai"
	"golang.org/x/text/unicode/norm"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/textdiff"
)

// Pi's edit tool (core/tools/edit.js, edit-diff.js): exact-text replacements
// matched against the original file, a fuzzy fallback for whitespace and
// typographic variants, and a numbered display diff for the TUI.

const EditToolDescription = "Edit a single file using exact text replacement. Every edits[].oldText must match a unique, non-overlapping region of the original file. If two changes affect the same block or nearby lines, merge them into one edit instead of emitting overlapping edits. Do not include large unchanged regions just to connect distant changes."

const EditToolParameters = `{"type":"object","properties":{"path":{"type":"string","description":"Path to the file to edit (relative or absolute)"},"edits":{"type":"array","items":{"type":"object","properties":{"oldText":{"type":"string","description":"Exact text for one targeted replacement. It must be unique in the original file and must not overlap with any other edits[].oldText in the same call."},"newText":{"type":"string","description":"Replacement text for this targeted edit."}},"required":["oldText","newText"]},"description":"One or more targeted replacements. Each edit is matched against the original file, not incrementally. Do not include overlapping or nested edits. If two changes touch the same block or nearby lines, merge them into one edit instead."}},"required":["path","edits"]}`

// Edit is one replacement.
type Edit struct {
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

func asEdit(v any) (Edit, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return Edit{}, false
	}
	o, ok1 := m["oldText"].(string)
	n, ok2 := m["newText"].(string)
	return Edit{o, n}, ok1 && ok2
}

// EditArguments is Pi's prepareEditArguments plus the renderer's preview
// input: edits sent as a JSON string or a single object, and the legacy
// top-level oldText/newText. ok is false when the edits are not all
// {oldText,newText} strings.
func EditArguments(args map[string]any) (path string, edits []Edit, ok bool) {
	path, _ = args["path"].(string)
	if path == "" {
		path, _ = args["file_path"].(string)
	}
	raw := args["edits"]
	if s, isString := raw.(string); isString {
		var parsed any
		if json.Unmarshal([]byte(s), &parsed) == nil {
			if _, isList := parsed.([]any); isList {
				raw = parsed
			} else if _, single := asEdit(parsed); single {
				raw = []any{parsed}
			}
		}
	} else if _, single := asEdit(raw); single {
		raw = []any{raw}
	}
	list, _ := raw.([]any)
	ok = true
	for _, item := range list {
		e, valid := asEdit(item)
		if !valid {
			ok = false
		}
		edits = append(edits, e)
	}
	o, ok1 := args["oldText"].(string)
	n, ok2 := args["newText"].(string)
	if ok1 && ok2 {
		edits = append(edits, Edit{o, n})
	}
	return path, edits, ok && len(edits) > 0
}

func splitBom(content string) (bom, text string) {
	if strings.HasPrefix(content, "\uFEFF") {
		return "\uFEFF", content[len("\uFEFF"):]
	}
	return "", content
}

func detectLineEnding(content string) string {
	crlf, lf := strings.Index(content, "\r\n"), strings.IndexByte(content, '\n')
	if lf == -1 || crlf == -1 {
		return "\n"
	}
	if crlf < lf {
		return "\r\n"
	}
	return "\n"
}

func normalizeToLF(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

func restoreLineEndings(text, ending string) string {
	if ending == "\r\n" {
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	return text
}

// isJSSpace is JavaScript's \s, which trimEnd removes.
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

var fuzzyReplacer = strings.NewReplacer(
	"\u2018", "'", "\u2019", "'", "\u201A", "'", "\u201B", "'",
	"\u201C", `"`, "\u201D", `"`, "\u201E", `"`, "\u201F", `"`,
	"\u2010", "-", "\u2011", "-", "\u2012", "-", "\u2013", "-", "\u2014", "-", "\u2015", "-", "\u2212", "-",
)

// normalizeForFuzzyMatch: NFKC, trailing whitespace stripped per line, smart
// quotes, Unicode dashes and special spaces to ASCII.
func normalizeForFuzzyMatch(text string) string {
	lines := strings.Split(norm.NFKC.String(text), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRightFunc(line, isJSSpace)
	}
	text = fuzzyReplacer.Replace(strings.Join(lines, "\n"))
	return strings.Map(func(r rune) rune {
		if r == 0xa0 || r >= 0x2002 && r <= 0x200a || r == 0x202f || r == 0x205f || r == 0x3000 {
			return ' '
		}
		return r
	}, text)
}

type fuzzyMatch struct {
	found, fuzzy  bool
	index, length int
}

func fuzzyFindText(content, oldText string) fuzzyMatch {
	if i := strings.Index(content, oldText); i != -1 {
		return fuzzyMatch{found: true, index: i, length: len(oldText)}
	}
	fuzzyContent, fuzzyOld := normalizeForFuzzyMatch(content), normalizeForFuzzyMatch(oldText)
	if i := strings.Index(fuzzyContent, fuzzyOld); i != -1 {
		return fuzzyMatch{found: true, fuzzy: true, index: i, length: len(fuzzyOld)}
	}
	return fuzzyMatch{index: -1}
}

type matchedEdit struct {
	editIndex, matchIndex, matchLength int
	newText                            string
}

var linesWithEndings = regexp.MustCompile(`[^\n]*\n|[^\n]+`)

func applyReplacements(content string, replacements []matchedEdit, offset int) string {
	for i := len(replacements) - 1; i >= 0; i-- {
		r := replacements[i]
		at := r.matchIndex - offset
		content = content[:at] + r.newText + content[at+r.matchLength:]
	}
	return content
}

// applyReplacementsPreservingUnchangedLines rewrites only the lines the
// fuzzy replacements touch; every other line keeps its original bytes.
func applyReplacementsPreservingUnchangedLines(original, base string, replacements []matchedEdit) (string, error) {
	originalLines := linesWithEndings.FindAllString(original, -1)
	type span struct{ start, end int }
	var baseLines []span
	for _, loc := range linesWithEndings.FindAllStringIndex(base, -1) {
		baseLines = append(baseLines, span{loc[0], loc[1]})
	}
	if len(originalLines) != len(baseLines) {
		return "", errors.New("Cannot preserve unchanged lines because the base content has a different line count.")
	}
	type group struct {
		startLine, endLine int
		replacements       []matchedEdit
	}
	var groups []*group
	sorted := append([]matchedEdit(nil), replacements...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].matchIndex < sorted[j].matchIndex })
	for _, r := range sorted {
		start, end := r.matchIndex, r.matchIndex+r.matchLength
		startLine := -1
		for i, l := range baseLines {
			if start >= l.start && start < l.end {
				startLine = i
				break
			}
		}
		if startLine == -1 {
			return "", errors.New("Replacement range is outside the base content.")
		}
		endLine := startLine
		for endLine < len(baseLines) && baseLines[endLine].end < end {
			endLine++
		}
		if endLine >= len(baseLines) {
			return "", errors.New("Replacement range is outside the base content.")
		}
		if n := len(groups); n > 0 && startLine < groups[n-1].endLine {
			groups[n-1].endLine = max(groups[n-1].endLine, endLine+1)
			groups[n-1].replacements = append(groups[n-1].replacements, r)
			continue
		}
		groups = append(groups, &group{startLine, endLine + 1, []matchedEdit{r}})
	}
	var b strings.Builder
	next := 0
	for _, g := range groups {
		b.WriteString(strings.Join(originalLines[next:g.startLine], ""))
		from, to := baseLines[g.startLine].start, baseLines[g.endLine-1].end
		b.WriteString(applyReplacements(base[from:to], g.replacements, from))
		next = g.endLine
	}
	b.WriteString(strings.Join(originalLines[next:], ""))
	return b.String(), nil
}

func editError(path string, index, total int, single, multi string) error {
	if total == 1 {
		return fmt.Errorf(single, path)
	}
	return fmt.Errorf(multi, index, path)
}

// ApplyEdits is Pi's applyEditsToNormalizedContent: every edit is matched
// against the same LF-normalized content.
func ApplyEdits(normalized string, edits []Edit, path string) (base, updated string, err error) {
	n := make([]Edit, len(edits))
	for i, e := range edits {
		n[i] = Edit{normalizeToLF(e.OldText), normalizeToLF(e.NewText)}
	}
	for i, e := range n {
		if e.OldText == "" {
			return "", "", editError(path, i, len(n), "oldText must not be empty in %s.", "edits[%d].oldText must not be empty in %s.")
		}
	}
	fuzzy := false
	for _, e := range n {
		if fuzzyFindText(normalized, e.OldText).fuzzy {
			fuzzy = true
		}
	}
	replacementBase := normalized
	if fuzzy {
		replacementBase = normalizeForFuzzyMatch(normalized)
	}
	var matched []matchedEdit
	for i, e := range n {
		m := fuzzyFindText(replacementBase, e.OldText)
		if !m.found {
			return "", "", editError(path, i, len(n), "Could not find the exact text in %s. The old text must match exactly including all whitespace and newlines.", "Could not find edits[%d] in %s. The oldText must match exactly including all whitespace and newlines.")
		}
		if count := strings.Count(normalizeForFuzzyMatch(replacementBase), normalizeForFuzzyMatch(e.OldText)); count > 1 {
			if len(n) == 1 {
				return "", "", fmt.Errorf("Found %d occurrences of the text in %s. The text must be unique. Please provide more context to make it unique.", count, path)
			}
			return "", "", fmt.Errorf("Found %d occurrences of edits[%d] in %s. Each oldText must be unique. Please provide more context to make it unique.", count, i, path)
		}
		matched = append(matched, matchedEdit{i, m.index, m.length, e.NewText})
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].matchIndex < matched[j].matchIndex })
	for i := 1; i < len(matched); i++ {
		prev, cur := matched[i-1], matched[i]
		if prev.matchIndex+prev.matchLength > cur.matchIndex {
			return "", "", fmt.Errorf("edits[%d] and edits[%d] overlap in %s. Merge them into one edit or target disjoint regions.", prev.editIndex, cur.editIndex, path)
		}
	}
	if fuzzy {
		updated, err = applyReplacementsPreservingUnchangedLines(normalized, replacementBase, matched)
		if err != nil {
			return "", "", err
		}
	} else {
		updated = applyReplacements(replacementBase, matched, 0)
	}
	if normalized == updated {
		if len(n) == 1 {
			return "", "", fmt.Errorf("No changes made to %s. The replacement produced identical content. This might indicate an issue with special characters or the text not existing as expected.", path)
		}
		return "", "", fmt.Errorf("No changes made to %s. The replacements produced identical content.", path)
	}
	return normalized, updated, nil
}

// DiffString is Pi's generateDiffString: "+N line", "-N line", " N line"
// with 4 context lines and " ..." for skipped runs; firstChangedLine is in
// the new file (0 when nothing changed).
func DiffString(oldContent, newContent string) (string, int) {
	const contextLines = 4
	parts := textdiff.Lines(oldContent, newContent)
	width := len(fmt.Sprint(max(len(strings.Split(oldContent, "\n")), len(strings.Split(newContent, "\n")))))
	num := func(n int) string { return fmt.Sprintf("%*d", width, n) }
	skipped := " " + strings.Repeat(" ", width) + " ..."
	var out []string
	oldLine, newLine := 1, 1
	lastWasChange := false
	firstChanged := 0
	for i, part := range parts {
		raw := strings.Split(part.Value, "\n")
		if raw[len(raw)-1] == "" {
			raw = raw[:len(raw)-1]
		}
		if part.Added || part.Removed {
			if firstChanged == 0 {
				firstChanged = newLine
			}
			for _, line := range raw {
				if part.Added {
					out = append(out, "+"+num(newLine)+" "+line)
					newLine++
				} else {
					out = append(out, "-"+num(oldLine)+" "+line)
					oldLine++
				}
			}
			lastWasChange = true
			continue
		}
		context := func(lines []string) {
			for _, line := range lines {
				out = append(out, " "+num(oldLine)+" "+line)
				oldLine++
				newLine++
			}
		}
		skip := func(n int) {
			out = append(out, skipped)
			oldLine += n
			newLine += n
		}
		nextIsChange := i < len(parts)-1 && (parts[i+1].Added || parts[i+1].Removed)
		switch {
		case lastWasChange && nextIsChange:
			if len(raw) <= contextLines*2 {
				context(raw)
			} else {
				context(raw[:contextLines])
				skip(len(raw) - 2*contextLines)
				context(raw[len(raw)-contextLines:])
			}
		case lastWasChange:
			shown := raw[:min(contextLines, len(raw))]
			context(shown)
			if n := len(raw) - len(shown); n > 0 {
				skip(n)
			}
		case nextIsChange:
			n := max(0, len(raw)-contextLines)
			if n > 0 {
				skip(n)
			}
			context(raw[n:])
		default:
			oldLine += len(raw)
			newLine += len(raw)
		}
		lastWasChange = false
	}
	return strings.Join(out, "\n"), firstChanged
}

// nodeErrorCode is the Node error code Pi reports for a file it cannot use.
func nodeErrorCode(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "Error code: ENOENT"
	case errors.Is(err, fs.ErrPermission):
		return "Error code: EACCES"
	}
	return err.Error()
}

func readEditTarget(ctx context.Context, workspaceRoot string, s *store.Store, path string) (string, error) {
	resolved, err := ResolveToolPath(workspaceRoot, path, false)
	if err != nil {
		return "", fmt.Errorf("Could not edit file: %s. %s.", path, err.Error())
	}
	var raw []byte
	if resolved.IsVFS() {
		if s == nil {
			return "", fmt.Errorf("Could not edit file: %s. Error code: ENOENT.", path)
		}
		_, raw, err = s.GetVFSFileContent(ctx, resolved.VFSNamespace, resolved.VFSPath)
	} else {
		raw, err = os.ReadFile(resolved.WorkspacePath)
	}
	if err != nil {
		return "", fmt.Errorf("Could not edit file: %s. %s.", path, nodeErrorCode(err))
	}
	return string(raw), nil
}

// EditPreview is Pi's computeEditsDiff: the diff an edit would make, or the
// error it would fail with, without writing.
func EditPreview(ctx context.Context, workspaceRoot string, s *store.Store, path string, edits []Edit) (diff string, firstChanged int, err error) {
	raw, err := readEditTarget(ctx, workspaceRoot, s, path)
	if err != nil {
		return "", 0, err
	}
	_, content := splitBom(raw)
	base, updated, err := ApplyEdits(normalizeToLF(content), edits, path)
	if err != nil {
		return "", 0, err
	}
	diff, firstChanged = DiffString(base, updated)
	return diff, firstChanged, nil
}

var editLocks sync.Map // path → *sync.Mutex (Pi's withFileMutationQueue)

// ExecuteEditTool is Pi's edit tool.
func ExecuteEditTool(ctx context.Context, cfg config.RuntimeConfig, rt ToolRuntime, call goai.ToolCall) (string, error) {
	path, edits, _ := EditArguments(call.Arguments)
	if len(edits) == 0 {
		return "", errors.New("Edit tool input is invalid. edits must contain at least one replacement.")
	}
	lock, _ := editLocks.LoadOrStore(path, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	if ctx.Err() != nil {
		return "", errors.New("Operation aborted")
	}
	raw, err := readEditTarget(ctx, cfg.WorkspaceRoot, rt.Store, path)
	if err != nil {
		return "", err
	}
	bom, content := splitBom(raw)
	ending := detectLineEnding(content)
	base, updated, err := ApplyEdits(normalizeToLF(content), edits, path)
	if err != nil {
		return "", err
	}
	if ctx.Err() != nil {
		return "", errors.New("Operation aborted")
	}
	if err := WriteFile(ctx, cfg, rt.Store, path, bom+restoreLineEndings(updated, ending)); err != nil {
		return "", err
	}
	diff, firstChanged := DiffString(base, updated)
	if rt.SetDetails != nil {
		details := map[string]any{"diff": diff}
		if firstChanged > 0 {
			details["firstChangedLine"] = firstChanged
		}
		rt.SetDetails(details)
	}
	return fmt.Sprintf("Successfully replaced %d block(s) in %s.", len(edits), path), nil
}
