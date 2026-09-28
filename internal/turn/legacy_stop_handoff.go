package turn

import "context"

func (e *Engine) handoffLegacyStoppedQueues(ctx context.Context) error {
	ids, err := e.store.LegacyStopQueueHandoffs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		// Normal continuation owns claim/session fences and launch rollback. Do not
		// retire the migration marker on failure; a later startup can retry safely.
		if _, err := e.ContinueSession(ctx, id); err != nil {
			return err
		}
		if err := e.store.CompleteLegacyStopQueueHandoff(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
