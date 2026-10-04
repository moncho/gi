package turn

import (
	"github.com/rcarmo/gi/internal/store"
	"strings"
)

// PublishPlanChanged announces committed plan changes to the owning session.
func (e *Engine) PublishPlanChanged(p store.SessionPlan, source, action string) {
	if !strings.HasPrefix(p.ChatJID, "gi:") || p.ChatJID == "gi:" {
		return
	}
	session := p.ChatJID[len("gi:"):]
	e.broadcast(session, map[string]any{"type": "extension_ui_status", "key": "plan.changes", "addon": "plan-sidebar", "chat_jid": p.ChatJID, "updated_at": p.UpdatedAt, "source": source, "action": action})
}
