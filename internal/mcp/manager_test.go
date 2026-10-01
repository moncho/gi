package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The test binary doubles as a fake stdio MCP server (GI_MCP_FAKE_SERVER=1).
func TestMain(m *testing.M) {
	if os.Getenv("GI_MCP_FAKE_SERVER") == "1" {
		_ = fakeServer().Run(context.Background(), &mcp.StdioTransport{})
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type textArgs struct {
	Text string `json:"text,omitempty"`
}

func fakeServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "fake", Version: "1"}, &mcp.ServerOptions{Instructions: "Fake server for gi tests.\nSecond line."})
	text := func(v string) *mcp.CallToolResult {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: v}}}
	}
	mcp.AddTool(s, &mcp.Tool{Name: "echo", Description: "Echo text"}, func(_ context.Context, _ *mcp.CallToolRequest, in textArgs) (*mcp.CallToolResult, any, error) {
		return text("echo:" + in.Text), nil, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "fail", Description: "Tool-level error"}, func(context.Context, *mcp.CallToolRequest, textArgs) (*mcp.CallToolResult, any, error) {
		r := text("it broke")
		r.IsError = true
		return r, nil, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "slow", Description: "Blocks until cancelled"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ textArgs) (*mcp.CallToolResult, any, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	})
	mcp.AddTool(s, &mcp.Tool{Name: "grow", Description: "Adds a tool"}, func(_ context.Context, _ *mcp.CallToolRequest, _ textArgs) (*mcp.CallToolResult, any, error) {
		mcp.AddTool(s, &mcp.Tool{Name: "grown", Description: "Added later"}, func(context.Context, *mcp.CallToolRequest, textArgs) (*mcp.CallToolResult, any, error) {
			return text("grown"), nil, nil
		})
		return text("added"), nil, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "log", Description: "Sends a log message"}, func(ctx context.Context, req *mcp.CallToolRequest, in textArgs) (*mcp.CallToolResult, any, error) {
		_ = req.Session.Log(ctx, &mcp.LoggingMessageParams{Level: "warning", Logger: "fake", Data: in.Text})
		return text("logged"), nil, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "spawn", Description: "Starts a grandchild process"}, func(context.Context, *mcp.CallToolRequest, textArgs) (*mcp.CallToolResult, any, error) {
		child := exec.Command("sleep", "300")
		if err := child.Start(); err != nil {
			return nil, nil, err
		}
		return text(fmt.Sprint(child.Process.Pid)), nil, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "crash", Description: "Exits the server"}, func(context.Context, *mcp.CallToolRequest, textArgs) (*mcp.CallToolResult, any, error) {
		os.Exit(3)
		return nil, nil, nil
	})
	return s
}

func stdioConfig(t *testing.T, extra string) Config {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := writeJSON(t, filepath.Join(t.TempDir(), "mcp.json"), fmt.Sprintf(`{"mcpServers": {"fake": {
	  "command": %q, "args": ["-test.run=^$"], "env": {"GI_MCP_FAKE_SERVER": "1"}, "timeout": 5 %s}}}`, exe, extra))
	cfg := LoadConfig(path, "", false)
	if len(cfg.Errors) > 0 {
		t.Fatal(cfg.Errors)
	}
	return cfg
}

func textOf(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if r == nil || len(r.Content) == 0 {
		t.Fatalf("empty result %+v", r)
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("not text: %T", r.Content[0])
	}
	return tc.Text
}

func TestStdioServerToolsCallsAndErrors(t *testing.T) {
	ctx := context.Background()
	logPath := filepath.Join(t.TempDir(), "mcp.log")
	m := NewManager(stdioConfig(t, ""), t.TempDir(), logPath)
	defer m.Close()
	tools, err := m.Tools(ctx, "fake")
	if err != nil || len(tools) != 7 {
		t.Fatalf("tools: %d %v", len(tools), err)
	}
	if st := m.Status()[0]; st.State != StateConnected || st.Tools != 7 || !strings.HasPrefix(st.Instructions, "Fake server") {
		t.Fatalf("status %+v", st)
	}
	if r, err := m.CallTool(ctx, "fake", "echo", map[string]any{"text": "hi"}); err != nil || textOf(t, r) != "echo:hi" {
		t.Fatalf("echo: %v", err)
	}
	if r, err := m.CallTool(ctx, "fake", "fail", map[string]any{}); err != nil || !r.IsError {
		t.Fatalf("isError must be a result, not an error: %+v %v", r, err)
	}
	if _, err := m.CallTool(ctx, "fake", "nope", map[string]any{}); err == nil {
		t.Fatal("unknown tool accepted")
	}
	// Logging notifications land in mcp.log as "[server] level logger: message".
	if _, err := m.CallTool(ctx, "fake", "log", map[string]any{"text": "disk nearly full"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, _ := os.ReadFile(logPath)
		if strings.Contains(string(data), "[fake] warning fake: disk nearly full") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("log line missing: %q", data)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// A changed tool list is picked up; a cancelled or timed-out call keeps the
// session; a dropped connection reconnects on the next call.
func TestStdioListChangeCancellationAndReconnect(t *testing.T) {
	ctx := context.Background()
	m := NewManager(stdioConfig(t, ""), t.TempDir(), "")
	defer m.Close()
	if _, err := m.CallTool(ctx, "fake", "grow", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		tools, err := m.Tools(ctx, "fake")
		if err != nil {
			t.Fatal(err)
		}
		if len(tools) == 8 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("list change not picked up: %d tools", len(tools))
		}
		time.Sleep(50 * time.Millisecond)
	}
	cctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	_, err := m.CallTool(cctx, "fake", "slow", map[string]any{})
	cancel()
	if err == nil {
		t.Fatal("cancelled call succeeded")
	}
	if r, err := m.CallTool(ctx, "fake", "echo", map[string]any{"text": "still here"}); err != nil || textOf(t, r) != "echo:still here" {
		t.Fatalf("session lost after cancellation: %v", err)
	}
	if _, err := m.CallTool(ctx, "fake", "crash", map[string]any{}); err == nil {
		t.Fatal("crash call succeeded")
	}
	if r, err := m.CallTool(ctx, "fake", "echo", map[string]any{"text": "again"}); err != nil || textOf(t, r) != "echo:again" {
		t.Fatalf("no reconnect after a dropped connection: %v", err)
	}
}

// Per-request timeout from the config.
func TestStdioCallTimeout(t *testing.T) {
	cfg := stdioConfig(t, "")
	s := cfg.Servers["fake"]
	s.Timeout = 300 * time.Millisecond
	cfg.Servers["fake"] = s
	m := NewManager(cfg, t.TempDir(), "")
	defer m.Close()
	_, err := m.CallTool(context.Background(), "fake", "slow", map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout: %v", err)
	}
}

// Close stops the whole process group, including grandchildren of wrappers.
func TestCloseStopsProcessGroup(t *testing.T) {
	m := NewManager(stdioConfig(t, ""), t.TempDir(), "")
	r, err := m.CallTool(context.Background(), "fake", "spawn", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	if err := json.Unmarshal([]byte(textOf(t, r)), &pid); err != nil {
		t.Fatal(err)
	}
	m.Close()
	deadline := time.Now().Add(6 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatal("grandchild survived Close")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestDisabledAndFailingServers(t *testing.T) {
	path := writeJSON(t, filepath.Join(t.TempDir(), "mcp.json"), `{"mcpServers": {
	  "off": {"command": "true", "enabled": false},
	  "missing": {"command": "/nonexistent/server-binary"}}}`)
	m := NewManager(LoadConfig(path, "", false), t.TempDir(), "")
	defer m.Close()
	if _, err := m.Tools(context.Background(), "off"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled: %v", err)
	}
	if _, err := m.Tools(context.Background(), "missing"); err == nil {
		t.Fatal("missing binary connected")
	}
	states := map[string]string{}
	for _, st := range m.Status() {
		states[st.Name] = st.State
	}
	if states["off"] != StateDisabled || states["missing"] != StateFailed {
		t.Fatalf("states %v", states)
	}
}

// Streamable HTTP with headers expanded from the environment.
func TestHTTPServerWithHeaders(t *testing.T) {
	var gotAuth string
	handler := mcp.NewStreamableHTTPHandler(func(*httpRequest) *mcp.Server { return fakeServer() }, nil)
	srv := httptest.NewServer(captureAuth(handler, &gotAuth))
	defer srv.Close()
	t.Setenv("GI_MCP_TEST_TOKEN", "secret-token")
	path := writeJSON(t, filepath.Join(t.TempDir(), "mcp.json"), fmt.Sprintf(`{"mcpServers": {"web": {
	  "url": %q, "headers": {"Authorization": "Bearer ${GI_MCP_TEST_TOKEN}"}}}}`, srv.URL))
	m := NewManager(LoadConfig(path, "", false), t.TempDir(), "")
	defer m.Close()
	r, err := m.CallTool(context.Background(), "web", "echo", map[string]any{"text": "over http"})
	if err != nil || textOf(t, r) != "echo:over http" {
		t.Fatalf("http call: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("Authorization header %q", gotAuth)
	}
}
