package web

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/keychain"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/turn"
)

// Settings → Keychain's API: entries are added, listed without secrets
// (with their shell variables), revealed only with the master password,
// and deleted; writes need loopback or HTTPS and JSON, as provider keys do.
func TestKeychainSettingsAPI(t *testing.T) {
	t.Setenv("GI_KEYCHAIN_KEY", "fixture-master")
	st, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(st, turn.New(st), config.RuntimeConfig{WorkspaceRoot: t.TempDir()})
	call := func(method, path, body, remote string) (int, map[string]any) {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.RemoteAddr, req.Host = remote, "127.0.0.1:8090"
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		srv.Handler().ServeHTTP(res, req)
		var out map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &out)
		if strings.Contains(res.Body.String(), "s3cret") && !strings.HasSuffix(path, "/reveal") {
			t.Fatalf("%s %s leaked the secret: %s", method, path, res.Body.String())
		}
		return res.Code, out
	}
	const local = "127.0.0.1:1"
	if code, out := call("POST", "/api/settings/keychain", `{"name":"fixtures/kc-x.v1","type":"basic","secret":"s3cret","username":"octo","userNote":"note"}`, local); code != 200 || out["ok"] != true {
		t.Fatal(code, out)
	}
	if code, out := call("POST", "/api/settings/keychain", `{"name":"x","secret":"s3cret"}`, "192.0.2.1:1"); code != 403 {
		t.Fatal("remote write", code, out)
	}
	if code, _ := call("POST", "/api/settings/keychain", `{"name":"x"}`, local); code != 400 {
		t.Fatal("no secret", code)
	}
	code, out := call("GET", "/api/settings/keychain", "", local)
	entries, _ := out["entries"].([]any)
	if code != 200 || len(entries) != 1 || out["enabled"] != true {
		t.Fatal(code, out)
	}
	e := entries[0].(map[string]any)
	if e["name"] != "fixtures/kc-x.v1" || e["type"] != "basic" || e["envVar"] != "FIXTURES_KC_X_V1" || e["userNote"] != "note" || e["updatedAt"] == "" {
		t.Fatal(e)
	}
	if code, out := call("POST", "/api/settings/keychain/reveal", `{"name":"fixtures/kc-x.v1"}`, local); code != 401 || out["needs_master_password"] != true {
		t.Fatal(code, out)
	}
	if code, out := call("POST", "/api/settings/keychain/reveal", `{"name":"fixtures/kc-x.v1","master_password":"nope"}`, local); code != 401 || out["error"] != "Invalid master password." {
		t.Fatal(code, out)
	}
	if code, out := call("POST", "/api/settings/keychain/reveal", `{"name":"fixtures/kc-x.v1","master_password":"fixture-master"}`, local); code != 200 || out["secret"] != "s3cret" || out["username"] != "octo" {
		t.Fatal(code, out)
	}
	if code, out := call("POST", "/api/settings/keychain/notes", `{"name":"fixtures/kc-x.v1","userNote":"u","agentNote":"a"}`, local); code != 200 {
		t.Fatal(code, out)
	}
	if code, out := call("DELETE", "/api/settings/keychain", `{"name":"fixtures/kc-x.v1"}`, local); code != 200 || out["removed"] != true {
		t.Fatal(code, out)
	}
	if _, out := call("GET", "/api/settings/keychain", "", local); len(out["entries"].([]any)) != 0 {
		t.Fatal(out)
	}
}

// The web shell tool injects the keychain as the agent's shell does.
func TestWebShellToolUsesKeychain(t *testing.T) {
	t.Setenv("GI_KEYCHAIN_KEY", "fixture-master")
	st, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(st, turn.New(st), config.RuntimeConfig{WorkspaceRoot: t.TempDir()})
	if err := srv.keychain().Set(t.Context(), keychain.Entry{Name: "deploy/token", Secret: "tok-1"}); err != nil {
		t.Fatal(err)
	}
	out := executeShellTool(t.Context(), srv.keychain(), `printf '%s|%s|' "$DEPLOY_TOKEN" keychain:deploy/token; env | grep -c tok-1`)
	if out.Result != "tok-1|tok-1|1\n" {
		t.Fatalf("%+v", out)
	}
}
