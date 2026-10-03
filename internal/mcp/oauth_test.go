package mcp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	gimcp "github.com/rcarmo/gi/internal/mcp"
	"github.com/rcarmo/gi/internal/mcp/mcptest"
)

// gi mcp login runs the authorization code flow (DCR, PKCE, loopback
// callback), list uses and refreshes the stored token, logout removes it.
func TestCLIOAuthLoginRefreshLogout(t *testing.T) {
	f := mcptest.NewOAuth(t)
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
	key := "mcp__remote|" + f.URL + "/mcp" // per server name and URL (Pi 1.0)
	if err := json.Unmarshal(data, &stored); err != nil || stored[key].Tokens.AccessToken != "token-1" || stored[key].TokensExpireAt == 0 {
		t.Fatalf("stored credentials: %s", data)
	}
	if r := runCLI(t, opts, "login", "remote"); r.code != 0 || r.out != `Already signed in to MCP server "remote" (4 tools).` {
		t.Fatalf("second login: %+v", r)
	}
	// The server rejects the stored token: the connection refreshes it.
	f.Revoke("token-1")
	if r := runCLI(t, opts, "list"); r.code != 0 || !strings.Contains(r.out, "remote: connected, 4 tools") {
		t.Fatalf("after revocation: %+v", r)
	}
	data, _ = os.ReadFile(opts.CredentialsPath)
	if f.Refreshes() != 1 || !strings.Contains(string(data), `"access_token": "token-2"`) {
		t.Fatalf("refresh: %d %s", f.Refreshes(), data)
	}
	if r := runCLI(t, opts, "logout", "remote"); r.code != 0 || r.out != `Signed out of MCP server "remote".` {
		t.Fatalf("logout: %+v", r)
	}
	if r := runCLI(t, opts, "logout", "remote"); r.out != `No stored credentials for MCP server "remote".` {
		t.Fatalf("second logout: %+v", r)
	}
	f.Revoke("")
	if r := runCLI(t, opts, "list"); r.code != 1 || !strings.Contains(r.out, "needs sign-in") {
		t.Fatalf("after logout: %+v", r)
	}
}

func loginOptions(t *testing.T) gimcp.CLIOptions {
	opts := cliOptions(t)
	opts.OpenURL = func(u string) {
		go func() {
			if resp, err := http.Get(u); err == nil {
				resp.Body.Close()
			}
		}()
	}
	return opts
}

// RFC 9207: an authorization response whose iss names another server is
// rejected before the code is exchanged.
func TestCLIOAuthRejectsWrongIssuer(t *testing.T) {
	f := mcptest.NewOAuth(t)
	f.Iss = "https://evil.example"
	opts := loginOptions(t)
	if r := runCLI(t, opts, "add", "remote", "--url", f.URL+"/mcp"); r.code != 0 {
		t.Fatalf("%+v", r)
	}
	r := runCLI(t, opts, "login", "remote")
	if r.code != 1 || !strings.Contains(r.err, `OAuth issuer mismatch: expected "`+f.URL+`", received "https://evil.example"`) || f.Issued() != 0 {
		t.Fatalf("%+v issued=%d", r, f.Issued())
	}
}

// oauth.authServerMetadataUrl replaces discovery for servers that advertise
// a wrong authorization server.
func TestCLIOAuthConfiguredMetadataURL(t *testing.T) {
	f := mcptest.NewOAuth(t)
	f.Advertise = "https://wrong.example"
	opts := loginOptions(t)
	_ = os.MkdirAll(filepath.Dir(opts.UserPath), 0o755)
	cfg := fmt.Sprintf(`{"mcpServers": {"remote": {"url": %q, "oauth": {"authServerMetadataUrl": %q}}}}`, f.URL+"/mcp", f.URL+"/.well-known/oauth-authorization-server")
	if err := os.WriteFile(opts.UserPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := runCLI(t, opts, "login", "remote"); r.code != 0 || !strings.HasSuffix(r.out, `Signed in to MCP server "remote" (4 tools).`) {
		t.Fatalf("%+v", r)
	}
}

// oauth.clientRegistration "cimd" (Pi 1.0.1) identifies with pi's Client ID
// Metadata Document instead of registering: a server-specific document and
// redirect URI without RFC 9207 iss support, the shared ones with it, and an
// error when the authorization server does not support documents.
func TestCLIOAuthClientIDMetadataDocument(t *testing.T) {
	for _, c := range []struct {
		name          string
		cimd, iss     bool
		client, error string
	}{
		{name: "server-specific", cimd: true, client: `^https://pi\.dev/oauth/([A-Za-z0-9_-]{12})/client\.json$`},
		{name: "with iss", cimd: true, iss: true, client: `^https://pi\.dev/oauth/client\.json$`},
		{name: "unsupported", error: `The authorization server does not support Client ID Metadata Documents for public clients; remove oauth.clientRegistration "cimd"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := mcptest.NewOAuth(t)
			f.CIMD = c.cimd
			if c.iss {
				f.Iss = f.URL
			}
			opts := loginOptions(t)
			opts.CredentialsPath = filepath.Join(filepath.Dir(opts.UserPath), "mcp-auth.json")
			config := fmt.Sprintf(`{"mcpServers": {"remote": {"url": %q, "oauth": {"clientRegistration": "cimd"}}}}`, f.URL+"/mcp")
			_ = os.MkdirAll(filepath.Dir(opts.UserPath), 0o755)
			if err := os.WriteFile(opts.UserPath, []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
			r := runCLI(t, opts, "login", "remote")
			if c.error != "" {
				if r.code != 1 || !strings.Contains(r.err, c.error) || f.Registrations != 0 {
					t.Fatalf("%+v registrations=%d", r, f.Registrations)
				}
				return
			}
			if r.code != 0 || !strings.HasSuffix(r.out, `Signed in to MCP server "remote" (4 tools).`) || f.Registrations != 0 || len(f.ClientIDs) != 1 {
				t.Fatalf("%+v registrations=%d clients=%v", r, f.Registrations, f.ClientIDs)
			}
			m := regexp.MustCompile(c.client).FindStringSubmatch(f.ClientIDs[0])
			if m == nil {
				t.Fatalf("client ID %s", f.ClientIDs[0])
			}
			wantPath := "/callback"
			if len(m) > 1 {
				wantPath += "/" + m[1]
			}
			if !strings.HasPrefix(f.RedirectURIs[0], "http://127.0.0.1:") || !strings.HasSuffix(f.RedirectURIs[0], wantPath) {
				t.Fatalf("redirect URI %s, want path %s", f.RedirectURIs[0], wantPath)
			}
			// A document is not stored: the next sign-in uses it again.
			if data, _ := os.ReadFile(opts.CredentialsPath); strings.Contains(string(data), "clientInformation") {
				t.Fatalf("stored client: %s", data)
			}
		})
	}
}
