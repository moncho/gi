package mcp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
