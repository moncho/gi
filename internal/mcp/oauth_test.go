package mcp_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rcarmo/gi/internal/mcp/mcptest"
)

// fakeOAuth is an MCP server behind Pi-style OAuth: protected resource
// metadata, authorization server metadata, dynamic client registration,
// PKCE authorization codes and refresh tokens.
type fakeOAuth struct {
	*httptest.Server
	mu        sync.Mutex
	valid     map[string]bool // accepted access tokens
	challenge string          // PKCE challenge of the pending code
	issued    int
	refreshes int
}

func newFakeOAuth(t *testing.T) *fakeOAuth {
	t.Helper()
	fake := mcptest.New("")
	t.Cleanup(fake.Close)
	f := &fakeOAuth{valid: map[string]bool{}}
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
		writeJSON(w, map[string]any{"resource": f.URL + "/mcp", "authorization_servers": []string{f.URL}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"issuer": f.URL, "authorization_endpoint": f.URL + "/authorize", "token_endpoint": f.URL + "/token",
			"registration_endpoint": f.URL + "/register", "response_types_supported": []string{"code"}, "code_challenge_methods_supported": []string{"S256"}})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		var meta map[string]any
		_ = json.NewDecoder(r.Body).Decode(&meta)
		meta["client_id"] = "client-1"
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, meta)
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("client_id") != "client-1" || q.Get("code_challenge_method") != "S256" || q.Get("resource") != f.URL+"/mcp" {
			http.Error(w, "bad authorize request", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.challenge = q.Get("code_challenge")
		f.mu.Unlock()
		http.Redirect(w, r, q.Get("redirect_uri")+"?code=code-1&state="+q.Get("state"), http.StatusFound)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if r.Form.Get("code") != "code-1" || base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge {
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

// gi mcp login runs the authorization code flow (DCR, PKCE, loopback
// callback), list uses and refreshes the stored token, logout removes it.
func TestCLIOAuthLoginRefreshLogout(t *testing.T) {
	f := newFakeOAuth(t)
	opts := cliOptions(t)
	opts.CredentialsPath = filepath.Join(filepath.Dir(opts.UserPath), "mcp-auth.json")
	opts.OpenURL = func(u string) {
		go func() { // the "browser": follow the redirect to the loopback callback
			if resp, err := http.Get(u); err == nil {
				resp.Body.Close()
			}
		}()
	}
	if r := runCLI(t, opts, "add", "remote", "--url", f.URL+"/mcp"); r.code != 0 {
		t.Fatalf("%+v", r)
	}
	r := runCLI(t, opts, "list")
	if r.code != 1 || !strings.Contains(r.out, "remote: needs sign-in (codemode, global)") || !strings.Contains(r.out, "sign in with: gi mcp login remote") {
		t.Fatalf("before login: %+v", r)
	}
	r = runCLI(t, opts, "login", "remote")
	if r.code != 0 || !strings.Contains(r.out, `Sign in to MCP server "remote" in your browser:`) || !strings.HasSuffix(r.out, `Signed in to MCP server "remote" (4 tools).`) {
		t.Fatalf("login: %+v", r)
	}
	var stored map[string]struct {
		ServerURL string `json:"serverUrl"`
		Tokens    struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
		TokensExpireAt float64 `json:"tokensExpireAt"`
	}
	data, _ := os.ReadFile(opts.CredentialsPath)
	if err := json.Unmarshal(data, &stored); err != nil || stored[f.URL+"/mcp"].Tokens.AccessToken != "token-1" || stored[f.URL+"/mcp"].TokensExpireAt == 0 {
		t.Fatalf("stored credentials: %s", data)
	}
	if r := runCLI(t, opts, "login", "remote"); r.code != 0 || r.out != `Already signed in to MCP server "remote" (4 tools).` {
		t.Fatalf("second login: %+v", r)
	}
	// The server rejects the stored token: the connection refreshes it.
	f.mu.Lock()
	delete(f.valid, "token-1")
	f.mu.Unlock()
	if r := runCLI(t, opts, "list"); r.code != 0 || !strings.Contains(r.out, "remote: connected, 4 tools") {
		t.Fatalf("after revocation: %+v", r)
	}
	data, _ = os.ReadFile(opts.CredentialsPath)
	if f.refreshes != 1 || !strings.Contains(string(data), `"access_token": "token-2"`) {
		t.Fatalf("refresh: %d %s", f.refreshes, data)
	}
	if r := runCLI(t, opts, "logout", "remote"); r.code != 0 || r.out != `Signed out of MCP server "remote".` {
		t.Fatalf("logout: %+v", r)
	}
	if r := runCLI(t, opts, "logout", "remote"); r.out != `No stored credentials for MCP server "remote".` {
		t.Fatalf("second logout: %+v", r)
	}
	f.mu.Lock()
	f.valid = map[string]bool{}
	f.mu.Unlock()
	if r := runCLI(t, opts, "list"); r.code != 1 || !strings.Contains(r.out, "needs sign-in") {
		t.Fatalf("after logout: %+v", r)
	}
}
