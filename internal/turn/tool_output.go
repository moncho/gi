package turn

import (
	"sync"
	"time"

	"github.com/rcarmo/gi/internal/store"
)

// Each reporter belongs to one execution, not a provider's reusable call ID.
// Persist before invalidating the browser snapshot. Throttle intermediate writes,
// but always flush the final preview (including cancellation output).
func (r *sessionRunner) toolOutputReporter(turnID, sessionID, callID, occurrence string) func(string, bool) error {
	var mu sync.Mutex
	var last time.Time
	var failure error
	var previous string
	return func(text string, final bool) error {
		mu.Lock()
		defer mu.Unlock()
		if failure != nil {
			return failure
		}
		if !final && time.Since(last) < 250*time.Millisecond {
			return nil
		}
		preview := store.ToolOutputPreview(text)
		if preview == nil || text == previous {
			return nil
		}
		preview["tool_call_id"], preview["occurrence_id"] = callID, occurrence
		failure = r.store.AppendTurnEvent(r.engine.backgroundContext(), turnID, sessionID, "tool.output", preview)
		if failure != nil {
			return failure
		}
		last, previous = time.Now(), text
		r.engine.broadcast(sessionID, map[string]any{"type": "tool_activity_changed", "chat_jid": "gi:" + sessionID, "turn_id": turnID})
		return nil
	}
}
