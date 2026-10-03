package store

import (
	"context"
	"regexp"
	"strings"
)

// Piclaw-compatible timeline reads for the messages tool's search/get actions.
// Piclaw's messages table holds chat posts only, so these reads use gi's
// conversation view (no tool results or tool-call-only rows; tool-call text
// stripped). Row IDs are the durable message_rows identities.

// TimelineMessage is one user/assistant post with its durable numeric row ID.
type TimelineMessage struct {
	RowID     int64
	SessionID string
	ID        string
	Role      string
	Content   string
	CreatedAt string
}

// TimelineFilter mirrors Piclaw's search/get filters. Empty SessionID means
// every session (Piclaw's chat_jid "*"/"all"). Role "user" selects user posts and
// "assistant" every other visible post (Piclaw's is_bot_message). Sender matches
// the author label (the stored role) case-insensitively.
type TimelineFilter struct {
	SessionID  string
	Role       string
	Sender     string
	AfterTime  string
	BeforeTime string
	AfterRow   int64
	BeforeRow  int64
}

const timelineColumns = `r.row_id,m.session_id,m.id,m.role,` + conversationContentSQL + `,m.created_at`
const timelineFrom = ` from messages m join message_rows r on r.message_id=m.id`

func (f TimelineFilter) conditions() ([]string, []any) {
	conds := []string{`(` + conversationVisibleSQL + `)`}
	var args []any
	if f.SessionID != "" {
		conds = append(conds, `m.session_id=?`)
		args = append(args, f.SessionID)
	}
	switch f.Role {
	case "user":
		conds = append(conds, `m.role='user'`)
	case "assistant":
		conds = append(conds, `m.role<>'user'`)
	}
	if f.Sender != "" {
		conds = append(conds, `m.role=? collate nocase`)
		args = append(args, f.Sender)
	}
	if f.AfterTime != "" {
		conds = append(conds, `m.created_at>?`)
		args = append(args, f.AfterTime)
	}
	if f.BeforeTime != "" {
		conds = append(conds, `m.created_at<?`)
		args = append(args, f.BeforeTime)
	}
	if f.AfterRow > 0 {
		conds = append(conds, `r.row_id>?`)
		args = append(args, f.AfterRow)
	}
	if f.BeforeRow > 0 {
		conds = append(conds, `r.row_id<?`)
		args = append(args, f.BeforeRow)
	}
	return conds, args
}

var ftsSpecialChars = regexp.MustCompile(`[":()^*{}]`)
var ftsOperatorWord = regexp.MustCompile(`\b(?:AND|OR|NOT|NEAR)\b`)
var ftsQuoted = regexp.MustCompile(`"[^"]+"`)
var ftsGroup = regexp.MustCompile(`\(.*\)`)
var ftsColumn = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*:`)

// IsSearchOperatorQuery ports Piclaw's isFtsOperatorQuery.
func IsSearchOperatorQuery(q string) bool {
	q = strings.TrimSpace(q)
	if ftsOperatorWord.MatchString(q) || ftsQuoted.MatchString(q) || ftsGroup.MatchString(q) {
		return true
	}
	if loc := ftsColumn.FindStringIndex(q); loc != nil && !strings.HasPrefix(q[loc[1]:], "//") {
		return true
	}
	return false
}

// SearchFallbackTerms ports Piclaw's extractFtsFallbackTerms.
func SearchFallbackTerms(q string, dropKeywords bool) []string {
	stripped := strings.Join(strings.Fields(ftsSpecialChars.ReplaceAllString(q, " ")), " ")
	var out []string
	for _, token := range strings.Split(stripped, " ") {
		token = strings.TrimSpace(strings.TrimLeft(token, "-"))
		if token == "" {
			continue
		}
		switch strings.ToUpper(token) {
		case "AND", "OR", "NOT", "NEAR":
			if dropKeywords {
				continue
			}
		}
		out = append(out, token)
	}
	return out
}

// SearchTimeline ports Piclaw's runSearch: "*" lists, "#tag" matches the tag,
// and other queries match terms as case-insensitive substrings (as in Piclaw's
// LIKE fallback, wildcards are not escaped). Results are newest row first.
func (s *Store) SearchTimeline(ctx context.Context, query string, f TimelineFilter, limit, offset int) ([]TimelineMessage, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	conds, args := f.conditions()
	switch {
	case query == "*":
	case strings.HasPrefix(query, "#"):
		tag := strings.TrimLeft(query, "#")
		if tag == "" {
			return nil, nil
		}
		conds = append(conds, `(`+conversationContentSQL+`) like ? collate nocase`)
		args = append(args, "%#"+tag+"%")
	default:
		// Piclaw passes operator queries to FTS5 and joins plain terms with OR
		// (its default match mode). Without an FTS index, plain queries match
		// any term and operator queries use Piclaw's all-terms fallback.
		operator := IsSearchOperatorQuery(query)
		terms := SearchFallbackTerms(query, operator)
		if len(terms) == 0 {
			return nil, nil
		}
		match := make([]string, len(terms))
		for i, term := range terms {
			match[i] = `(` + conversationContentSQL + `) like ? collate nocase`
			args = append(args, "%"+term+"%")
		}
		joiner := " or "
		if operator {
			joiner = " and "
		}
		conds = append(conds, `(`+strings.Join(match, joiner)+`)`)
	}
	args = append(args, limit, offset)
	return s.queryTimeline(ctx, `select `+timelineColumns+timelineFrom+` where `+strings.Join(conds, " and ")+` order by r.row_id desc limit ? offset ?`, args...)
}

// TimelineMessageByRow ports Piclaw's fetchByRowId; nil means not found.
func (s *Store) TimelineMessageByRow(ctx context.Context, rowID int64, f TimelineFilter) (*TimelineMessage, error) {
	conds, args := f.conditions()
	conds = append(conds, `r.row_id=?`)
	args = append(args, rowID)
	rows, err := s.queryTimeline(ctx, `select `+timelineColumns+timelineFrom+` where `+strings.Join(conds, " and ")+` limit 1`, args...)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// TimelineContext ports Piclaw's fetchContextRows: neighbours in row order
// within the anchor's session, oldest first.
func (s *Store) TimelineContext(ctx context.Context, anchor TimelineMessage, f TimelineFilter, before, after int) ([]TimelineMessage, []TimelineMessage, error) {
	f.SessionID = anchor.SessionID
	f.AfterTime, f.BeforeTime, f.AfterRow, f.BeforeRow = "", "", 0, 0
	conds, args := f.conditions()
	where := strings.Join(conds, " and ")
	var prev, next []TimelineMessage
	var err error
	if before > 0 {
		prev, err = s.queryTimeline(ctx, `select `+timelineColumns+timelineFrom+` where `+where+` and r.row_id<? order by r.row_id desc limit ?`, append(append([]any{}, args...), anchor.RowID, before)...)
		if err != nil {
			return nil, nil, err
		}
		for i, j := 0, len(prev)-1; i < j; i, j = i+1, j-1 {
			prev[i], prev[j] = prev[j], prev[i]
		}
	}
	if after > 0 {
		next, err = s.queryTimeline(ctx, `select `+timelineColumns+timelineFrom+` where `+where+` and r.row_id>? order by r.row_id asc limit ?`, append(append([]any{}, args...), anchor.RowID, after)...)
		if err != nil {
			return nil, nil, err
		}
	}
	return prev, next, nil
}

func (s *Store) queryTimeline(ctx context.Context, q string, args ...any) ([]TimelineMessage, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TimelineMessage
	for rows.Next() {
		var m TimelineMessage
		if err := rows.Scan(&m.RowID, &m.SessionID, &m.ID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
