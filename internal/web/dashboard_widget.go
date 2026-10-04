package web

import (
	"database/sql"
	"errors"
	"net/http"
)

func (s *Server) handleDashboardWidget(w http.ResponseWriter, r *http.Request, sessionID, widgetID string) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if widgetID == "" || len(widgetID) > 128 {
		writeJSON(w, 400, map[string]any{"error": "invalid widget ID"})
		return
	}
	artifact, err := s.store.DashboardWidget(r.Context(), sessionID, widgetID)
	if err != nil {
		code := 500
		if errors.Is(err, sql.ErrNoRows) {
			code = 404
		}
		writeJSON(w, code, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, artifact)
}
