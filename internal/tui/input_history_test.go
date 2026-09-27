package tui

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/store"
)

func historyKey(c *chatTUI, key gotui.Key) {
	for _, binding := range c.KeyMap() {
		if binding.Pattern.Key == key && binding.Pattern.Mod == 0 {
			binding.Handler(gotui.KeyEvent{Key: key})
			return
		}
	}
	panic("missing history key")
}

func TestCursorHistoryRoundTripAndSessionPersistence(t *testing.T) {
	c := sessionTestChat(t)
	for _, input := range []string{"/where", "/help", "!!echo hello"} {
		c.recordInputHistory(input)
	}
	c.input.SetText("draft 中文🙂")
	c.input.cursorPos = 5
	historyKey(c, gotui.KeyUp)
	if c.input.Text() != "!!echo hello" {
		t.Fatal("Up did not recall newest entry")
	}
	historyKey(c, gotui.KeyUp)
	historyKey(c, gotui.KeyUp)
	historyKey(c, gotui.KeyUp) // clamp to oldest
	if c.input.Text() != "/where" {
		t.Fatalf("oldest history: %q", c.input.Text())
	}
	historyKey(c, gotui.KeyDown)
	if c.input.Text() != "/help" {
		t.Fatal("Down did not advance")
	}
	historyKey(c, gotui.KeyDown)
	historyKey(c, gotui.KeyDown)
	if c.input.Text() != "draft 中文🙂" || c.input.cursorPos != 5 {
		t.Fatalf("draft lost: %q cursor %d", c.input.Text(), c.input.cursorPos)
	}
	historyKey(c, gotui.KeyDown)
	if c.input.Text() != "draft 中文🙂" {
		t.Fatal("Down without history changed draft")
	}
	historyKey(c, gotui.KeyUp)
	c.input.insertRune(gotui.KeyEvent{Rune: '!'})
	if c.histIdx != -1 {
		t.Fatal("editing recalled entry did not leave history navigation")
	}
	historyKey(c, gotui.KeyDown)
	if c.input.Text() != "!!echo hello!" {
		t.Fatalf("edited entry replaced: %q", c.input.Text())
	}
	c.input.SetText("A scratch")
	historyKey(c, gotui.KeyUp)
	if !c.switchSession("B") {
		t.Fatal("switch B")
	}
	if len(c.history) != 0 || c.input.Text() != "" {
		t.Fatal("A history leaked to B")
	}
	c.recordInputHistory("B command")
	historyKey(c, gotui.KeyUp)
	if c.input.Text() != "B command" {
		t.Fatal("B history missing")
	}
	if !c.switchSession("A") {
		t.Fatal("switch A")
	}
	if c.input.Text() != "!!echo hello" || c.historyDraft != "A scratch" {
		t.Fatalf("A navigation lost across switch: %q / %q", c.input.Text(), c.historyDraft)
	}
	historyKey(c, gotui.KeyDown)
	if c.input.Text() != "A scratch" {
		t.Fatal("A draft not restored after switch")
	}
	if !reflect.DeepEqual(c.loadCommandHistory(), []string{"/where", "/help", "!!echo hello"}) {
		t.Fatal("A history not persisted")
	}
	if !c.switchSession("B") {
		t.Fatal("switch B again")
	}
	if !reflect.DeepEqual(c.loadCommandHistory(), []string{"B command"}) {
		t.Fatal("B history not persisted")
	}
}

func TestHistoryReloadAndSearchCycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gi.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := s.CreateSession(ctx, "A", "A", nil); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"first alpha", "other", "second alpha"} {
		if err := s.RecordTUIInput(ctx, "A", text, 10000); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := &chatTUI{store: s, sessionID: "A"}
	c.ensureInput()
	c.historySearchIdx = -1
	c.history = c.loadCommandHistory()
	c.input.SetText("alpha")
	c.searchHistoryBackward()
	if c.input.Text() != "second alpha" {
		t.Fatal("search newest")
	}
	c.searchHistoryBackward()
	if c.input.Text() != "first alpha" {
		t.Fatal("Ctrl-R did not repeat search")
	}
	c.recallHistory(1)
	if c.input.Text() != "other" {
		t.Fatal("Down after search")
	}
	c.recallHistory(1)
	c.recallHistory(1)
	if c.input.Text() != "alpha" {
		t.Fatal("search draft not restored")
	}
}
