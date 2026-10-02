package mcp

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// The callback serves Pi's sign-in pages with gi's logo.
func TestOAuthCallbackPages(t *testing.T) {
	cs, err := listenForCallback("127.0.0.1", "127.0.0.1", "/callback", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.close()
	get := func(query string) (int, string, string) {
		resp, err := http.Get(cs.redirectURL + query)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		<-cs.results
		return resp.StatusCode, string(body), resp.Header.Get("Cache-Control")
	}
	status, body, cache := get("?code=c&state=s")
	if status != 200 || cache != "no-store" || !strings.Contains(body, "<h1>Authentication successful</h1>") || !strings.Contains(body, "Signed in to the MCP server. You may now close this page.") || !strings.Contains(body, `src="data:image/png;base64,`) {
		t.Fatalf("success page %d %q", status, cache)
	}
	status, body, _ = get("?error=access_denied&error_description=" + "User+%3Cdenied%3E&state=s")
	if status != 200 || !strings.Contains(body, "<h1>Authentication failed</h1>") || !strings.Contains(body, "Authorization failed. You may close this window.") || !strings.Contains(body, `<div class="details">User &lt;denied&gt;</div>`) {
		t.Fatalf("error page %d:\n%s", status, body)
	}
	if status, body, _ = get("?state=s"); status != 400 || !strings.Contains(body, "Missing authorization code") {
		t.Fatalf("missing code %d", status)
	}
}
