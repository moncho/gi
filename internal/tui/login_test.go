package tui

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/inference"
	"github.com/rcarmo/go-ai/oauth"
)

// /login's and /logout's selector and login dialog render as Pi's
// OAuthSelectorComponent and LoginDialogComponent (bun
// scripts/golden-login.mjs).
func TestLoginUIMatchesPi(t *testing.T) {
	raw, err := os.ReadFile("testdata/pi-login.json")
	if err != nil {
		t.Fatal(err)
	}
	type provider struct {
		ID, Name, AuthType string
		Subscription       bool
		Status             *struct{ Type, Source string }
	}
	var golden struct {
		Selectors map[string]struct {
			Mode      string            `json:"mode"`
			Providers []provider        `json:"providers"`
			Initial   string            `json:"initial"`
			Steps     []json.RawMessage `json:"steps"`
			Widths    map[string][]struct {
				Step string   `json:"step"`
				Rows []string `json:"rows"`
			} `json:"widths"`
		} `json:"selectors"`
		Dialogs map[string]map[string][]string `json:"dialogs"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	for name, s := range golden.Selectors {
		var options []inference.LoginOption
		for _, p := range s.Providers {
			o := inference.LoginOption{ID: p.ID, Name: p.Name, AuthType: p.AuthType, Subscription: p.Subscription}
			if p.Status != nil {
				o.StatusType, o.StatusSource = p.Status.Type, p.Status.Source
			}
			options = append(options, o)
		}
		for w, states := range s.Widths {
			var width int
			fmt.Sscan(w, &width)
			c := sessionTestChat(t)
			c.openAuthSelector(s.Mode, options, s.Initial, func(inference.LoginOption) {}, nil)
			for i, state := range states {
				if i > 0 {
					var key string
					var pair []string
					if json.Unmarshal(s.Steps[i-1], &key) == nil {
						pressMenuKey(t, c.KeyMap(), scopedModelsKeyEvents[key])
					} else if json.Unmarshal(s.Steps[i-1], &pair) == nil {
						for _, r := range pair[1] {
							pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyRune, Rune: r})
						}
					}
				}
				if got := spanRowsText(c.piAuthSelectorRows(width)); strings.Join(got, "\n") != strings.Join(state.Rows, "\n") {
					t.Fatalf("%s@%d after %s:\n%s\nPi:\n%s", name, width, state.Step, strings.Join(got, "\n"), strings.Join(state.Rows, "\n"))
				}
			}
		}
	}
	for w, dialogs := range golden.Dialogs {
		var width int
		fmt.Sscan(w, &width)
		build := map[string]func() *loginDialogState{
			"auth_prompt": func() *loginDialogState {
				d := &loginDialogState{title: "Login to Anthropic"}
				d.showAuth("https://claude.ai/oauth/authorize?code=true", "Paste the code shown after signing in.")
				d.showPrompt("Paste the authorization code:", "abc#123")
				return d
			},
			"auth_prompt_typed": func() *loginDialogState {
				d := &loginDialogState{title: "Login to Anthropic"}
				d.showAuth("https://claude.ai/oauth/authorize?code=true", "Paste the code shown after signing in.")
				d.showPrompt("Paste the authorization code:", "abc#123")
				d.input = "xyz"
				return d
			},
			"auth_submitted": func() *loginDialogState {
				d := &loginDialogState{title: "Login to Anthropic"}
				d.showAuth("https://claude.ai/oauth/authorize?code=true", "Paste the code shown after signing in.")
				d.showPrompt("Paste the authorization code:", "abc#123")
				d.input, d.answer = "xyz", nil
				d.showProgress("Exchanging code for tokens…")
				return d
			},
			"device": func() *loginDialogState {
				d := &loginDialogState{title: "Login to GitHub Copilot"}
				d.showDeviceCode("https://github.com/login/device", "ABCD-1234")
				d.showWaiting("Waiting for authentication...")
				return d
			},
			"api_key": func() *loginDialogState {
				d := &loginDialogState{title: "Login to Groq"}
				d.showPrompt("Enter API key:", "")
				return d
			},
		}
		for name, rows := range dialogs {
			make, ok := build[name]
			if !ok {
				continue // Pi's manual-input variant: gi's flows prompt instead
			}
			c := &chatTUI{loginDialog: make()}
			if got := spanRowsText(c.piLoginDialogRows(width)); strings.Join(got, "\n") != strings.Join(rows, "\n") {
				t.Errorf("%s@%d:\n%s\nPi:\n%s", name, width, strings.Join(got, "\n"), strings.Join(rows, "\n"))
			}
		}
	}
}

// An API key login saves Pi's {"type":"api_key","key":...}; /logout then
// lists it and removes it; cancelling returns to the menu it came from.
func TestLoginAPIKeyAndLogoutFlow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("GI_CODING_AGENT_DIR", "")
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(root, ".pi", "agent"))
	c := sessionTestChat(t)
	c.uiQueue = make(chan func(), 64)
	drain := func(cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !cond() {
			select {
			case fn := <-c.uiQueue:
				fn()
			case <-time.After(10 * time.Millisecond):
			}
			if time.Now().After(deadline) {
				t.Fatal("timed out")
			}
		}
	}
	keys := func(evs ...gotui.KeyEvent) {
		for _, ev := range evs {
			pressMenuKey(t, c.KeyMap(), ev)
		}
	}
	typeText := func(s string) {
		for _, r := range s {
			keys(gotui.KeyEvent{Key: gotui.KeyRune, Rune: r})
		}
	}

	c.handleCommand("/login groq")
	drain(func() bool { return c.loginDialog != nil && c.loginDialog.answer != nil })
	if got := strings.Join(spanRowsText(c.piLoginDialogRows(60)), "\n"); !strings.Contains(got, "Login to Groq") || !strings.Contains(got, "Enter API key:") {
		t.Fatalf("api key dialog:\n%s", got)
	}
	typeText("gsk-test")
	keys(gotui.KeyEvent{Key: gotui.KeyEnter})
	drain(func() bool { return c.loginDialog == nil })
	raw, _ := os.ReadFile(inference.AuthFilePath())
	if !strings.Contains(string(raw), `"groq"`) || !strings.Contains(string(raw), `"key": "gsk-test"`) && !strings.Contains(string(raw), `"key":"gsk-test"`) {
		t.Fatalf("auth.json: %s", raw)
	}
	if last := c.transcript[len(c.transcript)-1]; !strings.HasPrefix(last, "Saved API key for Groq. Credentials saved to ") {
		t.Fatalf("status: %q", last)
	}

	// Cancelling a login from the provider selector reopens it.
	c.handleCommand("/login")
	if c.modelMenuKind != "select" {
		t.Fatalf("auth method menu: %q", c.modelMenuKind)
	}
	keys(gotui.KeyEvent{Key: gotui.KeyDown}, gotui.KeyEvent{Key: gotui.KeyEnter})
	if c.modelMenuKind != "auth-selector" || c.authSelector.mode != "login" {
		t.Fatalf("provider selector: %q", c.modelMenuKind)
	}
	typeText("mistral")
	keys(gotui.KeyEvent{Key: gotui.KeyEnter})
	drain(func() bool { return c.loginDialog != nil && c.loginDialog.answer != nil })
	keys(gotui.KeyEvent{Key: gotui.KeyEscape})
	if c.loginDialog != nil || c.modelMenuKind != "auth-selector" || c.authSelector.query != "" {
		t.Fatalf("cancel did not return to the provider selector: %q", c.modelMenuKind)
	}
	keys(gotui.KeyEvent{Key: gotui.KeyEscape})
	if c.modelMenuKind != "select" {
		t.Fatalf("escape from the provider selector returns to the method menu: %q", c.modelMenuKind)
	}
	keys(gotui.KeyEvent{Key: gotui.KeyEscape})

	c.handleCommand("/logout")
	if c.modelMenuKind != "auth-selector" || c.authSelector.mode != "logout" || len(c.authSelector.all) != 1 {
		t.Fatalf("logout selector: %q %+v", c.modelMenuKind, c.authSelector)
	}
	keys(gotui.KeyEvent{Key: gotui.KeyEnter})
	if last := c.transcript[len(c.transcript)-1]; last != "Removed stored API key for Groq. Environment variables and models.json config are unchanged." {
		t.Fatalf("logout status: %q", last)
	}
	c.handleCommand("/logout")
	if last := c.transcript[len(c.transcript)-1]; !strings.HasPrefix(last, "No stored credentials to remove.") {
		t.Fatalf("empty logout: %q", last)
	}
}

type fakeOAuthProvider struct {
	oauth.ProviderInterface
	id string
}

func (p fakeOAuthProvider) ID() string { return p.id }
func (p fakeOAuthProvider) Login(cb oauth.LoginCallbacks) (*oauth.Credentials, error) {
	cb.OnAuth(oauth.AuthInfo{URL: "https://example.com/auth", Instructions: "Sign in, then paste the code."})
	code, err := cb.OnPrompt(oauth.Prompt{Message: "Paste the code:"})
	if err != nil {
		return nil, err
	}
	cb.OnProgress("Exchanging code…")
	return &oauth.Credentials{Access: "access-" + code, Refresh: "refresh", Expires: 1}, nil
}

// An OAuth login shows the sign-in URL, takes the pasted code and saves
// Pi's oauth entry.
func TestLoginOAuthFlow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("GI_CODING_AGENT_DIR", "")
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(root, ".pi", "agent"))
	inference.Init()
	real := oauth.GetProvider("openrouter")
	oauth.RegisterProvider(fakeOAuthProvider{ProviderInterface: real, id: "openrouter"})
	t.Cleanup(func() { oauth.RegisterProvider(real) })
	opened := ""
	defer func(f func(string)) { openBrowser = f }(openBrowser)
	openBrowser = func(u string) { opened = u }

	c := sessionTestChat(t)
	c.uiQueue = make(chan func(), 64)
	drain := func(cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !cond() {
			select {
			case fn := <-c.uiQueue:
				fn()
			case <-time.After(10 * time.Millisecond):
			}
			if time.Now().After(deadline) {
				t.Fatal("timed out")
			}
		}
	}
	c.handleCommand("/login openrouter")
	// Pi: a provider with both methods asks which one.
	if c.modelMenuKind != "select" || c.selectDialog.title != "Select authentication method for OpenRouter:" {
		t.Fatalf("method menu: %q %q", c.modelMenuKind, c.selectDialog.title)
	}
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEnter})
	drain(func() bool { return c.loginDialog != nil && c.loginDialog.answer != nil })
	got := strings.Join(spanRowsText(c.piLoginDialogRows(60)), "\n")
	if !strings.Contains(got, "Login to OpenRouter") || !strings.Contains(got, "https://example.com/auth") || !strings.Contains(got, "Paste the code:") || opened != "https://example.com/auth" {
		t.Fatalf("oauth dialog (opened %q):\n%s", opened, got)
	}
	for _, r := range "c0de" {
		pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyRune, Rune: r})
	}
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEnter})
	drain(func() bool { return c.loginDialog == nil })
	raw, _ := os.ReadFile(inference.AuthFilePath())
	var entries map[string]map[string]any
	_ = json.Unmarshal(raw, &entries)
	if e := entries["openrouter"]; e["type"] != "oauth" || e["access"] != "access-c0de" || e["refresh"] != "refresh" {
		t.Fatalf("auth.json: %s", raw)
	}
	if last := c.transcript[len(c.transcript)-1]; !strings.HasPrefix(last, "Logged in to OpenRouter. Credentials saved to ") {
		t.Fatalf("status: %q", last)
	}
}

// /login completes providers as Pi does: one item per provider, its
// methods in the description, fuzzy over id, name and methods.
func TestLoginArgumentCompletions(t *testing.T) {
	options := []inference.LoginOption{
		{ID: "groq", Name: "Groq", AuthType: "api_key"},
		{ID: "anthropic", Name: "Anthropic", AuthType: "api_key", Subscription: true},
		{ID: "anthropic", Name: "Anthropic", AuthType: "oauth", Subscription: true},
		{ID: "local", Name: "local", AuthType: "api_key"},
	}
	got := func(prefix string) string {
		var out []string
		for _, it := range loginArgumentCompletions(options, prefix) {
			out = append(out, it.value+"="+it.description)
		}
		return strings.Join(out, ";")
	}
	if all := got(""); all != "anthropic=Anthropic · subscription/API key;groq=Groq · API key;local=API key" {
		t.Fatalf("completions: %s", all)
	}
	if sub := got("subscr"); sub != "anthropic=Anthropic · subscription/API key" {
		t.Fatalf("fuzzy: %s", sub)
	}
	if got("zzz") != "" {
		t.Fatal("no match should be empty")
	}
}

// Ctrl+X copies the sign-in URL (Pi's AuthUrlComponent): over OSC 52 when
// there is no native clipboard and no display, and the copy hint becomes
// the result.
func TestLoginDialogCopiesAuthURL(t *testing.T) {
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("TERMUX_VERSION", "")
	var osc bytes.Buffer
	c := &chatTUI{osc52Writer: &osc, clipboardLookPath: func(string) (string, error) { return "", os.ErrNotExist }}
	url := "https://claude.ai/oauth/authorize?code=true"
	c.loginDialog = &loginDialogState{title: "Login to Anthropic"}
	c.loginDialog.showAuth(url, "")
	pressMenuKey(t, c.loginDialogKeys(), gotui.KeyEvent{Key: gotui.KeyRune, Rune: 'x', Mod: gotui.ModCtrl})
	if want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(url)) + "\x07"; osc.String() != want {
		t.Fatalf("OSC 52 %q, want %q", osc.String(), want)
	}
	rows := strings.Join(spanRowsText(c.piLoginDialogRows(80)), "\n")
	if !strings.Contains(rows, "Ctrl+click to open • Copied URL to clipboard") && !strings.Contains(rows, "Cmd+click to open • Copied URL to clipboard") {
		t.Fatalf("rows:\n%s", rows)
	}
}
