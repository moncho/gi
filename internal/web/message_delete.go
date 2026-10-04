package web

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/rcarmo/gi/internal/store"
)

func (s *Server) handleMessageDelete(w http.ResponseWriter, r *http.Request, sessionID, messageID string) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		w.WriteHeader(405)
		return
	}
	cascade := r.URL.Query().Get("cascade")
	if cascade != "" && cascade != "false" && cascade != "true" {
		writeJSON(w, 400, map[string]any{"error": "cascade must be true or false"})
		return
	}
	ids, err := s.store.DeleteMessageWithReplies(r.Context(), sessionID, messageID, cascade == "true")
	if err != nil {
		code := 500
		if errors.Is(err, sql.ErrNoRows) {
			code = 404
		} else if errors.Is(err, store.ErrMessageDeleteBusy) || errors.Is(err, store.ErrMessageDeleteProtected) {
			code = 409
		}
		writeJSON(w, code, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "deleted": ids, "ids": ids})
}
