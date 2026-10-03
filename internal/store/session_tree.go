package store

import (
	"context"
	"encoding/json"
	"time"
)

// Pi's /tree branches live in one session file. gi keeps one message list
// per session, so a /tree branch is a copy of the session up to the chosen
// entry, marked with the session it branched from; labels belong to the
// root of the branch family. See docs/internal/tui-tree.md.

const (
	treeParentKey = "tree_parent" // state: the session a /tree branch was taken from
	treeLabelsKey = "tree_labels" // state of the family root: entry ID -> {label, time}
)

// TreeLabel is a /tree label and when it was set (RFC 3339).
type TreeLabel struct {
	Label string `json:"label"`
	Time  string `json:"time,omitempty"`
}

// BranchSessionBefore copies sourceSessionID's messages before
// beforeMessageID (all of them when empty) into a new session that /tree
// shows as a branch of the source.
func (s *Store) BranchSessionBefore(ctx context.Context, sourceSessionID, newID, newTitle, newAgentID, beforeMessageID string) (*Session, error) {
	branch, err := s.CloneSessionBefore(ctx, sourceSessionID, newID, newTitle, newAgentID, beforeMessageID)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `update sessions set state_json=json_set(coalesce(nullif(state_json,''),'{}'),'$.`+treeParentKey+`',?) where id=?`, sourceSessionID, branch.ID); err != nil {
		return nil, err
	}
	return s.GetSession(ctx, branch.ID)
}

// TreeParent is the session a /tree branch was taken from, or "".
func TreeParent(sess Session) string {
	parent, _ := sess.State[treeParentKey].(string)
	return parent
}

// TreeLabels reads the labels kept in a family root's state.
func TreeLabels(sess Session) map[string]TreeLabel {
	out := map[string]TreeLabel{}
	raw, err := json.Marshal(sess.State[treeLabelsKey])
	if err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

// SetTreeLabel sets (or, with an empty label, removes) the label of a /tree
// entry in the family root's state.
func (s *Store) SetTreeLabel(ctx context.Context, rootSessionID, entryID, label string, at time.Time) error {
	var value any // JSON null removes the key in json_patch
	if label != "" {
		value = TreeLabel{Label: label, Time: at.UTC().Format("2006-01-02T15:04:05.000Z")}
	}
	patch, err := json.Marshal(map[string]any{entryID: value})
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `update sessions set state_json=json_set(coalesce(nullif(state_json,''),'{}'),'$.`+treeLabelsKey+`',
		json(json_patch(coalesce(json_extract(nullif(state_json,''),'$.`+treeLabelsKey+`'),'{}'),?))),updated_at=`+defaultNow+` where id=?`, string(patch), rootSessionID)
	return err
}
