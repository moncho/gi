package store

import (
	"context"
	"encoding/json"
)

// ClaimIdleQueueAction fences the exact queue row and observed hold in one
// statement. The hold is not removed: this action authorises one row only.
func (s *Store) ClaimIdleQueueAction(ctx context.Context, sessionID, queuedID, workerID, token, holdID string) (bool, error) {
	return s.claimIdleQueueAction(ctx, sessionID, queuedID, workerID, token, holdID, "")
}

// ClaimEndedQueueSteer admits only the latest released terminal run. It cannot
// bypass Stop or replay previously bound steering, even if it was returned.
func (s *Store) ClaimEndedQueueSteer(ctx context.Context, sessionID, queuedID, workerID, token, endedID string) (bool, error) {
	if endedID == "" || endedID == queuedID {
		return false, ErrQueueConflict
	}
	return s.claimIdleQueueAction(ctx, sessionID, queuedID, workerID, token, "", endedID)
}

func (s *Store) claimIdleQueueAction(ctx context.Context, sessionID, queuedID, workerID, token, holdID, endedID string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `insert into session_active_turns(session_id,turn_id,worker_id,claim_token,claimed_at,updated_at)
 select ?,?,?,?,`+defaultNow+`,`+defaultNow+` where exists(select 1 from turns where id=? and session_id=? and status='queued' and phase!='manual_compaction')
 and coalesce((select stop_turn_id from web_queue_holds where session_id=?),'')=?
 and (?='' or (
  exists(select 1 from session_last_run l join turns old on old.id=l.turn_id
   where l.session_id=? and l.turn_id=? and old.session_id=l.session_id
   and old.status in ('completed','failed','cancelled','aborted'))
  and exists(select 1 from turns q where q.id=? and q.session_id=? and q.phase<>'steer_returned'
   and q.claimed_by is null and coalesce(json_extract(q.metadata_json,'$.operation'),'')<>'manual_compaction')
  and not exists(select 1 from steering_queue where session_id=? and source_queue_id=?)
 ))
 on conflict(session_id) do nothing`, sessionID, queuedID, workerID, token, queuedID, sessionID, sessionID, holdID,
		endedID, sessionID, endedID, queuedID, sessionID, sessionID, queuedID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// PersistIdleQueuePrompt is the final launch barrier, after all fallible launch
// preparation. A crash/retry may reuse this exact durable prompt, never add it twice.
func (s *Store) PersistIdleQueuePrompt(ctx context.Context, sessionID, turnID, prompt string, payload map[string]any) error {
	ownedPayload := make(map[string]any, len(payload)+2)
	for key, value := range payload {
		ownedPayload[key] = value
	}
	ownedPayload["turn_id"] = turnID
	ownedPayload["idle_queue_action"] = true
	raw, err := json.Marshal(ownedPayload)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owned bool
	if err = tx.QueryRowContext(ctx, `select exists(select 1 from session_active_turns where session_id=? and turn_id=? and claim_token=?)`, sessionID, turnID, turnID).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return ErrQueueConflict
	}
	_, err = tx.ExecContext(ctx, `insert into messages(id,session_id,role,content,payload_json,created_at)
 select ?,?,'user',?,?,`+defaultNow+` where exists(select 1 from session_active_turns where session_id=? and turn_id=?)
 and not exists(select 1 from messages where session_id=? and role='user' and json_extract(payload_json,'$.turn_id')=? and json_extract(payload_json,'$.idle_queue_action')=1)`, NowID("msg"), sessionID, prompt, string(raw), sessionID, turnID, sessionID, turnID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) HasIdleQueuePrompt(ctx context.Context, sessionID, turnID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `select exists(select 1 from messages where session_id=? and role='user' and json_extract(payload_json,'$.turn_id')=? and json_extract(payload_json,'$.idle_queue_action')=1)`, sessionID, turnID).Scan(&exists)
	return exists, err
}
