package tui

import (
	"context"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

func pressComposerEscape(t *testing.T, c *chatTUI) {
	t.Helper()
	for _, binding := range c.input.KeyMap() {
		if binding.Pattern.Key == gotui.KeyEscape && binding.Pattern.Mod == 0 {
			binding.Handler(gotui.KeyEvent{})
			return
		}
	}
	t.Fatal("Escape not bound")
}

func TestActiveEscapeRestoresTextAndCancelsBeforeHandoff(t *testing.T) {
	c := sessionTestChat(t)
	c.durableDrafts = true
	c.loadDurableDraft()
	ctx := context.Background()
	if _, err := c.store.CreateTurnWithStatus(ctx, "active", c.sessionID, "running", "held", nil); err != nil {
		t.Fatal(err)
	}
	ok, err := c.store.ClaimSessionActiveTurn(ctx, c.sessionID, "active", "other frontend", "foreign")
	if err != nil || !ok {
		t.Fatalf("foreign claim=%t: %v", ok, err)
	}
	if _, err := c.store.CreateTurnWithStatus(ctx, "follow", c.sessionID, "queued", "follow-up", nil); err != nil {
		t.Fatal(err)
	}
	c.input.SetText("newer 中文🙂")
	c.input.cursorPos = 7
	c.input.Focus()
	pressComposerEscape(t, c)
	active, err := c.store.GetTurn(ctx, "active")
	if err != nil || active.Status != "running" {
		t.Fatalf("foreign run cancelled=%#v: %v", active, err)
	}
	queued, err := c.store.GetTurn(ctx, "follow")
	if err != nil || queued.Status != "queued" {
		t.Fatalf("foreign run lost queue=%#v: %v", queued, err)
	}
	if c.input.Text() != "newer 中文🙂" || !c.input.IsFocused() {
		t.Fatalf("foreign conflict changed editor=%q focused=%t", c.input.Text(), c.input.IsFocused())
	}
	if err := c.store.ReleaseSessionActiveTurn(ctx, c.sessionID, "foreign"); err != nil {
		t.Fatal(err)
	}
	// An observed claim is not sufficient: the engine must still own the run.
	pressComposerEscape(t, c)
	active, err = c.store.GetTurn(ctx, "active")
	if err != nil || active.Status != "running" {
		t.Fatalf("unowned run changed=%#v: %v", active, err)
	}
	queued, err = c.store.GetTurn(ctx, "follow")
	if err != nil || queued.Status != "queued" {
		t.Fatalf("unowned queue changed=%#v: %v", queued, err)
	}
	journal, err := c.store.LoadTUITextDraft(ctx, c.sessionID)
	if err != nil || journal.Text != "newer 中文🙂" {
		t.Fatalf("unsent text lost=%#v: %v", journal, err)
	}
}

func TestActiveEscapeWithOwnedTurnNoQueue(t *testing.T) {
	c := sessionTestChat(t)
	c.durableDrafts = true
	c.loadDurableDraft()
	// A stale finished claim must fail closed rather than clearing an editor.
	ctx := context.Background()
	if _, err := c.store.CreateTurnWithStatus(ctx, "finished", c.sessionID, "completed", "old", nil); err != nil {
		t.Fatal(err)
	}
	ok, err := c.store.ClaimSessionActiveTurn(ctx, c.sessionID, "finished", "stale", "finished")
	if err != nil || !ok {
		t.Fatalf("claim=%t %v", ok, err)
	}
	c.input.SetText("unsent")
	pressComposerEscape(t, c)
	journal, err := c.store.LoadTUITextDraft(ctx, c.sessionID)
	if err != nil || journal.Text != "unsent" {
		t.Fatalf("stale claim draft=%#v: %v", journal, err)
	}
	if c.input.Text() != "unsent" {
		t.Fatalf("stale claim editor=%q", c.input.Text())
	}
}

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
