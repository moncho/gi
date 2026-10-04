package tui

import (
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/keychain"
)

// !! local commands get the keychain as the agent's shell does: named
// variables and placeholders, nothing else.
func TestLocalShellUsesKeychain(t *testing.T) {
	t.Setenv("GI_KEYCHAIN_KEY", "tui-master")
	c := sessionTestChat(t)
	c.cfg.WorkspaceRoot = t.TempDir()
	kc := keychain.New(c.store.DB())
	for _, e := range []keychain.Entry{{Name: "deploy/token", Secret: "tok-1"}, {Name: "other/token", Secret: "tok-2"}} {
		if err := kc.Set(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}
	out := strings.Join(c.localShellShortcutLines(`printf '[%s|%s|%s]' "$DEPLOY_TOKEN" "$(env | grep -c tok-2)" keychain:deploy/token`), "\n")
	if !strings.Contains(out, "[tok-1|0|tok-1]") {
		t.Fatalf("output:\n%s", out)
	}
}
