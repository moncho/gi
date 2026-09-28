package turn

import (
	"context"
	"errors"

	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/topics"
)

// Active Steer binds to the observed run. After that latest run is released,
// it may launch the same queued row. Only explicit idle Steer bypasses Stop.
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
		if !errors.Is(err, store.ErrQueueConflict) {
			return err
		}
		launched, launchErr := e.launchQueueActionLocked(opCtx, runner, sessionID, queuedID, "", true, activeID)
		if launchErr != nil {
			return launchErr
		}
		if !launched {
			return store.ErrQueueConflict
		}
	}
	if bus := e.Topics(); bus != nil {
		bus.Publish(topics.Envelope{Topic: "session.queue", SessionID: sessionID, Type: "notice", Payload: map[string]any{"type": "queue_changed"}})
	}
	return nil
}
