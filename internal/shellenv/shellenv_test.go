package shellenv

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/environment"
	"github.com/rcarmo/gi/internal/keychain"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/tools"
)

func envValue(env []string, name string) (string, bool) {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			return v, true
		}
	}
	return "", false
}

// Settings overrides reach every command; keychain variables are injected
// only when named, never replace a variable already set, and can be
// neither listed nor overridden (Piclaw 3.2.5 shell-environment rules).
func TestPrepareOverridesAndKeychain(t *testing.T) {
	t.Setenv("GI_KEYCHAIN_KEY", "master")
	t.Setenv("GI_SHELLENV_INHERITED", "inherited")
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "gi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	kc := keychain.New(s.DB())
	for _, e := range []keychain.Entry{
		{Name: "fixtures/kcc-x", Type: "secret", Secret: "slash"},
		{Name: "fixtures.kcc.x", Type: "secret", Secret: "dots"},
		{Name: "lower_name", Type: "secret", Secret: "lower"},
		{Name: "nouser", Type: "secret", Secret: "s"},
	} {
		if err := kc.Set(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	env := Environment(s.DB())

	// Keychain names are refused and not listed; others validate as identifiers.
	if _, err := env.Set(ctx, "FIXTURES_KCC_X", "x"); err != environment.ErrKeychainName {
		t.Fatal("keychain variable overridable:", err)
	}
	if _, err := env.Set(ctx, "1BAD", "x"); err != environment.ErrInvalidName {
		t.Fatal(err)
	}
	if _, err := env.Set(ctx, "GI_SHELLENV_INHERITED", "override"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Set(ctx, "GI_SHELLENV_NEW", "new"); err != nil {
		t.Fatal(err)
	}
	data, err := env.Data(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]environment.Row{}
	for _, r := range data.Variables {
		seen[r.Name] = r
	}
	if _, ok := seen["PATH"]; !ok {
		t.Fatal("PATH not listed")
	}
	if seen["GI_SHELLENV_INHERITED"].Value != "override" || !seen["GI_SHELLENV_NEW"].Overridden {
		t.Fatal(seen["GI_SHELLENV_INHERITED"], seen["GI_SHELLENV_NEW"])
	}
	if _, ok := seen["FIXTURES_KCC_X"]; ok {
		t.Fatal("keychain variable listed")
	}

	p, err := Prepare(ctx, s.DB(), "", `echo '$FIXTURES_KCC_X' %lower_name% $LOWER_NAME`)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := envValue(p.Env, "FIXTURES_KCC_X"); v != "dots" {
		t.Fatal("byte-order winner not injected:", v)
	}
	if v, _ := envValue(p.Env, "lower_name"); v != "lower" {
		t.Fatal("identifier name not kept:", v)
	}
	if _, ok := envValue(p.Env, "LOWER_NAME"); ok {
		t.Fatal("case-insensitive reference injected")
	}
	if _, ok := envValue(p.Env, "NOUSER"); ok {
		t.Fatal("unreferenced entry injected")
	}
	if v, _ := envValue(p.Env, "GI_SHELLENV_INHERITED"); v != "override" {
		t.Fatal("override not applied:", v)
	}

	// Clearing restores the inherited value, or removes the variable.
	env.Clear(ctx, "GI_SHELLENV_INHERITED")
	env.Clear(ctx, "GI_SHELLENV_NEW")
	p, _ = Prepare(ctx, s.DB(), "", "true")
	if v, _ := envValue(p.Env, "GI_SHELLENV_INHERITED"); v != "inherited" {
		t.Fatal("inherited value not restored:", v)
	}
	if _, ok := envValue(p.Env, "GI_SHELLENV_NEW"); ok {
		t.Fatal("cleared override still set")
	}

	// A keychain value never replaces a variable the environment has.
	env.Set(ctx, "NOUSER", "from-env")
	p, _ = Prepare(ctx, s.DB(), "", "echo $NOUSER")
	if v, _ := envValue(p.Env, "NOUSER"); v != "from-env" {
		t.Fatal(v)
	}

	// Unresolvable placeholders fail before anything runs.
	for _, cmd := range []string{"echo keychain:missing/entry", "echo keychain:nouser:username"} {
		if _, err := Prepare(ctx, s.DB(), "", cmd); err == nil {
			t.Fatal("placeholder resolved:", cmd)
		}
	}
}

func TestShellCandidatesFollowPiclaw(t *testing.T) {
	exists := func(set ...string) func(string) bool {
		return func(p string) bool {
			for _, s := range set {
				if s == p {
					return true
				}
			}
			return false
		}
	}
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	names := func(cs []tools.ShellConfig) string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Shell+" "+strings.Join(c.Args, " "))
		}
		return strings.Join(out, "|")
	}
	if _, err := tools.ShellCandidates("/no/such/sh", "linux", env(nil), exists()); err == nil || !strings.Contains(err.Error(), "/no/such/sh") {
		t.Fatal(err)
	}
	cs, _ := tools.ShellCandidates("/opt/sh", "linux", env(nil), exists("/opt/sh"))
	if got := names(cs); got != "/opt/sh -c" {
		t.Fatal(got)
	}
	cs, _ = tools.ShellCandidates("", "linux", env(map[string]string{"SHELL": "/usr/bin/zsh"}), exists("/usr/bin/zsh", "/bin/bash"))
	if got := names(cs); got != "/usr/bin/zsh -c|/bin/bash -c|bash -c" {
		t.Fatal(got)
	}
	cs, _ = tools.ShellCandidates("", "linux", env(map[string]string{"SHELL": "/missing"}), exists())
	if got := names(cs); got != "bash -c" {
		t.Fatal(got)
	}
	cs, _ = tools.ShellCandidates("", "windows", env(map[string]string{"ComSpec": `C:\Windows\system32\cmd.exe`}), exists(`C:\Program Files\PowerShell\7\pwsh.exe`))
	want := `C:\Program Files\PowerShell\7\pwsh.exe -NoProfile -Command|pwsh.exe -NoProfile -Command|powershell.exe -NoProfile -Command|C:\Windows\system32\cmd.exe /c|cmd.exe /c`
	if got := names(cs); got != want {
		t.Fatal(got)
	}
	if _, err := exec.LookPath("bash"); err == nil {
		t.Setenv("SHELL", "")
		if c, err := tools.ResolveShell(""); err != nil || filepath.Base(c.Shell) != "bash" {
			t.Fatal(c, err)
		}
	}
}
