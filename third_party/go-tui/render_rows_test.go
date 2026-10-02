package tui

import "testing"

func TestRowRedrawOptionFrameSync(t *testing.T) {
	term := newSyncRecordingTerminal(20, 4)
	app := &App{terminal: term, buffer: NewBuffer(20, 4), focus: newFocusManager(), mounts: newMountState(), root: New(WithText("flag 🇵🇹"))}
	if err := WithRowRedraw()(app); err != nil {
		t.Fatal(err)
	}
	app.needsFullRedraw = true
	app.renderFrame()
	assertWrapped(t, term.ops)
	for _, op := range term.ops {
		if op == "clear" {
			t.Fatal("row redraw cleared whole screen/scrollback")
		}
	}
	if app.needsFullRedraw {
		t.Fatal("did not consume forced redraw")
	}
	term.ops = nil
	app.root.SetText("flag 🇯🇵")
	app.renderFrame()
	assertWrapped(t, term.ops)
}

func TestRowRedrawIgnoredInlineAndRetainedOnModeSwitch(t *testing.T) {
	app, emu := newInlineTestApp(20, 8, 3)
	app.mounts = newMountState()
	app.root = New(WithText("draft"))
	if err := WithRowRedraw()(app); err != nil {
		t.Fatal(err)
	}
	app.renderFrame()
	if app.buffer.Height() != 3 || emu.inAltScreen {
		t.Fatal("row option changed regular mode")
	}
	if err := app.EnterAlternateScreen(); err != nil {
		t.Fatal(err)
	}
	app.renderFrame()
	if !app.rowRedraw || app.needsFullRedraw || app.buffer.Height() != 8 {
		t.Fatal("switch lost row redraw")
	}
	if err := app.ExitAlternateScreen(); err != nil {
		t.Fatal(err)
	}
	app.renderFrame()
	if app.buffer.Height() != 3 || app.inlineHeight != 3 || !app.rowRedraw {
		t.Fatal("regular restoration lost mode state")
	}
}
