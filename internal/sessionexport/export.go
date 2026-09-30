// Package sessionexport writes a gi session as a Pi-compatible JSONL session
// file (format version 3, importable by Pi's /import) or a standalone HTML
// transcript, like Pi's /export.
package sessionexport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

// Entry is one Pi session line. Fields follow docs/session-format.md.
type Entry map[string]any

type toolCall struct {
	ID, Name  string
	Arguments map[string]any
	Preview   bool // arguments reconstructed from a tool.started preview
}

// item is one exported conversation element, before serialization.
type item struct {
	role      string // user | assistant | toolResult | system
	text      string
	calls     []toolCall
	callID    string
	toolName  string
	isError   bool
	model     string
	createdAt time.Time
	sourceID  string
}

// Load reads a session's messages (and tool.started previews for sessions
// recorded before full tool calls were stored) into export items.
func load(ctx context.Context, s *store.Store, sessionID string) ([]item, error) {
	messages, err := s.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	previews := map[string]string{} // tool_call_id -> preview
	turns := map[string]bool{}
	for _, m := range messages {
		if id, _ := m.Payload["turn_id"].(string); id != "" {
			turns[id] = true
		}
	}
	for turnID := range turns {
		events, err := s.ListTurnEvents(ctx, turnID)
		if err != nil {
			continue
		}
		for _, ev := range events {
			if ev.Type != "tool.started" {
				continue
			}
			id, _ := ev.Payload["tool_call_id"].(string)
			preview, _ := ev.Payload["preview"].(string)
			if id != "" {
				previews[id] = preview
			}
		}
	}

	var items []item
	for i, m := range messages {
		it := item{text: m.Content, createdAt: parseTime(m.CreatedAt), sourceID: m.ID}
		it.model, _ = m.Payload["model"].(string)
		switch m.Role {
		case "user":
			it.role = "user"
		case "assistant":
			it.role = "assistant"
			if kind, _ := m.Payload["kind"].(string); kind == "tool_calls" {
				it.text, _ = m.Payload["display_text"].(string)
				it.calls = recordedCalls(m.Payload)
				if len(it.calls) == 0 {
					it.calls = reconstructCalls(m.Content, messages[i+1:], previews)
				}
			}
		case "tool_result":
			it.role = "toolResult"
			it.callID, _ = m.Payload["tool_call_id"].(string)
			it.isError, _ = m.Payload["is_error"].(bool)
		case "system":
			it.role = "system"
		default:
			continue
		}
		items = append(items, it)
	}
	// Tool results carry names from their calls.
	names := map[string]string{}
	for _, it := range items {
		for _, c := range it.calls {
			names[c.ID] = c.Name
		}
	}
	for i := range items {
		if items[i].role == "toolResult" {
			items[i].toolName = names[items[i].callID]
		}
	}
	return items, nil
}

func recordedCalls(payload map[string]any) []toolCall {
	raw, ok := payload["tool_calls"].([]any)
	if !ok {
		return nil
	}
	var calls []toolCall
	for _, r := range raw {
		m, _ := r.(map[string]any)
		id, _ := m["id"].(string)
		name, _ := m["name"].(string)
		args, _ := m["arguments"].(map[string]any)
		if id == "" || name == "" {
			continue
		}
		if args == nil {
			args = map[string]any{}
		}
		calls = append(calls, toolCall{ID: id, Name: name, Arguments: args})
	}
	return calls
}

// reconstructCalls rebuilds calls for sessions recorded before full calls were
// stored: names come from the "[tool_call: name]" summary, IDs from the tool
// results that follow, arguments from the tool.started preview.
func reconstructCalls(summary string, following []store.Message, previews map[string]string) []toolCall {
	var names []string
	for _, line := range strings.Split(summary, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[tool_call: ") && strings.HasSuffix(line, "]") {
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(line, "[tool_call: "), "]"))
		}
	}
	var calls []toolCall
	for _, m := range following {
		if m.Role != "tool_result" || len(calls) == len(names) {
			break
		}
		id, _ := m.Payload["tool_call_id"].(string)
		if id == "" {
			continue
		}
		args := map[string]any{}
		if preview, ok := previews[id]; ok && preview != "" {
			args["preview"] = preview
		}
		calls = append(calls, toolCall{ID: id, Name: names[len(calls)], Arguments: args, Preview: true})
	}
	return calls
}

func parseTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func entryID(sourceID string) string {
	sum := sha256.Sum256([]byte(sourceID))
	return hex.EncodeToString(sum[:4])
}

func splitModel(label string) (provider, model string) {
	if i := strings.Index(label, "/"); i > 0 {
		return label[:i], label[i+1:]
	}
	return "", label
}

func isoTime(t time.Time) string {
	if t.IsZero() {
		t = time.Unix(0, 0).UTC()
	}
	return t.Format("2006-01-02T15:04:05.000Z")
}

// JSONL serializes a session as Pi session format version 3.
func JSONL(ctx context.Context, s *store.Store, sessionID, cwd string) ([]byte, error) {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	items, err := load(ctx, s, sessionID)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	header := Entry{"type": "session", "version": 3, "id": sessionID, "timestamp": isoTime(parseTime(sess.CreatedAt)), "cwd": cwd}
	if err := enc.Encode(header); err != nil {
		return nil, err
	}
	var parent any // null for the root entry
	emit := func(e Entry, sourceID string, at time.Time) error {
		id := entryID(sourceID)
		e["id"], e["parentId"], e["timestamp"] = id, parent, isoTime(at)
		parent = id
		return enc.Encode(e)
	}
	if title := strings.TrimSpace(sess.Title); title != "" {
		if err := emit(Entry{"type": "session_info", "name": title}, sessionID+"#name", parseTime(sess.CreatedAt)); err != nil {
			return nil, err
		}
	}
	for _, it := range items {
		ms := it.createdAt.UnixMilli()
		var e Entry
		switch it.role {
		case "user":
			e = Entry{"type": "message", "message": map[string]any{"role": "user", "content": it.text, "timestamp": ms}}
		case "assistant":
			content := []any{}
			if strings.TrimSpace(it.text) != "" {
				content = append(content, map[string]any{"type": "text", "text": it.text})
			}
			for _, c := range it.calls {
				content = append(content, map[string]any{"type": "toolCall", "id": c.ID, "name": c.Name, "arguments": c.Arguments})
			}
			provider, model := splitModel(it.model)
			stop := "stop"
			if len(it.calls) > 0 {
				stop = "toolUse"
			}
			msg := map[string]any{"role": "assistant", "content": content, "provider": provider, "model": model, "stopReason": stop, "timestamp": ms,
				"usage": map[string]any{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0, "totalTokens": 0, "cost": map[string]any{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0, "total": 0}}}
			if m := goai.GetModel(goai.Provider(provider), model); m != nil {
				msg["api"] = string(m.Api)
			}
			e = Entry{"type": "message", "message": msg}
		case "toolResult":
			e = Entry{"type": "message", "message": map[string]any{"role": "toolResult", "toolCallId": it.callID, "toolName": it.toolName,
				"content": []any{map[string]any{"type": "text", "text": it.text}}, "isError": it.isError, "timestamp": ms}}
		case "system":
			// gi notices are not model context: a Pi custom entry, not a message.
			e = Entry{"type": "custom", "customType": "gi.system", "data": map[string]any{"text": it.text}}
		default:
			continue
		}
		if err := emit(e, it.sourceID, it.createdAt); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// HTML renders a standalone, self-contained transcript.
func HTML(ctx context.Context, s *store.Store, sessionID string) ([]byte, error) {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	items, err := load(ctx, s, sessionID)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(sess.Title)
	if title == "" {
		title = sessionID
	}
	var b strings.Builder
	esc := html.EscapeString
	fmt.Fprintf(&b, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>
body{font:15px/1.5 system-ui,sans-serif;max-width:52rem;margin:2rem auto;padding:0 1rem;color:#1f2328;background:#fff}
@media(prefers-color-scheme:dark){body{color:#e6edf3;background:#0d1117}.user{background:#161b22!important}pre{background:#161b22!important}}
h1{font-size:1.3rem}.meta{color:#8b949e;font-size:.85rem}.msg{margin:1rem 0;padding:.6rem .9rem;border-radius:6px}
.user{background:#f6f8fa}.system{color:#8b949e;font-style:italic}.who{font-weight:600;font-size:.8rem;text-transform:uppercase;color:#8b949e}
pre{white-space:pre-wrap;word-break:break-word;background:#f6f8fa;padding:.6rem;border-radius:6px;font:13px/1.45 ui-monospace,monospace}
details{margin:.4rem 0}summary{cursor:pointer;font:13px ui-monospace,monospace}.error summary{color:#cf222e}
</style></head><body><h1>%s</h1><p class="meta">%s · exported by gi %s</p>
`, esc(title), esc(title), esc(sessionID), esc(time.Now().UTC().Format("2006-01-02 15:04 UTC")))
	for _, it := range items {
		switch it.role {
		case "user":
			fmt.Fprintf(&b, `<div class="msg user"><div class="who">user</div><pre>%s</pre></div>`+"\n", esc(it.text))
		case "assistant":
			b.WriteString(`<div class="msg assistant"><div class="who">assistant</div>`)
			if strings.TrimSpace(it.text) != "" {
				fmt.Fprintf(&b, `<pre>%s</pre>`, esc(it.text))
			}
			for _, c := range it.calls {
				args, _ := json.MarshalIndent(c.Arguments, "", "  ")
				fmt.Fprintf(&b, `<details><summary>→ %s</summary><pre>%s</pre></details>`, esc(c.Name), esc(string(args)))
			}
			b.WriteString("</div>\n")
		case "toolResult":
			class := ""
			if it.isError {
				class = ` class="error"`
			}
			fmt.Fprintf(&b, `<details%s><summary>← %s result</summary><pre>%s</pre></details>`+"\n", class, esc(it.toolName), esc(it.text))
		case "system":
			fmt.Fprintf(&b, `<div class="msg system">%s</div>`+"\n", esc(it.text))
		}
	}
	b.WriteString("</body></html>\n")
	return []byte(b.String()), nil
}

// Export writes the session to path (resolved against cwd) as JSONL when the
// path ends in .jsonl and as HTML otherwise, like Pi's /export. An empty path
// writes gi-session-<session>.html in cwd. It returns the written path.
func Export(ctx context.Context, s *store.Store, sessionID, cwd, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = "gi-session-" + sessionID + ".html"
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	var data []byte
	var err error
	if strings.HasSuffix(strings.ToLower(path), ".jsonl") {
		data, err = JSONL(ctx, s, sessionID, cwd)
	} else {
		data, err = HTML(ctx, s, sessionID)
	}
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
