package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ListTUIInputHistory returns the most recent submitted editor entries in
// chronological order. Before the first TUI write, older databases still have
// user prompts in messages; those provide the initial history on upgrade.
func (s *Store) ListTUIInputHistory(ctx context.Context, sessionID string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("invalid input history limit: %d", limit)
	}
	rows, err := s.db.QueryContext(ctx, `select content from tui_input_history where session_id=? order by id desc limit ?`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list tui input history: %w", err)
	}
	var recent []string
	for rows.Next() {
		var content string
		if err = rows.Scan(&content); err != nil {
			break
		}
		recent = append(recent, content)
	}
	if err == nil {
		err = rows.Err()
	}
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("list tui input history: %w", err)
	}
	if len(recent) == 0 {
		// Keep historical prompts available without writing on a read or
		// pulling an entire long conversation into memory.
		rows, err = s.db.QueryContext(ctx, `select content from messages where session_id=? and role='user' and trim(content)<>'' order by created_at desc, id desc limit ?`, sessionID, limit)
		if err != nil {
			return nil, fmt.Errorf("list legacy tui input history: %w", err)
		}
		for rows.Next() {
			var content string
			if err = rows.Scan(&content); err != nil {
				break
			}
			recent = append(recent, content)
		}
		if err == nil {
			err = rows.Err()
		}
		if closeErr := rows.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, fmt.Errorf("list legacy tui input history: %w", err)
		}
	}
	for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
		recent[i], recent[j] = recent[j], recent[i]
	}
	return recent, nil
}

// RecordTUIInput persists one accepted TUI editor submission. The first write
// seeds existing prompts in the same transaction, so an upgrade does not lose
// old history when the first command is entered. History is not a conversation
// message and never changes the session's updated_at or inference context.
func (s *Store) RecordTUIInput(ctx context.Context, sessionID, content string, limit int) error {
	if limit <= 0 || strings.TrimSpace(content) == "" {
		return fmt.Errorf("invalid tui input history entry")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tui input history: %w", err)
	}
	defer tx.Rollback()
	var existing int
	err = tx.QueryRowContext(ctx, `select 1 from tui_input_history where session_id=? limit 1`, sessionID).Scan(&existing)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `insert into tui_input_history(session_id, content)
			select session_id, content from (
				select session_id, content, created_at, id from messages
				where session_id=? and role='user' and trim(content)<>''
				order by created_at desc, id desc limit ?
			) order by created_at, id`, sessionID, limit)
	}
	if err != nil {
		return fmt.Errorf("seed tui input history: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `insert into tui_input_history(session_id, content) values (?,?)`, sessionID, content); err != nil {
		return fmt.Errorf("record tui input history: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `delete from tui_input_history where session_id=? and id not in
		(select id from tui_input_history where session_id=? order by id desc limit ?)`, sessionID, sessionID, limit); err != nil {
		return fmt.Errorf("trim tui input history: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tui input history: %w", err)
	}
	return nil
}
