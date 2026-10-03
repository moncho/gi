package tui

import (
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

func TestPiFuzzyFilterOrdersLikePi(t *testing.T) {
	items := []slashItem{{name: "scoped-models"}, {name: "model"}, {name: "compact"}}
	got := piFuzzyFilter(items, "mo", func(it slashItem) string { return it.name })
	if len(got) != 2 || got[0].name != "model" || got[1].name != "scoped-models" {
		t.Fatalf("fuzzy order %+v", got)
	}
	if ok, _ := piFuzzyMatch("xyz", "model"); ok {
		t.Fatal("non-match matched")
	}
}

func TestSlashMenuOpensFiltersCompletesAndCloses(t *testing.T) {
	c := &chatTUI{}
	c.ensureInput()
	type_ := func(s string) {
		for _, r := range s {
			c.input.insertRune(gotui.KeyEvent{Key: gotui.KeyRune, Rune: r})
		}
	}
	type_("/")
	if !c.slash.active || len(c.slash.items) != len(c.slashCommandItems()) || c.slashMenuHeight() != slashMaxVisible+1 {
		t.Fatalf("slash did not open with all commands: %+v", c.slash)
	}
	type_("mo")
	if c.slash.items[0].name != "model" || !strings.HasPrefix(c.slash.items[0].description, "<provider/model> — ") {
		t.Fatalf("filter: %+v", c.slash.items)
	}
	rows := c.slashMenuRows(80)
	if got := rows[0][0].Text; !strings.HasPrefix(got, "→ model") || rows[0][0].Style.Fg != piAccent {
		t.Fatalf("selected row %q", got)
	}
	c.moveSlashSelection(1)
	if !c.handleSlashKey(gotui.KeyTab) || c.input.Text() != "/scoped-models " || c.slash.active || c.input.cursorPos != len("/scoped-models ") {
		t.Fatalf("tab completion %q %v", c.input.Text(), c.slash.active)
	}
	type_("x")
	if c.slash.active {
		t.Fatal("arguments reopened the list")
	}
	c.input.SetText("")
	type_("/zzzz")
	if c.slash.active {
		t.Fatal("no matches must close")
	}
	c.input.SetText("")
	type_("/co")
	if !c.handleSlashKey(gotui.KeyEscape) || c.slash.active {
		t.Fatal("escape")
	}
	type_("m")
	if !c.slash.active || c.slash.items[0].name != "compact" {
		t.Fatalf("typing in a slash command reopens the list, as in Pi: %+v", c.slash)
	}
	c.input.SetText("say /model")
	if c.slash.active {
		t.Fatal("slash mid-message opened list")
	}
}

func TestSlashEnterCompletesAndSubmits(t *testing.T) {
	c := &chatTUI{}
	c.ensureInput()
	var submitted string
	c.input.onSubmit = func(s string) { submitted = s }
	for _, r := range "/hotk" {
		c.input.insertRune(gotui.KeyEvent{Key: gotui.KeyRune, Rune: r})
	}
	if !c.handleSlashKey(gotui.KeyEnter) || strings.TrimSpace(submitted) != "/hotkeys" {
		t.Fatalf("enter submitted %q", submitted)
	}
}

// The catalogue lists Pi's built-in commands first, in Pi's order.
func TestSlashCatalogueFollowsPiBuiltinOrder(t *testing.T) {
	pi := []string{"settings", "model", "tree", "thinking", "scoped-models", "export", "import", "share", "copy", "name", "session", "hotkeys", "fork", "clone", "login", "logout", "new", "compact", "resume", "reload", "quit"}
	items := (&chatTUI{}).slashCommandItems()
	if len(items) < len(pi) {
		t.Fatalf("catalogue too short: %d", len(items))
	}
	for i, name := range pi {
		if items[i].name != name {
			t.Fatalf("item %d = %q, want Pi's %q", i, items[i].name, name)
		}
	}
}

// Pi 1.0 (#10218): the slash menu also works after leading whitespace, and
// completing keeps the whitespace.
func TestSlashMenuAfterLeadingWhitespace(t *testing.T) {
	c := &chatTUI{}
	c.ensureInput()
	for _, r := range "  /mo" {
		c.input.insertRune(gotui.KeyEvent{Key: gotui.KeyRune, Rune: r})
	}
	if !c.slash.active || c.slash.items[0].name != "model" {
		t.Fatalf("slash menu after whitespace: %+v", c.slash)
	}
	c.applySlashSelection(false)
	if c.input.Text() != "  /model " || c.input.cursorPos != 9 {
		t.Fatalf("completion %q cursor %d", c.input.Text(), c.input.cursorPos)
	}
	c.input.SetText("")
	c.slash = slashMenu{}
	for _, r := range "a /" {
		c.input.insertRune(gotui.KeyEvent{Key: gotui.KeyRune, Rune: r})
	}
	if c.slash.active {
		t.Fatal("slash menu opened after text")
	}
}
