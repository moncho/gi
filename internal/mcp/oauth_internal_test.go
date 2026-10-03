package mcp

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Credentials stored by URL alone (Pi < 1.0) move to the first server that
// loads them; Remove also deletes legacy state.
func TestCredentialStoreLegacyMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-auth.json")
	if err := os.WriteFile(path, []byte(`{"https://a.example/mcp": {"serverUrl": "https://a.example/mcp", "tokens": {"access_token": "t", "token_type": "Bearer"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewCredentialStore(path)
	state := store.load("my-server", "https://a.example/mcp")
	if state == nil || state.Tokens.AccessToken != "t" {
		t.Fatalf("legacy state not loaded: %+v", state)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"mcp__my_server|https://a.example/mcp"`) || strings.Contains(string(data), `"https://a.example/mcp": {`) {
		t.Fatalf("not migrated: %s", data)
	}
	if store.load("other", "https://a.example/mcp") != nil {
		t.Fatal("another server with the same URL took over the credentials")
	}
	if !store.Remove("my-server", "https://a.example/mcp") || store.Remove("my-server", "https://a.example/mcp") {
		t.Fatal("remove")
	}
}

func TestOAuthScopesAndTokenParsing(t *testing.T) {
	if got := stepUpScope("read write", "write admin"); got != "read write admin" {
		t.Fatal(got)
	}
	if stepUpScope("read", "") != "" {
		t.Fatal("step-up without challenged scopes")
	}
	tokens, err := parseTokens([]byte(`{"access_token": "a", "token_type": "Bearer", "scope": "", "refresh_token": null, "expires_in": "3600"}`))
	if err != nil || tokens.Scope != "" || tokens.RefreshToken != "" || tokens.ExpiresIn == nil || *tokens.ExpiresIn != 3600 {
		t.Fatalf("%+v %v", tokens, err)
	}
	if withScope(tokens, "read").Scope != "read" || withScope(&oauthTokens{Scope: "x"}, "read").Scope != "x" {
		t.Fatal("withScope")
	}
}

// A pasted redirect URL must match the sign-in's redirect URI (Pi 1.0.1):
// a server-specific path tells authorization servers apart.
func TestPastedRedirectMustMatchRedirectURI(t *testing.T) {
	redirect, _ := url.Parse("http://127.0.0.1:8123/callback/abc")
	for input, want := range map[string]string{
		"http://127.0.0.1:8123/callback/abc?code=c&state=s": "",
		"http://127.0.0.1:8123/callback?code=c&state=s":     "The redirect URL does not match this sign-in's redirect URI",
		"http://127.0.0.1:9000/callback/abc?code=c&state=s": "The redirect URL does not match this sign-in's redirect URI",
		"http://127.0.0.1:8123/callback/abc?code=c&state=x": "The redirect URL belongs to a different sign-in",
		"not a url": "Expected the full redirect URL from the browser address bar",
	} {
		code, _, err := codeFromRedirectURL(input, "s", redirect)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != want || (want == "" && code != "c") {
			t.Errorf("%s: %q %q, want %q", input, code, got, want)
		}
	}
}
