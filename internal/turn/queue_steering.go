package turn

import (
	"context"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/topics"
)

// Active Steer binds to the observed run. An explicitly empty active ID means
// an observed idle session, never automatic fallback from a stale active run.
func (e *Engine) SteerQueuedTurn(ctx context.Context, sessionID, queuedID, activeID string) error {
	runner := e.runner(sessionID)
	runner.mu.Lock()
	defer runner.mu.Unlock()
	opCtx := store.CoordinationContext(ctx, e.backgroundContext())
	if activeID == "" {
		hold, err := e.store.WebQueueHold(opCtx, sessionID)
		if err != nil {
			return err
		}
		launched, err := e.launchTurnWithQueueActionLocked(opCtx, runner, sessionID, queuedID, hold, true)
		if err != nil {
			return err
		}
		if !launched {
			return store.ErrQueueConflict
		}
	} else if err := e.store.SteerQueuedTurn(opCtx, sessionID, queuedID, activeID); err != nil {
		return err
	}
	if bus := e.Topics(); bus != nil {
		bus.Publish(topics.Envelope{Topic: "session.queue", SessionID: sessionID, Type: "notice", Payload: map[string]any{"type": "queue_changed"}})
	}
	return nil
}
