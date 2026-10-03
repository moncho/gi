package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// ImportedMessage is a message with its own time, for sessions imported from
// another format.
type ImportedMessage struct {
	ID, Role, Content string
	Payload           map[string]any
	CreatedAt         string // "2006-01-02T15:04:05.000Z"
}

// AddImportedMessages inserts messages with their own times, in one
// transaction.
func (s *Store) AddImportedMessages(ctx context.Context, sessionID string, messages []ImportedMessage) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, m := range messages {
		payload, err := marshalJSON(m.Payload)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `insert into messages (id, session_id, role, content, payload_json, created_at) values (?, ?, ?, ?, ?, ?)`,
			m.ID, sessionID, m.Role, m.Content, payload, m.CreatedAt); err != nil {
			return fmt.Errorf("import message: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `update sessions set updated_at = `+defaultNow+` where id = ?`, sessionID); err != nil {
		return err
	}
	return tx.Commit()
}

// SetContextCheckpoint starts a session's context with summary in place of
// the covered messages (an imported compaction). They must be the first
// messages of the session's context, in order.
func (s *Store) SetContextCheckpoint(ctx context.Context, sessionID, summary string, coveredIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := readContextSnapshot(ctx, tx, sessionID)
	if err != nil {
		return err
	}
	if current.Summary != "" || len(coveredIDs) == 0 || len(coveredIDs) > len(current.Messages) {
		return ErrContextChanged
	}
	covered := make([]ContextFingerprint, len(coveredIDs))
	for i, id := range coveredIDs {
		if current.Messages[i].ID != id {
			return ErrContextChanged
		}
		if covered[i], err = fingerprintMessage(current.Messages[i]); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(covered)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into context_checkpoints(session_id,version,summary,covered_json,created_at) values(?,?,?,?,`+defaultNow+`) on conflict(session_id) do update set version=excluded.version,summary=excluded.summary,covered_json=excluded.covered_json,created_at=excluded.created_at`,
		sessionID, current.Version+1, summary, string(raw)); err != nil {
		return err
	}
	return tx.Commit()
}
