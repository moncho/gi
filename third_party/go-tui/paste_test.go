package tui

import "testing"

// A bracketed paste is one PasteEvent with its text verbatim (newlines
// included), whatever surrounds it and however the reads split it.
func TestBracketedPasteParsing(t *testing.T) {
	events, rest := parseInputWithRemainder([]byte("a\x1b[200~line 1\rline 2\n\x1b[Aend\x1b[201~b"))
	if len(rest) != 0 || len(events) != 3 {
		t.Fatalf("events %#v rest %q", events, rest)
	}
	if p, ok := events[1].(PasteEvent); !ok || p.Text != "line 1\rline 2\n\x1b[Aend" {
		t.Fatalf("paste %#v", events[1])
	}
	if k, ok := events[2].(KeyEvent); !ok || k.Rune != 'b' {
		t.Fatalf("after %#v", events[2])
	}
	// Split across reads: the incomplete paste is held back whole.
	events, rest = parseInputWithRemainder([]byte("x\x1b[200~first\rhalf"))
	if len(events) != 1 || string(rest) != "\x1b[200~first\rhalf" {
		t.Fatalf("split: %#v %q", events, rest)
	}
	events, rest = parseInputWithRemainder(append(rest, []byte(" second\x1b[201~")...))
	if len(rest) != 0 || len(events) != 1 || events[0].(PasteEvent).Text != "first\rhalf second" {
		t.Fatalf("joined: %#v %q", events, rest)
	}
	// The start marker itself split across reads.
	events, rest = parseInputWithRemainder([]byte("y\x1b[20"))
	if len(events) != 1 || string(rest) != "\x1b[20" {
		t.Fatalf("marker split: %#v %q", events, rest)
	}
}

// Without a paste handler a paste replays as keystrokes.
func TestPasteFallsBackToKeys(t *testing.T) {
	a := &App{focus: newFocusManager()}
	var got []Key
	a.SetGlobalKeyHandler(func(e KeyEvent) bool { got = append(got, e.Key); return true })
	a.Dispatch(PasteEvent{Text: "a\r"})
	if len(got) != 2 || got[0] != KeyRune || got[1] != KeyEnter {
		t.Fatalf("keys %v", got)
	}
	var pasted string
	a.SetPasteHandler(func(e PasteEvent) bool { pasted = e.Text; return true })
	got = nil
	a.Dispatch(PasteEvent{Text: "a\r"})
	if pasted != "a\r" || len(got) != 0 {
		t.Fatalf("handler %q keys %v", pasted, got)
	}
}
