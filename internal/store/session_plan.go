package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rcarmo/gi/internal/plan"
)

type SessionPlan struct {
	ChatJID     string      `json:"chat_jid"`
	Markdown    string      `json:"markdown"`
	UpdatedAt   *string     `json:"updated_at"`
	Explanation *string     `json:"explanation"`
	Plan        []plan.Item `json:"plan"`
}

func planDetails(session, markdown string, updated *string) SessionPlan {
	parsed := plan.Parse(markdown)
	return SessionPlan{"gi:" + session, markdown, updated, parsed.Explanation, parsed.Plan}
}
func (s *Store) SessionPlan(ctx context.Context, session string) (SessionPlan, error) {
	if _, err := s.GetSession(ctx, session); err != nil {
		return SessionPlan{}, err
	}
	var raw []byte
	err := s.db.QueryRowContext(ctx, "select value from kv_store where namespace='session_plan' and key=?", session).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return planDetails(session, plan.DefaultMarkdown, nil), nil
	}
	if err != nil {
		return SessionPlan{}, err
	}
	var saved SessionPlan
	if err = json.Unmarshal(raw, &saved); err != nil {
		return SessionPlan{}, err
	}
	return planDetails(session, saved.Markdown, saved.UpdatedAt), nil
}
func (s *Store) MutateSessionPlan(ctx context.Context, session string, mutation plan.Mutation) (SessionPlan, error) {
	var out SessionPlan
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, "select exists(select 1 from sessions where id=?)", session).Scan(&exists); err != nil {
		return out, err
	}
	if !exists {
		return out, sql.ErrNoRows
	}
	markdown := plan.DefaultMarkdown
	var raw []byte
	err = tx.QueryRowContext(ctx, "select value from kv_store where namespace='session_plan' and key=?", session).Scan(&raw)
	if err == nil {
		var current SessionPlan
		if err = json.Unmarshal(raw, &current); err != nil {
			return out, err
		}
		markdown = current.Markdown
	} else if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	markdown, err = plan.Apply(markdown, mutation)
	if err != nil {
		return out, fmt.Errorf("%w: %v", plan.ErrInvalid, err)
	}
	updated := time.Now().UTC().Format(time.RFC3339Nano)
	out = planDetails(session, markdown, &updated)
	raw, err = json.Marshal(out)
	if err != nil {
		return SessionPlan{}, err
	}
	if _, err = tx.ExecContext(ctx, `insert into kv_store(namespace,key,value,created_at,updated_at) values('session_plan',?,?,?,?)
 on conflict(namespace,key) do update set value=excluded.value,updated_at=excluded.updated_at`, session, raw, updated, updated); err != nil {
		return SessionPlan{}, err
	}
	if err = tx.Commit(); err != nil {
		return SessionPlan{}, err
	}
	return out, nil
}
