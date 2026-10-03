package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"
	gimcp "github.com/rcarmo/gi/internal/mcp"
	"github.com/rcarmo/gi/internal/mcp/mcptest"
)

func spanRowsText(rows spanRows) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		var b strings.Builder
		for _, span := range row {
			b.WriteString(span.Text)
		}
		out[i] = strings.TrimRight(b.String(), " ")
	}
	return out
}

// Pi's ExtensionSelectorComponent rendered by Pi: bun scripts/golden-select-dialog.mjs
func TestSelectDialogMatchesPi(t *testing.T) {
	raw, err := os.ReadFile("testdata/pi-select-dialog.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Width  int
		States []struct {
			Step   string
			Rows   []string
			Result map[string]any
		}
		Escape map[string]any
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	c := &chatTUI{}
	c.ensureInput()
	var result map[string]any
	c.openSelect("MCP server", []string{"github", "local", "remote-with-a-rather-long-name-here"},
		func(v string) { result = map[string]any{"select": v} }, func() { result = map[string]any{"cancel": true} })
	for _, want := range golden.States {
		if want.Step != "start" {
			ev := map[string]gotui.KeyEvent{
				"up": {Key: gotui.KeyUp}, "down": {Key: gotui.KeyDown}, "enter": {Key: gotui.KeyEnter},
				"j": {Key: gotui.KeyRune, Rune: 'j'}, "k": {Key: gotui.KeyRune, Rune: 'k'},
			}[want.Step]
			if !dispatchKey(c.KeyMap(), ev) {
				t.Fatalf("%s not handled", want.Step)
			}
		}
		if want.Result == nil {
			if got := spanRowsText(c.piSelectDialogRows(golden.Width)); strings.Join(got, "\n") != strings.Join(want.Rows, "\n") {
				t.Fatalf("after %s:\n%s\nPi:\n%s", want.Step, strings.Join(got, "\n"), strings.Join(want.Rows, "\n"))
			}
		} else if fmt.Sprint(result) != fmt.Sprint(want.Result) || c.modelMenuOpen {
			t.Fatalf("after %s: result %v open %v; Pi %v", want.Step, result, c.modelMenuOpen, want.Result)
		}
	}
	result = nil
	c.openSelect("MCP server", []string{"github", "local"}, func(string) { result = map[string]any{"select": true} }, func() { result = map[string]any{"cancel": true} })
	if !dispatchKey(c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEscape}) || fmt.Sprint(result) != fmt.Sprint(golden.Escape) {
		t.Fatalf("escape: %v; Pi %v", result, golden.Escape)
	}
	// Closed by something else (Pi resolves undefined): the cancel callback runs.
	result = nil
	c.openSelect("MCP server", []string{"github"}, nil, func() { result = map[string]any{"cancel": true} })
	c.closeModelMenu()
	if fmt.Sprint(result) != fmt.Sprint(golden.Escape) {
		t.Fatalf("external close: %v", result)
	}
}

// dispatchKey runs the first unmodified binding in keys for ev's key or
// rune (rune patterns set only Rune).
func dispatchKey(keys gotui.KeyMap, ev gotui.KeyEvent) bool {
	for _, binding := range keys {
		p := binding.Pattern
		runeMatch := ev.Key == gotui.KeyRune && p.Rune != 0 && p.Rune == ev.Rune
		if p.Mod == 0 && !p.AnyRune && (runeMatch || p.Rune == 0 && p.Key == ev.Key) {
			binding.Handler(ev)
			return true
		}
	}
	return false
}

// /mcp reconnect without a name asks with Pi's picker when several servers
// qualify and none is preferred; choosing one reconnects it.
func TestTUIMCPReconnectPicker(t *testing.T) {
	a, b := mcptest.New("a"), mcptest.New("b")
	t.Cleanup(a.Close)
	t.Cleanup(b.Close)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(cfgPath, []byte(fmt.Sprintf(`{"mcpServers": {"alpha": {"url": %q}, "beta": {"url": %q}}}`, a.URL, b.URL)), 0o600); err != nil {
		t.Fatal(err)
	}
	c := sessionTestChat(t)
	c.engine.EnableMCPConfig(gimcp.LoadConfig(cfgPath, "", false), nil, "")
	deadline := time.Now().Add(10 * time.Second)
	for {
		statuses, _ := c.engine.MCPStatus()
		if len(statuses) == 2 && statuses[0].State == gimcp.StateConnected && statuses[1].State == gimcp.StateConnected {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("servers not connected: %+v", statuses)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if out := c.mcpCommand([]string{"/mcp", "reconnect"}); out != nil || !c.modelMenuOpen || c.modelMenuKind != "select" ||
		strings.Join(c.modelMenuChoices, ",") != "alpha,beta" || c.selectDialog.title != "MCP server" {
		t.Fatalf("picker not shown: %q %v %q", out, c.modelMenuOpen, c.modelMenuChoices)
	}
	dispatchKey(c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyDown})
	dispatchKey(c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEnter})
	if c.modelMenuOpen || !strings.Contains(strings.Join(c.transcript, "\n"), `Reconnected to MCP server "beta" (connected`) {
		t.Fatalf("choice not run:\n%s", strings.Join(c.transcript, "\n"))
	}
	if out := c.mcpCommand([]string{"/mcp", "reconnect", "gamma"}); strings.Join(out, "") != `No MCP server named "gamma".` {
		t.Fatalf("%q", out)
	}
}
