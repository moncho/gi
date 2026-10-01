package turn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gimcp "github.com/rcarmo/gi/internal/mcp"
	"github.com/rcarmo/gi/internal/mcp/mcptest"
)

func mcpTestEngine(t *testing.T) (*Engine, *mcptest.Server) {
	t.Helper()
	fake := mcptest.New("Fake instructions for tests.\nSecond line.")
	t.Cleanup(fake.Close)
	path := filepath.Join(t.TempDir(), "mcp.json")
	body := fmt.Sprintf(`{"mcpServers": {
	  "fake": {"url": %q, "toolExposure": {"echo": "direct", "upper": "direct", "delete_*": "hidden"}},
	  "fake-direct": {"url": %q, "exposure": "direct", "description": "Everything direct"}}}`, fake.URL, fake.URL)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := gimcp.LoadConfig(path, "", false)
	if len(cfg.Errors) > 0 {
		t.Fatal(cfg.Errors)
	}
	s := openTestStore(t)
	e := New(s)
	t.Cleanup(func() { e.Close(); s.Close() })
	e.enableMCPWith(gimcp.NewManager(cfg, t.TempDir(), ""))
	e.awaitDirectMCPTools(context.Background())
	return e, fake
}

// Direct tools are registered under Pi's names and run through the tool
// registry; codemode tools are not declared; hidden tools are unreachable.
func TestMCPDirectToolsRegisterAndRun(t *testing.T) {
	e, _ := mcpTestEngine(t)
	ctx := context.Background()
	for _, name := range []string{"mcp__fake__echo", "mcp__fake__upper", "mcp__fake_direct__search_code", "mcp__fake_direct__delete_all"} {
		if _, ok := e.tools.GetRegistered(name); !ok {
			t.Fatalf("direct tool %s not registered", name)
		}
	}
	for _, name := range []string{"mcp__fake__search_code", "mcp__fake__delete_all"} {
		if _, ok := e.tools.GetRegistered(name); ok {
			t.Fatalf("non-direct tool %s registered", name)
		}
	}
	if tool, _ := e.tools.GetRegistered("mcp__fake__upper"); tool.Kind != "read" || tool.Source != "mcp:fake" || tool.Description != "Uppercase text" {
		t.Fatalf("tool metadata %+v", tool)
	}
	out, err := e.ExecuteToolByName(ctx, "mcp__fake__echo", "", map[string]any{"text": "hi"})
	if err != nil || out != "echo:hi" {
		t.Fatalf("echo: %q %v", out, err)
	}
	if _, err := e.ExecuteToolByName(ctx, "mcp__fake_direct__delete_all", "", map[string]any{}); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("isError must surface as a tool error: %v", err)
	}
}

// Admission waits for direct servers, so MCP tools are in the turn's tool set.
func TestMCPDirectToolsAdmittedInTurn(t *testing.T) {
	e, _ := mcpTestEngine(t)
	ctx := context.Background()
	sess, err := e.store.CreateSession(ctx, "session_mcp", "MCP", map[string]any{"status": "idle", "model": "bootstrap"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.SubmitPrompt(ctx, RunInput{SessionID: sess.ID, Prompt: "hello", Model: "bootstrap"})
	if err != nil {
		t.Fatal(err)
	}
	turn, err := e.store.GetTurn(ctx, res.TurnID)
	if err != nil {
		t.Fatal(err)
	}
	if !toolAllowedByMetadata(turn.Metadata, "mcp__fake__echo") || toolAllowedByMetadata(turn.Metadata, "mcp__fake__search_code") {
		t.Fatalf("effective tools %v", turn.Metadata["effective_tools"])
	}
}

// Servers with indirect tools are listed in the mcp_servers prompt section,
// summarised by the configured description or the server instructions.
func TestMCPServersPromptSection(t *testing.T) {
	e, _ := mcpTestEngine(t)
	section := e.mcpServersSection()
	if !strings.HasPrefix(section, "<mcp_servers>\nMCP servers whose tools are not declared to you.") ||
		!strings.Contains(section, "- mcp__fake (codemode): Fake instructions for tests.") || strings.Contains(section, "mcp__fake_direct") {
		t.Fatalf("section:\n%s", section)
	}
}

// Resource tools are registered while a server with resources is direct.
func TestMCPResourceTools(t *testing.T) {
	e, _ := mcpTestEngine(t)
	ctx := context.Background()
	list, err := e.ExecuteToolByName(ctx, "list_mcp_resources", "", map[string]any{})
	if err != nil || !strings.Contains(list, `"uri": "mem://notes/readme.txt"`) || !strings.Contains(list, `"server": "fake-direct"`) {
		t.Fatalf("list: %v\n%s", err, list)
	}
	one, err := e.ExecuteToolByName(ctx, "list_mcp_resources", "", map[string]any{"server": "fake-direct"})
	if err != nil || !strings.Contains(one, `"server": "fake-direct"`) {
		t.Fatalf("list one server: %v\n%s", err, one)
	}
	read, err := e.ExecuteToolByName(ctx, "read_mcp_resource", "", map[string]any{"server": "fake-direct", "uri": "mem://notes/readme.txt"})
	if err != nil || read != "hello resource" {
		t.Fatalf("read: %q %v", read, err)
	}
}

// A server announcing a changed tool list gets its tools re-registered.
func TestMCPToolListChangeReregisters(t *testing.T) {
	e, fake := mcpTestEngine(t)
	ctx := context.Background()
	if _, err := e.ExecuteToolByName(ctx, "mcp__fake_direct__echo", "", map[string]any{"text": "x"}); err != nil {
		t.Fatal(err)
	}
	mcptest.AddTool(fake, "late_tool", "late")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := e.tools.GetRegistered("mcp__fake_direct__late_tool"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("tool added by the server was not registered")
		}
		time.Sleep(50 * time.Millisecond)
	}
	out, err := e.ExecuteToolByName(ctx, "mcp__fake_direct__late_tool", "", map[string]any{})
	if err != nil || out != "late" {
		t.Fatalf("late tool: %q %v", out, err)
	}
}

// Saved MCP outputs older than the retention period are pruned; newer ones
// and other namespaces are kept.
func TestMCPOutputRetention(t *testing.T) {
	s := openTestStore(t)
	e := New(s)
	defer e.Close()
	ctx := context.Background()
	for _, f := range []struct{ ns, path string }{{mcpOutputNamespace, "s/old.txt"}, {mcpOutputNamespace, "s/new.txt"}, {"skills", "keep.md"}} {
		if _, err := s.SaveVFSFile(ctx, f.ns, f.path, "text/plain", []byte("x"), nil); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-mcpOutputRetention - time.Hour).UTC().Format("2006-01-02T15:04:05.000Z")
	if _, err := s.DB().ExecContext(ctx, `update vfs_files set updated_at=? where path in ('s/old.txt','keep.md')`, old); err != nil {
		t.Fatal(err)
	}
	e.pruneMCPOutput(ctx)
	var paths []string
	rows, err := s.DB().QueryContext(ctx, `select namespace||':'||path from vfs_files order by 1`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		paths = append(paths, p)
	}
	rows.Close()
	if strings.Join(paths, ",") != "mcp-output:s/new.txt,skills:keep.md" {
		t.Fatalf("after prune: %v", paths)
	}
}
