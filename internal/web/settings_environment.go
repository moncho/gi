package web

import (
	"errors"
	"net/http"

	"github.com/rcarmo/gi/internal/environment"
	"github.com/rcarmo/gi/internal/shellenv"
)

// handleEnvironment is Settings → Environment (Piclaw's environment
// overrides): GET lists the runtime's variables with their overrides; POST
// {name, value} sets an override and {name, clear: true} removes it.
func (s *Server) handleEnvironment(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Environment settings are unavailable"})
		return
	}
	env := shellenv.Environment(s.store.DB())
	w.Header().Set("Cache-Control", "private, no-store")
	switch r.Method {
	case http.MethodGet:
		data, err := env.Data(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, data)
	case http.MethodPost:
		if !keychainWriteAllowed(w, r) {
			return
		}
		var req struct {
			Name  string  `json:"name"`
			Value *string `json:"value"`
			Clear bool    `json:"clear"`
		}
		if !decodeKeychainRequest(w, r, &req) {
			return
		}
		var (
			data environment.Data
			err  error
		)
		if req.Clear {
			data, err = env.Clear(r.Context(), req.Name)
		} else {
			value := ""
			if req.Value != nil {
				value = *req.Value
			}
			data, err = env.Set(r.Context(), req.Name, value)
		}
		if errors.Is(err, environment.ErrInvalidName) || errors.Is(err, environment.ErrKeychainName) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, data)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
