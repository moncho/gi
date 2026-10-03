package mcptest

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// OAuthServer is an MCP server behind Pi-style OAuth: protected resource
// metadata, authorization server metadata, dynamic client registration,
// PKCE authorization codes and refresh tokens.
type OAuthServer struct {
	*httptest.Server
	mu        sync.Mutex
	valid     map[string]bool // accepted access tokens
	challenge string          // PKCE challenge of the pending code
	issued    int
	refreshes int
	// Iss is added as the iss parameter of authorization responses.
	Iss string
	// Advertise is the authorization server named in the resource metadata.
	Advertise string
	// CIMD advertises Client ID Metadata Documents for public clients and
	// accepts any https client ID instead of the registered one.
	CIMD bool
	// ClientIDs and RedirectURIs are what authorization requests used;
	// Registrations counts dynamic client registrations.
	ClientIDs, RedirectURIs []string
	Registrations           int
	redirect                string // redirect_uri of the pending code
}

// NewOAuth starts the fake MCP server (tools at /mcp) behind OAuth.
func NewOAuth(t testing.TB) *OAuthServer {
	t.Helper()
	fake := New("")
	t.Cleanup(fake.Close)
	f := &OAuthServer{valid: map[string]bool{}}
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		ok := f.valid[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
		f.mu.Unlock()
		if !ok {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource/mcp"`, f.URL))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fake.HTTP.Config.Handler.ServeHTTP(w, r)
	})
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		as := f.URL
		if f.Advertise != "" {
			as = f.Advertise
		}
		writeJSON(w, map[string]any{"resource": f.URL + "/mcp", "authorization_servers": []string{as}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		meta := map[string]any{"issuer": f.URL, "authorization_endpoint": f.URL + "/authorize", "token_endpoint": f.URL + "/token",
			"registration_endpoint": f.URL + "/register", "response_types_supported": []string{"code"}, "code_challenge_methods_supported": []string{"S256"}}
		if f.CIMD {
			meta["client_id_metadata_document_supported"] = true
			meta["token_endpoint_auth_methods_supported"] = []string{"none"}
		}
		if f.Iss != "" {
			meta["authorization_response_iss_parameter_supported"] = true
		}
		writeJSON(w, meta)
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		var meta map[string]any
		_ = json.NewDecoder(r.Body).Decode(&meta)
		meta["client_id"] = "client-1"
		f.mu.Lock()
		f.Registrations++
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, meta)
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		clientOK := q.Get("client_id") == "client-1" || (f.CIMD && strings.HasPrefix(q.Get("client_id"), "https://"))
		if !clientOK || q.Get("code_challenge_method") != "S256" || q.Get("resource") != f.URL+"/mcp" {
			http.Error(w, "bad authorize request", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.challenge, f.redirect = q.Get("code_challenge"), q.Get("redirect_uri")
		f.ClientIDs = append(f.ClientIDs, q.Get("client_id"))
		f.RedirectURIs = append(f.RedirectURIs, q.Get("redirect_uri"))
		f.mu.Unlock()
		target := q.Get("redirect_uri") + "?code=code-1&state=" + q.Get("state")
		if f.Iss != "" {
			target += "&iss=" + f.Iss
		}
		http.Redirect(w, r, target, http.StatusFound)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if r.Form.Get("code") != "code-1" || base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge || r.Form.Get("redirect_uri") != f.redirect {
				w.WriteHeader(http.StatusBadRequest)
				writeJSON(w, map[string]string{"error": "invalid_grant"})
				return
			}
		case "refresh_token":
			if r.Form.Get("refresh_token") != "refresh-1" {
				w.WriteHeader(http.StatusBadRequest)
				writeJSON(w, map[string]string{"error": "invalid_grant"})
				return
			}
			f.refreshes++
		}
		f.issued++
		token := fmt.Sprintf("token-%d", f.issued)
		f.valid[token] = true
		writeJSON(w, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 3600, "refresh_token": "refresh-1"})
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

// Revoke rejects an access token from now on (all tokens when empty).
func (f *OAuthServer) Revoke(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if token == "" {
		f.valid = map[string]bool{}
		return
	}
	delete(f.valid, token)
}

// Issued counts issued access tokens; Refreshes counts refresh grants.
func (f *OAuthServer) Issued() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.issued
}

func (f *OAuthServer) Refreshes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refreshes
}
