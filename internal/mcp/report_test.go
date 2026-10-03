package mcp

import (
	"errors"
	"testing"
)

// Pi's reportProblems text (extensions/mcp/index.js).
func TestProblemReport(t *testing.T) {
	statuses := []Status{
		{Name: "ok", State: StateConnected, Tools: 2},
		{Name: "auth", State: StateNeedsAuth},
		{Name: "down", State: StateFailed, Error: "dial refused\nmore detail"},
		{Name: "off", State: StateDisabled},
		{Name: "slow", State: StateConnecting},
	}
	want := "MCP servers need attention:\n  config: bad entry\n  auth: needs sign-in\n  down: failed: dial refused\nRun /mcp to fix."
	if got := ProblemReport(statuses, []error{errors.New("bad entry")}); got != want {
		t.Fatalf("%q", got)
	}
	if got := ProblemReport(statuses[:1], nil); got != "" {
		t.Fatalf("%q", got)
	}
}
