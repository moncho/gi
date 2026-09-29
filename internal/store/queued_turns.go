package store

import (
	"context"
	"errors"
	"fmt"
)

var ErrQueueConflict = errors.New("queue changed; refresh and retry")

// PendingTUIMessage is a read-only projection of undelivered work, not an
// admission or restoration claim. Steering appears before follow-ups as in Pi.
type PendingTUIMessage struct {
	Kind string
	Text string
}

func (s *Store) ListPendingTUIMessages(ctx context.Context, sessionID string) ([]PendingTUIMessage, error) {
	rows, err := s.db.QueryContext(ctx, `
		select kind, text from (
			select 0 as group_order, id as position, created_at, cast(id as text) as item_id,
				'Steering' as kind,
				case when content = '' and (json_array_length(media_json) > 0 or json_array_length(json_extract(payload_json, '$.media')) > 0)
					then '[attachment]' else content end as text
			from steering_queue where session_id = ? and status = 'queued'
			union all
			select 1 as group_order, queue_position as position, created_at, id as item_id,
				'Follow-up' as kind,
				case when prompt = '' and json_array_length(json_extract(metadata_json, '$.media')) > 0
					then '[attachment]' else prompt end as text
			from turns where session_id = ? and status = 'queued'
					and (prompt != '' or json_array_length(json_extract(metadata_json, '$.media')) > 0)
		) order by group_order, position, created_at, item_id`, sessionID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []PendingTUIMessage
	for rows.Next() {
		var item PendingTUIMessage
		if err := rows.Scan(&item.Kind, &item.Text); err != nil {
			return nil, err
		}
		messages = append(messages, item)
	}
	return messages, rows.Err()
}

// Queue order is separate from immutable admission timestamps and turn events.
func (s *Store) ListQueuedTurns(ctx context.Context, sessionID string) ([]Turn, error) {
	rows, err := s.db.QueryContext(ctx, `select id, session_id, status, phase, prompt, metadata_json, created_at, updated_at from turns where session_id = ? and status = 'queued' order by queue_position, created_at, id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Turn{}
	for rows.Next() {
		var item Turn
		var metadata string
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Status, &item.Phase, &item.Prompt, &metadata, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Metadata, err = unmarshalJSONMap(metadata)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// Conditional update protects against a claim in another engine/process.
func (s *Store) CancelQueuedTurn(ctx context.Context, sessionID, turnID string) error {
	result, err := s.db.ExecContext(ctx, `update turns set status = 'cancelled', phase = 'aborted', updated_at = `+defaultNow+` where id = ? and session_id = ? and status = 'queued' and not exists(select 1 from session_active_turns where turn_id = ?)`, turnID, sessionID, turnID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrQueueConflict
	}
	return nil
}

// Reorder only an exact queue snapshot; never reorder a running or foreign turn.
func (s *Store) ReorderQueuedTurns(ctx context.Context, sessionID string, expected, order []string) error {
	if len(order) == 0 || len(order) != len(expected) {
		return ErrQueueConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var claimed bool
	if err := tx.QueryRowContext(ctx, `select exists(select 1 from turns t join session_active_turns a on a.turn_id=t.id where t.session_id=? and t.status='queued')`, sessionID).Scan(&claimed); err != nil {
		return err
	}
	if claimed {
		return ErrQueueConflict
	}
	rows, err := tx.QueryContext(ctx, `select id from turns where session_id = ? and status = 'queued' order by queue_position, created_at, id`, sessionID)
	if err != nil {
		return err
	}
	current := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		current = append(current, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(current) != len(expected) {
		return ErrQueueConflict
	}
	wanted := map[string]bool{}
	for i, id := range expected {
		if current[i] != id {
			return ErrQueueConflict
		}
		wanted[id] = true
	}
	for _, id := range order {
		if !wanted[id] {
			return ErrQueueConflict
		}
		delete(wanted, id)
	}
	if len(wanted) != 0 {
		return ErrQueueConflict
	}
	for i, id := range order {
		result, err := tx.ExecContext(ctx, `update turns set queue_position = ? where id = ? and session_id = ? and status = 'queued' and not exists(select 1 from session_active_turns where turn_id=?)`, i+1, id, sessionID, id)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("%w: %s", ErrQueueConflict, id)
		}
	}
	return tx.Commit()
}
