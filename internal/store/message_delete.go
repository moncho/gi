package store

import (
	"context"
	"database/sql"
	"errors"
)

var ErrMessageDeleteBusy = errors.New("Session has active or queued work")
var ErrMessageDeleteProtected = errors.New("Only user and assistant messages can be deleted")

// DeleteMessage removes one flat timeline row, never its audit turn/events or
// stored media. A checkpoint reset in the same transaction advances its version
// so a concurrent snapshot cannot publish a summary containing removed content.
func (s *Store) DeleteMessage(ctx context.Context, sessionID, messageID string) error {
	_, err := s.DeleteMessageWithReplies(ctx, sessionID, messageID, false)
	return err
}

// DeleteMessageWithReplies removes the target and, when cascade is explicit,
// its assistant replies. Checkpoint invalidation and deletion commit together.
func (s *Store) DeleteMessageWithReplies(ctx context.Context, sessionID, messageID string, cascade bool) (ids []string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var role string
	if err = tx.QueryRowContext(ctx, "SELECT role FROM messages WHERE session_id=? AND id=?", sessionID, messageID).Scan(&role); err != nil {
		return nil, err
	}
	if role != "user" && role != "assistant" {
		return nil, ErrMessageDeleteProtected
	}
	var busy bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM session_active_turns WHERE session_id=?) OR EXISTS(SELECT 1 FROM turns WHERE session_id=? AND status IN ('running','queued','cancelling'))`, sessionID, sessionID).Scan(&busy); err != nil {
		return nil, err
	}
	if busy {
		return nil, ErrMessageDeleteBusy
	}
	ids = []string{messageID}
	if cascade {
		rows, e := tx.QueryContext(ctx, `select m.id from messages m where m.session_id=? and m.id!=? and (`+conversationReplyToSQL("m")+`)=? order by m.created_at,m.id`, sessionID, messageID, messageID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return nil, e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	for _, id := range ids {
		result, err := tx.ExecContext(ctx, "DELETE FROM messages WHERE session_id=? AND id=?", sessionID, id)
		if err != nil {
			return nil, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return nil, sql.ErrNoRows
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO context_checkpoints(session_id,version,summary,covered_json,created_at) VALUES(?,1,'','[]',`+defaultNow+`)
 ON CONFLICT(session_id) DO UPDATE SET version=version+1,summary='',covered_json='[]',created_at=excluded.created_at`, sessionID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sessions SET updated_at="+defaultNow+" WHERE id=?", sessionID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return ids, nil
}

// MessageReplyTo returns the persisted prompt associated with an assistant
// message; used to give live posts the same reply identity as loaded pages.
func (s *Store) MessageReplyTo(ctx context.Context, sessionID, messageID string) (string, error) {
	var parent string
	err := s.db.QueryRowContext(ctx, `select `+conversationReplyToSQL("m")+` from messages m where m.session_id=? and m.id=?`, sessionID, messageID).Scan(&parent)
	return parent, err
}
