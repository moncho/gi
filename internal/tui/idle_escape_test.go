package tui

import (
	"context"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

func TestIdleEscapeKeepsComposerFocusAndDraft(t *testing.T) {
	c := sessionTestChat(t)
	c.durableDrafts = true
	c.loadDurableDraft()
	c.input.SetText("probe-中文🙂")
	c.input.Focus()
	found := false
	for _, binding := range c.input.KeyMap() {
		if binding.Pattern.Key != gotui.KeyEscape || binding.Pattern.Mod != 0 {
			continue
		}
		found = true
		binding.Handler(gotui.KeyEvent{})
	}
	if !found || !c.input.IsFocused() || c.input.Text() != "probe-中文🙂" {
		t.Fatalf("idle Escape changed focus or draft: found=%t focused=%t text=%q", found, c.input.IsFocused(), c.input.Text())
	}
	if got, err := c.store.LoadTUITextDraft(context.Background(), c.sessionID); err != nil || got.Text != "probe-中文🙂" {
		t.Fatalf("idle Escape changed durable draft: %#v %v", got, err)
	}
}
