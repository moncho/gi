package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

var ErrWidgetExists = errors.New("widget ID already exists in this session")

type DashboardWidget struct {
	WidgetID     string         `json:"widget_id"`
	PostID       string         `json:"post_id"`
	ChatJID      string         `json:"chat_jid"`
	ContentBlock map[string]any `json:"content_block"`
}

// DashboardWidget reads the persisted artifact from the owning message. The
// session is always explicit: widget IDs never grant cross-session access.
func (s *Store) DashboardWidget(ctx context.Context, sessionID, widgetID string) (DashboardWidget, error) {
	var out DashboardWidget
	var raw string
	err := s.db.QueryRowContext(ctx, `select m.id,block.value from messages m,
 json_each(m.payload_json,'$.content_blocks') block where m.session_id=?
 and json_extract(block.value,'$.type')='generated_widget'
 and json_extract(block.value,'$.widget_id')=? order by m.created_at desc,m.id desc limit 1`, sessionID, widgetID).Scan(&out.PostID, &raw)
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal([]byte(raw), &out.ContentBlock); err != nil {
		return out, err
	}
	out.WidgetID, out.ChatJID = widgetID, "gi:"+sessionID
	return out, nil
}

// PostDashboardWidget stores the artifact and updates session activity together.
// The artifact belongs to an assistant timeline message; model context sees
// only its fallback content, not HTML in the payload.
func (s *Store) PostDashboardWidget(ctx context.Context, sessionID, turnID, content string, block map[string]any) (Message, error) {
	var out Message
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	var exists bool
	err = tx.QueryRowContext(ctx, `select exists(select 1 from messages m,json_each(m.payload_json,'$.content_blocks') block
 where m.session_id=? and json_extract(block.value,'$.type')='generated_widget' and json_extract(block.value,'$.widget_id')=?)`, sessionID, block["widget_id"]).Scan(&exists)
	if err != nil {
		return out, err
	}
	if exists {
		return out, ErrWidgetExists
	}
	result, err := tx.ExecContext(ctx, "update sessions set updated_at="+defaultNow+" where id=?", sessionID)
	if err != nil {
		return out, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return out, sql.ErrNoRows
	}
	out = Message{ID: NowID("msg"), SessionID: sessionID, Role: "assistant", Content: content,
		Payload: map[string]any{"kind": "dashboard_widget", "source": "send_dashboard_widget", "turn_id": turnID, "content_blocks": []any{block}}}
	raw, err := json.Marshal(out.Payload)
	if err != nil {
		return Message{}, err
	}
	if _, err = tx.ExecContext(ctx, `insert into messages(id,session_id,role,content,payload_json,created_at) values(?,?,?,?,?,`+defaultNow+`)`, out.ID, sessionID, out.Role, content, string(raw)); err != nil {
		return Message{}, err
	}
	if err = tx.QueryRowContext(ctx, "select created_at from messages where id=?", out.ID).Scan(&out.CreatedAt); err != nil {
		return Message{}, err
	}
	if err = tx.Commit(); err != nil {
		return Message{}, err
	}
	return out, nil
}
