package inference

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcarmo/go-ai/oauth"
)

// Copilot credentials as Pi keeps them: the stored token until it expires,
// then one refresh under Pi's lock that stores the token, its expiry and the
// account's available models, keeping the entry's other fields.
func TestCopilotAuthRefreshesLikePi(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GI_CODING_AGENT_DIR", t.TempDir()) // empty: Pi's file is used
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	authPath := filepath.Join(dir, "auth.json")
	write := func(body string) {
		if err := os.WriteFile(authPath, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	read := func() map[string]map[string]any {
		raw, err := os.ReadFile(authPath)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	var calls atomic.Int32
	previous := refreshCopilotCredentials
	t.Cleanup(func() { refreshCopilotCredentials = previous })
	refreshCopilotCredentials = func(_ context.Context, refresh, enterprise string) (*oauth.Credentials, error) {
		calls.Add(1)
		if refresh != "gh-refresh" || enterprise != "ghe.example.com" {
			t.Errorf("refresh %q %q", refresh, enterprise)
		}
		// Pi's lock is held while refreshing.
		if _, err := os.Stat(authPath + ".lock"); err != nil {
			t.Errorf("auth.json.lock not held: %v", err)
		}
		return &oauth.Credentials{Refresh: refresh, Access: "tid=1;proxy-ep=proxy.enterprise.githubcopilot.com;exp=1",
			Expires: time.Now().Add(time.Hour).UnixMilli(), Extra: map[string]any{"availableModelIds": []string{"gpt-5", "claude-4"}}}, nil
	}

	// Expired: refresh once and store Pi's fields.
	write(`{"github-copilot":{"type":"oauth","refresh":"gh-refresh","access":"old","expires":1,"enterpriseUrl":"ghe.example.com","note":"kept"},"openai":{"type":"api_key","apiKey":"k"}}`)
	token, base, err := loadAuth("github-copilot")
	if err != nil || token != "tid=1;proxy-ep=proxy.enterprise.githubcopilot.com;exp=1" || base != "https://api.enterprise.githubcopilot.com" {
		t.Fatalf("%q %q %v", token, base, err)
	}
	entry := read()["github-copilot"]
	if entry["access"] != token || entry["note"] != "kept" || entry["enterpriseUrl"] != "ghe.example.com" || entry["type"] != "oauth" ||
		entry["expires"].(float64) < float64(time.Now().UnixMilli()) || len(entry["availableModelIds"].([]any)) != 2 {
		t.Fatalf("entry %+v", entry)
	}
	if read()["openai"]["apiKey"] != "k" {
		t.Fatal("other providers changed")
	}
	if _, err := os.Stat(authPath + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock left behind: %v", err)
	}
	// The listing uses the stored available models.
	if creds := authEntryToOAuthCredentials(authEntry{AvailableModelIDs: []string{"gpt-5"}}); creds.Extra["availableModelIds"] == nil {
		t.Fatal("availableModelIds not carried")
	}

	// Valid: the stored token, no refresh.
	if _, _, err := loadAuth("github-copilot"); err != nil || calls.Load() != 1 {
		t.Fatalf("refreshed again: %d %v", calls.Load(), err)
	}

	// Another process refreshed while gi waited for the lock: its token is used.
	expiredLocally := authEntry{Refresh: "gh-refresh", Access: "old", Expires: 1}
	token, _, err = copilotAuth(expiredLocally)
	if err != nil || token != entry["access"] || calls.Load() != 1 {
		t.Fatalf("%q %d %v", token, calls.Load(), err)
	}

	// Startup refreshes only expired credentials.
	if err := RefreshExpiredCopilotCredentials(); err != nil || calls.Load() != 1 {
		t.Fatalf("current credentials refreshed: %d %v", calls.Load(), err)
	}
	write(`{"github-copilot":{"type":"oauth","refresh":"gh-refresh","access":"old","expires":1,"enterpriseUrl":"ghe.example.com"}}`)
	if err := RefreshExpiredCopilotCredentials(); err != nil || calls.Load() != 2 || read()["github-copilot"]["access"] == "old" {
		t.Fatalf("expired credentials not refreshed: %d %v", calls.Load(), err)
	}

	// A failed refresh (Pi: the model fetch failing too) leaves the file.
	write(`{"github-copilot":{"type":"oauth","refresh":"gh-refresh","access":"old","expires":1,"enterpriseUrl":"ghe.example.com"}}`)
	refreshCopilotCredentials = func(context.Context, string, string) (*oauth.Credentials, error) {
		return nil, errors.New("HTTP 500: models")
	}
	if _, _, err := loadAuth("github-copilot"); err == nil || err.Error() != "refresh copilot token: HTTP 500: models" {
		t.Fatal(err)
	}
	if read()["github-copilot"]["access"] != "old" {
		t.Fatal("failed refresh changed auth.json")
	}
}
