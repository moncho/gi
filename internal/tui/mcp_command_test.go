package tui

import (
	"strings"
	"testing"

	gimcp "github.com/rcarmo/gi/internal/mcp"
)

func TestTUIMCPCommand(t *testing.T) {
	c := sessionTestChat(t)
	if out := c.mcpCommand([]string{"/mcp"}); !strings.HasPrefix(out[0], "No MCP servers configured. Add them to ") {
		t.Fatal(out)
	}
	if out := c.mcpCommand([]string{"/mcp", "frob"}); out[0] != mcpUsage {
		t.Fatal(out)
	}
	if out := c.mcpCommand([]string{"/mcp", "reconnect"}); out[0] != "No enabled MCP server to reconnect." {
		t.Fatal(out)
	}
	for st, want := range map[gimcp.Status]string{
		{State: gimcp.StateConnected, Tools: 1}:         "connected · 1 tool",
		{State: gimcp.StateFailed, Error: "boom\nmore"}: "failed: boom",
		{State: gimcp.StateConnecting}:                  "connecting…",
	} {
		if got := gimcp.DescribeState(st); got != want {
			t.Fatalf("%+v: %q", st, got)
		}
	}
}
