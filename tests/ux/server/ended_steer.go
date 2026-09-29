package main

import (
	"fmt"
	"github.com/rcarmo/gi/internal/store"
	"net/http"
)

// A disposable claim fixture, not a production route. The real Steer handler,
// launch, persistence and local provider process the selected row unchanged.
func endedSteerFixture(s *store.Store, next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /__test/ended-steer/{session}/{action}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session := r.PathValue("session")
		var err error
		switch r.PathValue("action") {
		case "seed":
			_, err = s.CreateTurnWithStatus(ctx, "observed", session, "running", "prior fixture turn", nil)
			if err == nil {
				var ok bool
				ok, err = s.ClaimSessionActiveTurn(ctx, session, "observed", "fixture", "observed")
				if err == nil && !ok {
					err = fmt.Errorf("claim conflict")
				}
			}
			if err == nil {
				_, err = s.CreateTurnWithStatus(ctx, "selected", session, "queued", "ended steer selected instruction", map[string]any{"model": "ux-local/gate"})
			}
		case "end":
			err = s.UpdateTurnStatusAndPhase(ctx, "observed", "completed", "completed")
			if err == nil {
				err = s.ReleaseSessionActiveTurn(ctx, session, "observed")
			}
		case "cancel":
			err = s.UpdateTurnStatusAndPhase(ctx, "observed", "cancelling", "cancelling")
		case "release-cancelled":
			err = s.UpdateTurnStatusAndPhase(ctx, "observed", "cancelled", "aborted")
			if err == nil {
				err = s.ReleaseSessionActiveTurn(ctx, session, "observed")
			}
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(204)
	})
	mux.Handle("/", next)
	return mux
}
