package mcp

import (
	"fmt"
	"testing"
)

// Pi's formatStatus (extensions/mcp/index.js).
func TestFormatStatusMatchesPi(t *testing.T) {
	statuses := []Status{
		{Name: "a", State: StateConnected, Tools: 2, Exposure: ExposureDirect},
		{Name: "b", State: StateFailed, Error: "boom\nmore"},
		{Name: "c", State: StateDisconnected, Exposure: ExposureDeferred},
		{Name: "d", State: StateDisabled},
		{Name: "e", State: StateConnecting},
	}
	want := "a: connected, 2 tools (direct)\nb: failed (codemode)\n    boom\n    more\nc: disconnected, reconnects on next call (deferred)\nd: disabled (codemode)\ne: connecting (codemode)\nconfig error: bad"
	if got := FormatStatus(statuses, []error{fmt.Errorf("bad")}); got != want {
		t.Fatalf("%q", got)
	}
	if got := FormatStatus(nil, nil); got != "No MCP servers configured. Add them to "+UserConfigPath()+" or .pi/mcp.json." {
		t.Fatal(got)
	}
}
