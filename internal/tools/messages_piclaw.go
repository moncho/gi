package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rcarmo/gi/internal/store"
)

// Piclaw-compatible messages actions (runtime/src/extensions/messages-crud.ts,
// single-user mode). Only the read actions search and get are ported; Piclaw's
// grep/extract/diff and the mutating actions are not available in gi.

type piclawMessagesParams struct {
	Action          string  `json:"action"`
	Query           *string `json:"query"`
	RowIDs          []int64 `json:"row_ids"`
	ChatJID         *string `json:"chat_jid"`
	Role            string  `json:"role"`
	Sender          string  `json:"sender"`
	After           string  `json:"after"`
	Before          string  `json:"before"`
	Since           string  `json:"since"`
	AfterRow        int64   `json:"after_row"`
	BeforeRow       int64   `json:"before_row"`
	Limit           *int    `json:"limit"`
	Offset          int     `json:"offset"`
	ExcerptChars    *int    `json:"excerpt_chars"`
	ContextBefore   int     `json:"context_before"`
	ContextAfter    int     `json:"context_after"`
	DetailsMaxChars *int    `json:"details_max_chars"`
}

// isPiclawMessagesCall selects the Piclaw contract when the model uses its
// action/query fields; calls without them keep gi's bounded JSON retrieval.
func isPiclawMessagesCall(args map[string]any) bool {
	for _, name := range []string{"action", "query", "chat_jid", "role", "sender", "after", "before", "since", "offset", "excerpt_chars", "details_max_chars"} {
		if _, ok := args[name]; ok {
			return true
		}
	}
	return false
}

// piclawChatSession maps Piclaw's chat_jid to a gi session: omitted means the
// current session, "*"/"all" every session, otherwise "gi:<id>" or "<id>".
func piclawChatSession(input *string, current string) (session string, all bool) {
	if input == nil {
		return current, false
	}
	trimmed := strings.TrimSpace(*input)
	if trimmed == "" || trimmed == "*" || strings.EqualFold(trimmed, "all") {
		return "", true
	}
	return strings.TrimPrefix(trimmed, "gi:"), false
}

func piclawChatJID(sessionID string) string { return "gi:" + sessionID }

func piclawRole(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "assistant":
		return "assistant"
	case "user":
		return "user"
	}
	return ""
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func executePiclawMessages(ctx context.Context, rt ToolRuntime, raw []byte) (string, error) {
	var p piclawMessagesParams
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return "", fmt.Errorf("messages: invalid arguments: %w", err)
	}
	action := p.Action
	if action == "" {
		action = "search"
	}
	var text string
	var details map[string]any
	var err error
	switch action {
	case "search":
		text, details, err = piclawSearch(ctx, rt, p)
	case "get":
		text, details, err = piclawGet(ctx, rt, p)
	case "grep", "extract", "diff", "add", "post", "delete", "move":
		return "", fmt.Errorf("messages: action %q is not available in gi; use search or get", action)
	default:
		text, details = "Unknown action: "+action, map[string]any{"error": "Unknown action: " + action, "requested_action": action}
	}
	if err != nil {
		return "", err
	}
	if rt.SetDetails != nil {
		rt.SetDetails(details)
	}
	return text, nil
}

func piclawFilter(p piclawMessagesParams, session string) store.TimelineFilter {
	after := p.Since
	if after == "" {
		after = p.After
	}
	return store.TimelineFilter{SessionID: session, Role: piclawRole(p.Role), Sender: strings.TrimSpace(p.Sender), AfterTime: after, BeforeTime: p.Before, AfterRow: p.AfterRow, BeforeRow: p.BeforeRow}
}

func piclawSearch(ctx context.Context, rt ToolRuntime, p piclawMessagesParams) (string, map[string]any, error) {
	query := ""
	if p.Query != nil {
		query = strings.TrimSpace(*p.Query)
	}
	if query == "" {
		return "Provide query for action=search.", map[string]any{"action": "search", "count": 0, "results": []any{}}, nil
	}
	session, _ := piclawChatSession(p.ChatJID, rt.SessionID)
	limit := 10
	if p.Limit != nil {
		limit = clampInt(*p.Limit, 1, 50)
	}
	offset := max(p.Offset, 0)
	excerptChars := -1
	if p.ExcerptChars != nil {
		excerptChars = clampInt(*p.ExcerptChars, 0, 1000)
	}
	rows, err := rt.Store.SearchTimeline(ctx, query, piclawFilter(p, session), limit, offset)
	if err != nil {
		return "", nil, err
	}
	if len(rows) == 0 {
		return "No matching messages found.", map[string]any{"action": "search", "count": 0, "results": []any{}}, nil
	}
	var terms []string
	if excerptChars > 0 {
		terms = piclawSearchTerms(query)
	}
	results := make([]map[string]any, 0, len(rows))
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		item := piclawResultRow(row, p.DetailsMaxChars)
		preview := item["content"].(string)
		if excerpt, truncated, ok := piclawExcerpt(row.Content, terms, excerptChars); ok {
			item["content_excerpt"], item["content_excerpt_truncated"] = excerpt, truncated
			preview = excerpt
		}
		results = append(results, item)
		lines = append(lines, fmt.Sprintf("[%d] %s: %s", row.RowID, row.Role, preview))
	}
	plural := "s"
	if len(rows) == 1 {
		plural = ""
	}
	details := map[string]any{"action": "search", "count": len(rows), "query": query, "results": results, "limit": limit, "offset": offset}
	if session != "" {
		details["chat_jid"] = piclawChatJID(session)
	}
	return fmt.Sprintf("Found %d message%s.\n%s", len(rows), plural, strings.Join(lines, "\n")), details, nil
}

func piclawGet(ctx context.Context, rt ToolRuntime, p piclawMessagesParams) (string, map[string]any, error) {
	seen := map[int64]bool{}
	var rowIDs []int64
	for _, id := range p.RowIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			rowIDs = append(rowIDs, id)
		}
	}
	if len(rowIDs) == 0 {
		return "Provide row_ids for action=get.", map[string]any{"action": "get", "count": 0, "messages": []any{}, "missing_row_ids": []any{}}, nil
	}
	if len(rowIDs) > 50 {
		return "", nil, fmt.Errorf("messages: row_ids accepts at most 50 IDs")
	}
	// As in Piclaw, get without chat_jid may select row IDs from any session.
	session := ""
	if p.ChatJID != nil {
		session, _ = piclawChatSession(p.ChatJID, rt.SessionID)
	}
	filter := piclawFilter(p, session)
	filter.AfterTime, filter.BeforeTime, filter.AfterRow, filter.BeforeRow = "", "", 0, 0
	before, after := clampInt(p.ContextBefore, 0, 20), clampInt(p.ContextAfter, 0, 20)
	var blocks []string
	messages := []map[string]any{}
	missing := []int64{}
	for _, id := range rowIDs {
		row, err := rt.Store.TimelineMessageByRow(ctx, id, filter)
		if err != nil {
			return "", nil, err
		}
		if row == nil {
			missing = append(missing, id)
			continue
		}
		prev, next, err := rt.Store.TimelineContext(ctx, *row, filter, before, after)
		if err != nil {
			return "", nil, err
		}
		parts := []string{fmt.Sprintf("- [%d] %s: %s", row.RowID, row.Role, piclawPreview(row.Content))}
		if len(prev) > 0 {
			parts = append(parts, "  before:\n"+piclawContextLines(prev))
		}
		if len(next) > 0 {
			parts = append(parts, "  after:\n"+piclawContextLines(next))
		}
		blocks = append(blocks, strings.Join(parts, "\n"))
		messages = append(messages, map[string]any{"message": piclawResultRow(*row, p.DetailsMaxChars), "context_before": piclawResultRows(prev, p.DetailsMaxChars), "context_after": piclawResultRows(next, p.DetailsMaxChars)})
	}
	text := strings.Join(blocks, "\n\n")
	if text == "" {
		text = "No messages found for requested row IDs."
	}
	return text, map[string]any{"action": "get", "count": len(messages), "messages": messages, "missing_row_ids": missing, "context_before": before, "context_after": after}, nil
}

func piclawPreview(content string) string {
	if content == "" {
		return "[empty message]"
	}
	return content
}

func piclawContextLines(rows []store.TimelineMessage) string {
	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = fmt.Sprintf("  [%d] %s: %s", r.RowID, r.Role, piclawPreview(r.Content))
	}
	return strings.Join(lines, "\n")
}

func piclawResultRows(rows []store.TimelineMessage, maxChars *int) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = piclawResultRow(r, maxChars)
	}
	return out
}

// piclawResultRow ports clipContent. Lengths count runes (Piclaw counts UTF-16
// code units).
func piclawResultRow(r store.TimelineMessage, maxChars *int) map[string]any {
	row := map[string]any{"rowid": r.RowID, "id": r.ID, "chat_jid": piclawChatJID(r.SessionID), "sender": r.Role, "role": r.Role, "content": r.Content, "created_at": r.CreatedAt}
	if maxChars == nil {
		return row
	}
	limit := max(*maxChars, 0)
	runes := []rune(r.Content)
	switch {
	case limit == 0:
		row["content"], row["content_truncated"], row["content_full_length"] = "", len(runes) > 0, len(runes)
	case len(runes) > limit:
		row["content"], row["content_truncated"], row["content_full_length"] = string(runes[:max(1, limit-1)])+"…", true, len(runes)
	}
	return row
}

// piclawSearchTerms ports extractSearchTerms.
func piclawSearchTerms(query string) []string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" || trimmed == "*" {
		return nil
	}
	if strings.HasPrefix(trimmed, "#") {
		if tag := strings.TrimSpace(strings.TrimLeft(trimmed, "#")); tag != "" {
			return []string{tag}
		}
		return nil
	}
	edge := func(r rune) bool { return !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '-') }
	seen := map[string]bool{}
	var out []string
	for _, field := range strings.Fields(trimmed) {
		term := strings.TrimRightFunc(strings.TrimLeftFunc(field, edge), edge)
		if term != "" && !seen[term] {
			seen[term] = true
			out = append(out, term)
		}
	}
	return out
}

// piclawExcerpt ports buildContentExcerpt, counting runes.
func piclawExcerpt(content string, terms []string, width int) (string, bool, bool) {
	if len(terms) == 0 || width <= 0 {
		return "", false, false
	}
	runes := []rune(content)
	lower := []rune(strings.ToLower(content))
	if len(lower) != len(runes) {
		lower = runes // case mapping changed length; match exactly instead
	}
	seen := map[string]bool{}
	var normalized []string
	for _, t := range terms {
		if t = strings.ToLower(t); t != "" && !seen[t] {
			seen[t] = true
			normalized = append(normalized, t)
		}
	}
	matchIndex, matchLen := -1, 0
	for _, t := range normalized {
		if i := runeIndex(lower, []rune(t)); i != -1 && (matchIndex == -1 || i < matchIndex) {
			matchIndex, matchLen = i, len([]rune(t))
		}
	}
	if matchIndex == -1 {
		return "", false, false
	}
	safe := max(8, width)
	start := max(0, matchIndex-(safe-matchLen)/2)
	end := min(len(runes), start+safe)
	if end-start < safe {
		start = max(0, end-safe)
	}
	sorted := append([]string(nil), normalized...)
	sort.SliceStable(sorted, func(i, j int) bool { return utf8.RuneCountInString(sorted[i]) > utf8.RuneCountInString(sorted[j]) })
	for i, t := range sorted {
		sorted[i] = regexp.QuoteMeta(t)
	}
	pattern := regexp.MustCompile("(?i)" + strings.Join(sorted, "|"))
	highlighted := pattern.ReplaceAllString(string(runes[start:end]), "[[$0]]")
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(runes) {
		suffix = "…"
	}
	return prefix + highlighted + suffix, start > 0 || end < len(runes), true
}

func runeIndex(haystack, needle []rune) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
