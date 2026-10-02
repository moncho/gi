package sessionexport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/store"
)

func exportFixture(t *testing.T) (*store.Store, string) {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "gi.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	const sid = "session_export"
	if _, err := s.CreateSession(ctx, sid, "Export <demo>", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, "turn_a", sid, "completed", "list files", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	add := func(id, role, content string, payload map[string]any) {
		t.Helper()
		if err := s.AddMessage(ctx, id, sid, role, content, payload); err != nil {
			t.Fatal(err)
		}
	}
	// New-style tool calls carry full arguments.
	add("m1", "user", "list files", map[string]any{"turn_id": "turn_a"})
	add("m2", "assistant", "[tool_call: shell]", map[string]any{"kind": "tool_calls", "model": "github-copilot/gpt-5-mini", "turn_id": "turn_a",
		"tool_calls": []any{map[string]any{"id": "call_1", "name": "shell", "arguments": map[string]any{"command": "ls -la"}}}})
	add("m3", "tool_result", "a.go\nb.go", map[string]any{"tool_call_id": "call_1", "is_error": false, "turn_id": "turn_a"})
	// Legacy tool calls: arguments come from the tool.started preview.
	add("m4", "assistant", "[tool_call: read]", map[string]any{"kind": "tool_calls", "model": "github-copilot/gpt-5-mini", "turn_id": "turn_a"})
	if err := s.AppendTurnEvent(ctx, "turn_a", sid, "tool.started", map[string]any{"tool": "read", "tool_call_id": "call_2", "preview": "a.go"}); err != nil {
		t.Fatal(err)
	}
	add("m5", "tool_result", "package a", map[string]any{"tool_call_id": "call_2", "is_error": true, "turn_id": "turn_a"})
	add("m6", "assistant", "Two files: <a.go> & b.go", map[string]any{"kind": "chat", "model": "github-copilot/gpt-5-mini", "turn_id": "turn_a"})
	add("m7", "system", "compaction completed", map[string]any{})
	return s, sid
}

// The JSONL export follows Pi's session format v3: a header, then a linear
// id/parentId chain of message entries with Pi's roles and content blocks.
func TestJSONLFollowsPiSessionFormat(t *testing.T) {
	s, sid := exportFixture(t)
	data, err := JSONL(context.Background(), s, sid, "/work")
	if err != nil {
		t.Fatal(err)
	}
	var lines []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("invalid JSON line %q: %v", sc.Text(), err)
		}
		lines = append(lines, m)
	}
	if h := lines[0]; h["type"] != "session" || h["version"] != float64(3) || h["cwd"] != "/work" || h["id"] != sid {
		t.Fatalf("header %v", h)
	}
	if lines[1]["type"] != "session_info" || lines[1]["name"] != "Export <demo>" || lines[1]["parentId"] != nil {
		t.Fatalf("name entry %v", lines[1])
	}
	for i := 2; i < len(lines); i++ {
		if lines[i]["parentId"] != lines[i-1]["id"] {
			t.Fatalf("entry %d parentId %v, want %v", i, lines[i]["parentId"], lines[i-1]["id"])
		}
	}
	msg := func(i int) map[string]any { return lines[i]["message"].(map[string]any) }
	if m := msg(2); m["role"] != "user" || m["content"] != "list files" {
		t.Fatalf("user %v", m)
	}
	call := msg(3)["content"].([]any)[0].(map[string]any)
	if msg(3)["stopReason"] != "toolUse" || msg(3)["provider"] != "github-copilot" || msg(3)["model"] != "gpt-5-mini" ||
		call["type"] != "toolCall" || call["id"] != "call_1" || call["arguments"].(map[string]any)["command"] != "ls -la" {
		t.Fatalf("assistant tool call %v", msg(3))
	}
	if m := msg(4); m["role"] != "toolResult" || m["toolCallId"] != "call_1" || m["toolName"] != "shell" || m["isError"] != false {
		t.Fatalf("tool result %v", m)
	}
	legacy := msg(5)["content"].([]any)[0].(map[string]any)
	if legacy["id"] != "call_2" || legacy["name"] != "read" || legacy["arguments"].(map[string]any)["preview"] != "a.go" {
		t.Fatalf("legacy call %v", legacy)
	}
	if m := msg(6); m["toolName"] != "read" || m["isError"] != true {
		t.Fatalf("legacy result %v", m)
	}
	if m := msg(7); m["stopReason"] != "stop" || m["content"].([]any)[0].(map[string]any)["text"] != "Two files: <a.go> & b.go" {
		t.Fatalf("assistant text %v", m)
	}
	if e := lines[8]; e["type"] != "custom" || e["customType"] != "gi.system" {
		t.Fatalf("system notice %v", e)
	}
}

// Export picks HTML by default and JSONL for .jsonl paths, resolved against cwd.
func TestExportPathsAndHTMLEscaping(t *testing.T) {
	s, sid := exportFixture(t)
	cwd := t.TempDir()
	path, err := Export(context.Background(), s, sid, cwd, "")
	if err != nil || path != filepath.Join(cwd, "gi-session-"+sid+".html") {
		t.Fatalf("default path %q %v", path, err)
	}
	page, _ := os.ReadFile(path)
	if !strings.Contains(string(page), "Two files: &lt;a.go&gt; &amp; b.go") || strings.Contains(string(page), "<a.go>") {
		t.Fatal("HTML export does not escape content")
	}
	path, err = Export(context.Background(), s, sid, cwd, "out/session.jsonl")
	if err != nil || path != filepath.Join(cwd, "out", "session.jsonl") {
		t.Fatalf("jsonl path %q %v", path, err)
	}
	data, _ := os.ReadFile(path)
	var header map[string]any
	if err := json.Unmarshal([]byte(strings.SplitN(string(data), "\n", 2)[0]), &header); err != nil || header["type"] != "session" {
		t.Fatalf("jsonl header %.120s (%v)", data, err)
	}
}
