package keychain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/store"
)

type piclawGolden struct {
	Key     string `json:"key"`
	Entries []struct {
		Name, Type, Secret, Username string
	} `json:"entries"`
	Rows []struct {
		Name          string `json:"name"`
		Type          string `json:"type"`
		Ciphertext    string `json:"ciphertext"`
		Nonce         string `json:"nonce"`
		Salt          string `json:"salt"`
		KDF           string `json:"kdf"`
		KDFIterations int    `json:"kdf_iterations"`
	} `json:"rows"`
	Injectable []struct {
		EnvName      string `json:"envName"`
		KeychainName string `json:"keychainName"`
	} `json:"injectable"`
	EnvNames map[string]*string `json:"envNames"`
	Shell    []struct {
		Command  string            `json:"command"`
		Env      map[string]string `json:"env"`
		Resolved *string           `json:"resolved"`
		Error    *string           `json:"error"`
	} `json:"shell"`
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "gi.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// Entries sealed by Piclaw's keychain (scripts/golden-keychain.mjs) open in
// gi's, and gi names, injects and substitutes them as Piclaw does.
func TestMatchesPiclaw(t *testing.T) {
	raw, err := os.ReadFile("testdata/piclaw-keychain.json")
	if err != nil {
		t.Fatal(err)
	}
	var g piclawGolden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s := openStore(t)
	for _, r := range g.Rows {
		b := func(v string) []byte { d, _ := base64.StdEncoding.DecodeString(v); return d }
		if _, err := s.DB().Exec(`insert into keychain_entries (name, type, ciphertext, nonce, salt, kdf, kdf_iterations, created_at, updated_at) values (?, ?, ?, ?, ?, ?, ?, '2026-10-04T00:00:00Z', '2026-10-04T00:00:00Z')`,
			r.Name, r.Type, b(r.Ciphertext), b(r.Nonce), b(r.Salt), r.KDF, r.KDFIterations); err != nil {
			t.Fatal(err)
		}
	}
	k := WithKey(s.DB(), g.Key)
	for _, want := range g.Entries {
		got, err := k.Get(ctx, want.Name)
		if err != nil || got.Secret != want.Secret || got.Username != want.Username || got.Type != want.Type {
			t.Fatalf("%s: %+v %v", want.Name, got, err)
		}
	}
	list, err := k.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{}
	for _, m := range list {
		if m.EnvVar != "" {
			vars[m.Name] = m.EnvVar
		}
	}
	want := map[string]string{}
	for _, e := range g.Injectable {
		want[e.KeychainName] = e.EnvName
	}
	if !reflect.DeepEqual(vars, want) {
		t.Fatalf("variables %v, want %v", vars, want)
	}
	for name, v := range g.EnvNames {
		if w := ""; v != nil && EnvName(name) != *v || v == nil && EnvName(name) != w {
			t.Errorf("EnvName(%q) = %q, want %v", name, EnvName(name), v)
		}
	}
	for _, c := range g.Shell {
		env, err := k.Environment(ctx, c.Command)
		if err != nil {
			t.Fatal(c.Command, err)
		}
		gotEnv := map[string]string{}
		for _, kv := range env {
			name, value, _ := strings.Cut(kv, "=")
			gotEnv[name] = value
		}
		if !reflect.DeepEqual(gotEnv, c.Env) {
			t.Errorf("%q: env %v, want %v", c.Command, gotEnv, c.Env)
		}
		resolved, err := k.ResolvePlaceholders(ctx, c.Command)
		switch {
		case c.Error != nil && (err == nil || err.Error() != *c.Error):
			t.Errorf("%q: error %v, want %q", c.Command, err, *c.Error)
		case c.Error == nil && (err != nil || resolved != *c.Resolved):
			t.Errorf("%q: %q %v, want %q", c.Command, resolved, err, *c.Resolved)
		}
	}
}

// Entries round-trip, keep notes unless given, and leave with Delete; the
// master key unlocks reveal; without a key nothing is sealed or injected.
func TestEntryLifecycle(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	k := WithKey(s.DB(), "master")
	note := "for deploys"
	if err := k.Set(ctx, Entry{Name: "github/deploy", Type: "basic", Secret: "tok", Username: "octo", UserNote: &note}); err != nil {
		t.Fatal(err)
	}
	if err := k.Set(ctx, Entry{Name: "github/deploy", Type: "bogus", Secret: "tok2"}); err != nil {
		t.Fatal(err)
	}
	e, err := k.Get(ctx, "github/deploy")
	if err != nil || e.Secret != "tok2" || e.Username != "" || e.Type != "secret" {
		t.Fatalf("%+v %v", e, err)
	}
	list, _ := k.List(ctx)
	if len(list) != 1 || list[0].UserNote != "for deploys" || list[0].EnvVar != "GITHUB_DEPLOY" || list[0].CreatedAt == "" {
		t.Fatalf("%+v", list)
	}
	var sealed []byte
	_ = s.DB().QueryRow(`select ciphertext from keychain_entries`).Scan(&sealed)
	if strings.Contains(string(sealed), "tok2") {
		t.Fatal("secret stored in the clear")
	}
	if ok, _ := k.UpdateNotes(ctx, "github/deploy", "u", "a"); !ok {
		t.Fatal("notes not updated")
	}
	if !k.MasterKeyMatches("master") || k.MasterKeyMatches("other") || k.MasterKeyMatches("") {
		t.Fatal("master key check")
	}
	if _, err := WithKey(s.DB(), "wrong").Get(ctx, "github/deploy"); err == nil {
		t.Fatal("opened with the wrong key")
	}
	t.Setenv("GITHUB_DEPLOY", "from-process")
	if env, _ := k.Environment(ctx, "echo $GITHUB_DEPLOY"); len(env) != 0 {
		t.Fatalf("process variable overridden: %v", env)
	}
	disabled := &Keychain{db: s.DB(), key: func() (string, error) { return "", ErrDisabled }}
	if err := disabled.Set(ctx, Entry{Name: "x", Secret: "y"}); err != ErrDisabled || disabled.Enabled() {
		t.Fatalf("disabled set: %v", err)
	}
	os.Unsetenv("GITHUB_DEPLOY")
	if env, err := disabled.Environment(ctx, "echo $GITHUB_DEPLOY"); err != nil || len(env) != 0 {
		t.Fatalf("disabled environment %v %v", env, err)
	}
	if ok, _ := k.Delete(ctx, "github/deploy"); !ok {
		t.Fatal("not deleted")
	}
	if ok, _ := k.Delete(ctx, "github/deploy"); ok {
		t.Fatal("deleted twice")
	}
}

func TestMasterKeyFromEnvironment(t *testing.T) {
	for _, name := range []string{"GI_KEYCHAIN_KEY", "GI_KEYCHAIN_KEY_FILE", "PICLAW_KEYCHAIN_KEY", "PICLAW_KEYCHAIN_KEY_FILE"} {
		t.Setenv(name, "")
	}
	if _, err := MasterKey(); err != ErrDisabled {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "key")
	_ = os.WriteFile(file, []byte(" from-file \n"), 0o600)
	t.Setenv("PICLAW_KEYCHAIN_KEY_FILE", file)
	if key, _ := MasterKey(); key != "from-file" {
		t.Fatal(key)
	}
	t.Setenv("GI_KEYCHAIN_KEY", "gi-key")
	if key, _ := MasterKey(); key != "gi-key" {
		t.Fatal(key)
	}
}
