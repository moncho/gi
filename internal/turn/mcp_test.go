package turn

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/inference"
	gimcp "github.com/rcarmo/gi/internal/mcp"
	"github.com/rcarmo/gi/internal/mcp/mcptest"
	"github.com/rcarmo/gi/internal/tools"
	goai "github.com/rcarmo/go-ai"
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
	// codemode/deferred tools are registered deferred (not declared); hidden
	// tools are not registered at all.
	if tool, ok := e.tools.GetRegistered("mcp__fake__search_code"); !ok || !tool.Deferred {
		t.Fatalf("codemode tool must be registered deferred: %+v %v", tool, ok)
	}
	for _, def := range e.tools.Definitions() {
		if def.Name == "mcp__fake__search_code" {
			t.Fatal("deferred tool declared by default")
		}
	}
	if _, ok := e.tools.GetRegistered("mcp__fake__delete_all"); ok {
		t.Fatal("hidden tool registered")
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

// An image in an MCP result reaches the model as an image block of the tool
// result (Pi); the stored transcript describes it.
func TestMCPToolResultImagesReachModel(t *testing.T) {
	e, fake := mcpTestEngine(t)
	mcptest.AddPictureTool(fake)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := e.tools.GetRegistered("mcp__fake_direct__picture"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("picture tool not registered")
		}
		time.Sleep(50 * time.Millisecond)
	}
	ctx := context.Background()
	seen := make(chan []goai.ContentBlock, 1)
	calls := 0
	withStreamWithToolsStub(t, func(_ context.Context, _ string, conv *goai.Context, _ func(map[string]any)) (*inference.StreamResult, error) {
		calls++
		if calls == 1 {
			return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonToolUse,
				Content: []goai.ContentBlock{{Type: "toolCall", ID: "tc_pic", Name: "mcp__fake_direct__picture", Arguments: map[string]any{}}}}}, nil
		}
		for _, m := range conv.Messages {
			if m.Role == goai.RoleToolResult && m.ToolCallID == "tc_pic" {
				seen <- m.Content
			}
		}
		return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "done"}}}}, nil
	})
	sess, err := e.store.CreateSession(ctx, "session_mcp_img", "MCP", map[string]any{"status": "idle", "model": "mock-img"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.SubmitPrompt(ctx, RunInput{SessionID: sess.ID, Prompt: "show me", Model: "mock-img"}); err != nil {
		t.Fatal(err)
	}
	var content []goai.ContentBlock
	select {
	case content = <-seen:
	case <-time.After(10 * time.Second):
		t.Fatal("tool result never reached the model")
	}
	if len(content) != 2 || content[0].Text != "here it is" || content[1].Type != "image" || content[1].MimeType != "image/png" || content[1].Data != base64.StdEncoding.EncodeToString(mcptest.PNG) {
		t.Fatalf("tool result content %+v", content)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		msgs, _ := e.store.ListMessages(ctx, sess.ID)
		for _, m := range msgs {
			if m.Role == "tool_result" && m.Content == "here it is\n[image image/png, 12B]" {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("stored transcript lacks the image note: %+v", msgs)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func toolSearchEngine(t *testing.T) (*Engine, *mcptest.Server) {
	t.Helper()
	fake := mcptest.New("Code hosting tools.")
	t.Cleanup(fake.Close)
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(fmt.Sprintf(`{"mcpServers": {"hub": {"url": %q, "exposure": "deferred", "description": "Code hosting"}}}`, fake.URL)), 0o600); err != nil {
		t.Fatal(err)
	}
	s := openTestStore(t)
	e := New(s)
	t.Cleanup(func() { e.Close(); s.Close() })
	e.enableMCPWith(gimcp.NewManager(gimcp.LoadConfig(path, "", false), t.TempDir(), ""))
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := e.tools.GetRegistered("mcp__hub__search_code"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("deferred tools not registered")
		}
		time.Sleep(50 * time.Millisecond)
	}
	return e, fake
}

// tool_search ranks deferred tools, loads matches for the session (persisted)
// and reports them like Pi; loaded tools are then allowed and declared.
func TestToolSearchLoadsDeferredTools(t *testing.T) {
	e, _ := toolSearchEngine(t)
	ctx := context.Background()
	if _, ok := e.tools.GetRegistered(tools.ToolSearchName); !ok {
		t.Fatal("tool_search not registered for a deferred server")
	}
	sess, err := e.store.CreateSession(ctx, "session_ts", "TS", map[string]any{"status": "idle", "model": "bootstrap"})
	if err != nil {
		t.Fatal(err)
	}
	var added []string
	rt := tools.ToolRuntime{Store: e.store, SessionID: sess.ID, AddTools: func(n []string) { added = append(added, n...) }}
	search, _ := e.tools.GetRegistered(tools.ToolSearchName)
	out, err := search.Executor(ctx, rt, goai.ToolCall{Name: tools.ToolSearchName, Arguments: map[string]any{"query": "search code", "limit": float64(1)}})
	if err != nil || out != "Loaded 1 tool. They are available from your next call:\n- mcp__hub__search_code: Search code" {
		t.Fatalf("tool_search: %q %v", out, err)
	}
	if strings.Join(added, ",") != "mcp__hub__search_code" || !e.sessionLoadedTool(ctx, sess.ID, "mcp__hub__search_code") {
		t.Fatalf("not loaded: %v", added)
	}
	// Already-loaded tools are no longer candidates.
	if again, _ := search.Executor(ctx, rt, goai.ToolCall{Arguments: map[string]any{"query": "search code"}}); strings.Contains(again, "mcp__hub__search_code") {
		t.Fatalf("loaded tool offered again: %q", again)
	}
	if none, _ := search.Executor(ctx, rt, goai.ToolCall{Arguments: map[string]any{"query": "zzzz"}}); none != "No matching tools found." {
		t.Fatalf("no match: %q", none)
	}
	if _, err := search.Executor(ctx, rt, goai.ToolCall{Arguments: map[string]any{"query": " "}}); err == nil || err.Error() != "query must not be empty" {
		t.Fatalf("empty query: %v", err)
	}
	if _, err := search.Executor(ctx, rt, goai.ToolCall{Arguments: map[string]any{"query": "x", "limit": float64(0)}}); err == nil || err.Error() != "limit must be a positive integer" {
		t.Fatalf("bad limit: %v", err)
	}
	conv := &goai.Context{}
	e.declareLoadedTools(conv, e.sessionLoadedTools(ctx, sess.ID))
	declared := map[string]bool{}
	for _, tool := range conv.Tools {
		declared[tool.Name] = true
	}
	if !declared["mcp__hub__search_code"] || len(conv.Tools) != len(e.sessionLoadedTools(ctx, sess.ID)) {
		t.Fatalf("declared %v", conv.Tools)
	}
}

// Through a real turn: the model calls tool_search, the result carries the
// AddedToolNames marker, the next request declares the tool, and the model
// can call it.
func TestToolSearchInTurn(t *testing.T) {
	e, _ := toolSearchEngine(t)
	ctx := context.Background()
	type seen struct {
		marker   []string
		declared bool
		output   string
	}
	got := make(chan seen, 1)
	calls := 0
	withStreamWithToolsStub(t, func(_ context.Context, _ string, conv *goai.Context, _ func(map[string]any)) (*inference.StreamResult, error) {
		calls++
		toolUse := func(id, name string, args map[string]any) (*inference.StreamResult, error) {
			return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonToolUse,
				Content: []goai.ContentBlock{{Type: "toolCall", ID: id, Name: name, Arguments: args}}}}, nil
		}
		switch calls {
		case 1:
			return toolUse("tc_search", tools.ToolSearchName, map[string]any{"query": "upper case text", "limit": float64(1)})
		case 2:
			return toolUse("tc_upper", "mcp__hub__upper", map[string]any{"text": "abc"})
		}
		var s seen
		for _, m := range conv.Messages {
			if m.Role == goai.RoleToolResult && m.ToolCallID == "tc_search" {
				s.marker = m.AddedToolNames
			}
			if m.Role == goai.RoleToolResult && m.ToolCallID == "tc_upper" && len(m.Content) > 0 {
				s.output = m.Content[0].Text
			}
		}
		for _, tool := range conv.Tools {
			if tool.Name == "mcp__hub__upper" {
				s.declared = true
			}
		}
		got <- s
		return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "done"}}}}, nil
	})
	sess, err := e.store.CreateSession(ctx, "session_ts_turn", "TS", map[string]any{"status": "idle", "model": "mock-ts"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.SubmitPrompt(ctx, RunInput{SessionID: sess.ID, Prompt: "uppercase abc", Model: "mock-ts"}); err != nil {
		t.Fatal(err)
	}
	select {
	case s := <-got:
		if strings.Join(s.marker, ",") != "mcp__hub__upper" || !s.declared || s.output != "ABC" {
			t.Fatalf("turn: %+v", s)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("turn did not reach the final call")
	}
}

// /mcp reconnect drops and re-establishes a server's connection and keeps
// its tools registered.
func TestMCPReconnect(t *testing.T) {
	e, _ := mcpTestEngine(t)
	ctx := context.Background()
	if err := e.MCPReconnect(ctx, "fake_direct"); err == nil {
		t.Fatal("unknown server reconnected")
	}
	if err := e.MCPReconnect(ctx, "fake-direct"); err != nil {
		t.Fatal(err)
	}
	statuses, _ := e.MCPStatus()
	for _, st := range statuses {
		if st.Name == "fake-direct" && (st.State != gimcp.StateConnected || st.Tools == 0) {
			t.Fatalf("after reconnect: %+v", st)
		}
	}
	if _, ok := e.tools.GetRegistered("mcp__fake_direct__echo"); !ok {
		t.Fatal("tools lost after reconnect")
	}
}

// Pi's startup report: once every enabled server finished its first
// connection attempt, config errors and failed servers are reported in one
// message, delivered to a notifier set later too.
func TestMCPStartupProblemReport(t *testing.T) {
	fake := mcptest.New("ok")
	t.Cleanup(fake.Close)
	dead := hangingListener(t, false)
	path := filepath.Join(t.TempDir(), "mcp.json")
	body := fmt.Sprintf(`{"mcpServers": {"good": {"url": %q}, "dead": {"url": %q}, "broken": {}}}`, fake.URL, dead)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := gimcp.LoadConfig(path, "", false)
	if len(cfg.Errors) != 1 {
		t.Fatal(cfg.Errors)
	}
	s := openTestStore(t)
	e := New(s)
	t.Cleanup(func() { e.Close(); s.Close() })
	e.enableMCPWith(gimcp.NewManager(cfg, t.TempDir(), ""))
	deadline := time.Now().Add(10 * time.Second)
	for {
		e.mcpNotices.mu.Lock()
		n := len(e.mcpNotices.pending)
		e.mcpNotices.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no startup report")
		}
		time.Sleep(20 * time.Millisecond)
	}
	var got [][2]string
	e.SetMCPNotifier(func(level, text string) { got = append(got, [2]string{level, text}) })
	if len(got) != 1 || got[0][0] != "warning" {
		t.Fatalf("%q", got)
	}
	lines := strings.Split(got[0][1], "\n")
	if len(lines) != 4 || lines[0] != "MCP servers need attention:" || lines[1] != "  config: "+cfg.Errors[0].Error() ||
		!strings.HasPrefix(lines[2], "  dead: failed: ") || lines[3] != "Run /mcp to fix." {
		t.Fatalf("%q", lines)
	}
}

// Pi's waitForDirectServers: only the first prompt waits, and when a direct
// server is still connecting it says so.
func TestMCPDirectWaitOnlyOnce(t *testing.T) {
	old := mcpDirectWait
	mcpDirectWait = 200 * time.Millisecond
	t.Cleanup(func() { mcpDirectWait = old })
	slow := hangingListener(t, true)
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(fmt.Sprintf(`{"mcpServers": {"slow": {"url": %q, "exposure": "direct"}}}`, slow)), 0o600); err != nil {
		t.Fatal(err)
	}
	s := openTestStore(t)
	e := New(s)
	t.Cleanup(func() { e.Close(); s.Close() })
	var notices []string
	e.SetMCPNotifier(func(level, text string) { notices = append(notices, level+": "+text) })
	e.enableMCPWith(gimcp.NewManager(gimcp.LoadConfig(path, "", false), t.TempDir(), ""))
	start := time.Now()
	e.awaitDirectMCPTools(context.Background())
	if waited := time.Since(start); waited < 150*time.Millisecond {
		t.Fatalf("first prompt did not wait: %v", waited)
	}
	start = time.Now()
	e.awaitDirectMCPTools(context.Background())
	if waited := time.Since(start); waited > 50*time.Millisecond {
		t.Fatalf("later prompt waited: %v", waited)
	}
	if len(notices) != 1 || notices[0] != "info: "+gimcp.StillConnectingNotice {
		t.Fatalf("%q", notices)
	}
}

// hangingListener returns an HTTP URL whose connections either close at once
// (a failed server) or are held open without a response (still connecting).
func hangingListener(t *testing.T, hold bool) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var conns []net.Conn
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		for _, c := range conns {
			c.Close()
		}
		mu.Unlock()
	})
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			if !hold {
				c.Close()
				continue
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	return "http://" + ln.Addr().String() + "/mcp"
}
