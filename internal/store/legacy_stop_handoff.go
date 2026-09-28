package store

import "context"

// LegacyStopQueueHandoffs identifies only queues paused by the removed web UX.
func (s *Store) LegacyStopQueueHandoffs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `select id from sessions where json_extract(state_json,'$.legacy_stop_queue_handoff')=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) CompleteLegacyStopQueueHandoff(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `update sessions set state_json=json_remove(state_json,'$.legacy_stop_queue_handoff') where id=?`, id)
	return err
}
