package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// generalSettings is the editable part of Settings -> General (Piclaw's
// /agent/settings/general instance configuration).
type generalSettings struct {
	WorkspaceUploadLimitMB int `json:"workspaceUploadLimitMb"`
}

const (
	defaultUploadLimitMB = 256
	maxUploadLimitMB     = 1024
)

func (s *Server) generalSettings(ctx context.Context) generalSettings {
	out := generalSettings{WorkspaceUploadLimitMB: defaultUploadLimitMB}
	if s.store == nil {
		return out
	}
	var raw []byte
	if err := s.store.DB().QueryRowContext(ctx, `select value from kv_store where namespace='settings' and key='general'`).Scan(&raw); err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	if out.WorkspaceUploadLimitMB < 1 || out.WorkspaceUploadLimitMB > maxUploadLimitMB {
		out.WorkspaceUploadLimitMB = defaultUploadLimitMB
	}
	return out
}

// handleGeneralSettings reads (GET) or saves (POST) the editable General
// settings; the response is {ok, settings}.
func (s *Server) handleGeneralSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "settings": s.generalSettings(r.Context())})
	case http.MethodPost:
		if s.store == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Settings are unavailable"})
			return
		}
		if !settingsWriteAllowed(w, r, "Settings") {
			return
		}
		var req generalSettings
		if !decodeKeychainRequest(w, r, &req) {
			return
		}
		if req.WorkspaceUploadLimitMB < 1 || req.WorkspaceUploadLimitMB > maxUploadLimitMB {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Upload limit must be between 1 and 1024 MB"})
			return
		}
		raw, _ := json.Marshal(req)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := s.store.DB().ExecContext(r.Context(), `insert into kv_store(namespace,key,value,created_at,updated_at) values('settings','general',?,?,?) on conflict(namespace,key) do update set value=excluded.value, updated_at=excluded.updated_at`, raw, now, now); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "settings": s.generalSettings(r.Context())})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
