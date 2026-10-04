package turn

import (
	"context"

	"github.com/rcarmo/gi/internal/logutil"
	"github.com/rcarmo/gi/internal/store"
)

func (e *Engine) publishWidgetMessage(m store.Message) {
	parent, err := e.store.MessageReplyTo(context.Background(), m.SessionID, m.ID)
	logutil.WarnIfErr("widget reply identity", err)
	agentID := "default"
	if session, err := e.store.GetSession(context.Background(), m.SessionID); err == nil {
		if name, ok := session.State["agent_id"].(string); ok && name != "" {
			agentID = name
		}
	}
	e.broadcast(m.SessionID, map[string]any{"type": "new_post", "id": m.ID, "chat_jid": "gi:" + m.SessionID,
		"turn_id": m.Payload["turn_id"], "content": m.Content, "timestamp": m.CreatedAt, "sender": "agent", "is_bot_message": true,
		"data": map[string]any{"type": "agent_response", "content": m.Content, "agent_id": agentID, "thread_id": parent, "content_blocks": m.Payload["content_blocks"]}})
}
