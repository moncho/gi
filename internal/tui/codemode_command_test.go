package tui

import (
	"strings"
	"testing"
)

// /codemode toggles the codemode tool for the session and reports its state.
func TestTUICodemodeCommand(t *testing.T) {
	c := sessionTestChat(t)
	if out := c.codemodeCommand([]string{"/codemode"}); !strings.Contains(out[0], "codemode: off for this session (from settings)") {
		t.Fatal(out)
	}
	if out := c.codemodeCommand([]string{"/codemode", "only"}); !strings.Contains(out[0], "codemode: on (only) for this session (from session)") {
		t.Fatal(out)
	}
	if out := c.codemodeCommand([]string{"/codemode", "off"}); !strings.Contains(out[0], "codemode: off for this session (from session)") {
		t.Fatal(out)
	}
	if out := c.codemodeCommand([]string{"/codemode", "default"}); !strings.Contains(out[0], "(from settings)") {
		t.Fatal(out)
	}
	if out := c.codemodeCommand([]string{"/codemode", "maybe"}); !strings.HasPrefix(out[0], "usage: /codemode") {
		t.Fatal(out)
	}
}

// /tools activate codemode switches the session's codemode toggle on;
// /tools reset returns it to the settings default.
func TestTUIToolsActivateCodemode(t *testing.T) {
	c := sessionTestChat(t)
	if out := c.toolCommand([]string{"/tools", "activate", "codemode"}); len(out) == 0 || !strings.Contains(out[0], "codemode: on (on) for this session (from session)") {
		t.Fatal(out)
	}
	out := c.toolCommand([]string{"/tools", "reset"})
	if !strings.Contains(strings.Join(out, "\n"), "codemode: off for this session (from settings)") {
		t.Fatal(out)
	}
}
