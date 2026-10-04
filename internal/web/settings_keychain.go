package web

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/rcarmo/gi/internal/keychain"
)

// keychain is the server's keychain (in its store), or nil without one.
func (s *Server) keychain() *keychain.Keychain {
	if s.store == nil {
		return nil
	}
	return keychain.New(s.store.DB())
}

// keychainWriteAllowed is the providers' write policy: HTTPS or loopback,
// same origin, JSON. It writes the refusal.
func keychainWriteAllowed(w http.ResponseWriter, r *http.Request) bool {
	return settingsWriteAllowed(w, r, "Keychain")
}

// settingsWriteAllowed applies the same policy to other settings writes;
// subject names them in errors ("Keychain", "Settings").
func settingsWriteAllowed(w http.ResponseWriter, r *http.Request, subject string) bool {
	if !providerWriteTransport(r) {
		writeJSON(w, 403, map[string]any{"error": subject + " changes require HTTPS or a loopback connection"})
		return false
	}
	if err := http.NewCrossOriginProtection().Check(r); err != nil {
		writeJSON(w, 403, map[string]any{"error": "Cross-origin " + strings.ToLower(subject) + " requests are not allowed"})
		return false
	}
	if media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); media != "application/json" {
		writeJSON(w, 415, map[string]any{"error": "JSON content type required"})
		return false
	}
	return true
}

// decodeKeychainRequest reads one JSON object into v; it writes the refusal.
func decodeKeychainRequest(w http.ResponseWriter, r *http.Request, v any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	if err := decoder.Decode(v); err != nil {
		writeJSON(w, 400, map[string]any{"error": "Invalid keychain request"})
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(w, 400, map[string]any{"error": "Expected one JSON object"})
		return false
	}
	return true
}

// handleKeychain is Piclaw's /agent/keychain: GET lists entries without
// secrets, POST adds or replaces one, DELETE removes one.
func (s *Server) handleKeychain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	kc := s.keychain()
	if kc == nil {
		writeJSON(w, 503, map[string]any{"error": "Keychain unavailable"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		entries, err := kc.List(r.Context())
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": err.Error()})
			return
		}
		if entries == nil {
			entries = []keychain.Meta{}
		}
		writeJSON(w, 200, map[string]any{"ok": true, "entries": entries, "enabled": kc.Enabled(), "can_write": providerWriteTransport(r)})
	case http.MethodPost:
		if !keychainWriteAllowed(w, r) {
			return
		}
		var body struct {
			Name      string  `json:"name"`
			Type      string  `json:"type"`
			Secret    string  `json:"secret"`
			Username  string  `json:"username"`
			UserNote  *string `json:"userNote"`
			AgentNote *string `json:"agentNote"`
		}
		if !decodeKeychainRequest(w, r, &body) {
			return
		}
		name := strings.TrimSpace(body.Name)
		if name == "" || body.Secret == "" {
			writeJSON(w, 400, map[string]any{"error": "Provide name and secret."})
			return
		}
		entry := keychain.Entry{Name: name, Type: body.Type, Secret: body.Secret, Username: strings.TrimSpace(body.Username), UserNote: body.UserNote, AgentNote: body.AgentNote}
		body.Secret = ""
		if err := kc.Set(r.Context(), entry); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "name": name})
	case http.MethodDelete:
		if !keychainWriteAllowed(w, r) {
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if !decodeKeychainRequest(w, r, &body) {
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			writeJSON(w, 400, map[string]any{"error": "Provide name."})
			return
		}
		removed, err := kc.Delete(r.Context(), strings.TrimSpace(body.Name))
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "removed": removed})
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		writeJSON(w, 405, map[string]any{"error": "Method not allowed"})
	}
}

// handleKeychainNotes is Piclaw's /agent/keychain/notes.
func (s *Server) handleKeychainNotes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	kc := s.keychain()
	if r.Method != http.MethodPost || kc == nil {
		w.Header().Set("Allow", "POST")
		writeJSON(w, 405, map[string]any{"error": "Method not allowed"})
		return
	}
	if !keychainWriteAllowed(w, r) {
		return
	}
	var body struct {
		Name      string `json:"name"`
		UserNote  string `json:"userNote"`
		AgentNote string `json:"agentNote"`
	}
	if !decodeKeychainRequest(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	updated, err := kc.UpdateNotes(r.Context(), name, body.UserNote, body.AgentNote)
	switch {
	case err != nil:
		writeJSON(w, 400, map[string]any{"error": err.Error()})
	case !updated:
		writeJSON(w, 404, map[string]any{"error": "Keychain entry not found: " + name})
	default:
		writeJSON(w, 200, map[string]any{"ok": true, "name": name, "userNote": body.UserNote, "agentNote": body.AgentNote})
	}
}

// handleKeychainReveal is Piclaw's /agent/keychain/reveal: the secret of one
// entry, after the master password.
func (s *Server) handleKeychainReveal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	kc := s.keychain()
	if r.Method != http.MethodPost || kc == nil {
		w.Header().Set("Allow", "POST")
		writeJSON(w, 405, map[string]any{"error": "Method not allowed"})
		return
	}
	if !keychainWriteAllowed(w, r) {
		return
	}
	var body struct {
		Name           string `json:"name"`
		MasterPassword string `json:"master_password"`
	}
	if !decodeKeychainRequest(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeJSON(w, 400, map[string]any{"error": "Provide name."})
		return
	}
	switch {
	case !kc.Enabled():
		writeJSON(w, 500, map[string]any{"error": "Keychain master key not configured on server."})
		return
	case body.MasterPassword == "":
		writeJSON(w, 401, map[string]any{"error": "Master password required.", "needs_master_password": true})
		return
	case !kc.MasterKeyMatches(body.MasterPassword):
		writeJSON(w, 401, map[string]any{"error": "Invalid master password.", "needs_master_password": true})
		return
	}
	entry, err := kc.Get(r.Context(), name)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	var username any
	if entry.Username != "" {
		username = entry.Username
	}
	writeJSON(w, 200, map[string]any{"ok": true, "name": entry.Name, "secret": entry.Secret, "username": username})
}
