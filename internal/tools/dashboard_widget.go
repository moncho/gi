package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

const MaxWidgetHTMLBytes = 256 << 10

var widgetIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,128}$`)

func DashboardWidgetTool() RegisteredTool {
	return RegisteredTool{Name: "send_dashboard_widget", Source: "builtin", Kind: "mutating", Weight: "standard", Activation: "default",
		Description: "Post a self-contained HTML/CSS/JS dashboard widget to the current session timeline. It opens in a sandboxed pane; never execute the HTML on the server. Requires html; optional title, fallback content, open_label, interactive (default true), widget_id. Only the current session is allowed. Interactive widgets can submit text, close locally, or request a refresh through window.piclawWidget.",
		Parameters:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["html"],"properties":{"html":{"type":"string","minLength":1,"maxLength":262144},"content":{"type":"string","maxLength":4096},"title":{"type":"string","maxLength":160},"open_label":{"type":"string","maxLength":80},"interactive":{"type":"boolean"},"chat_jid":{"type":"string"},"widget_id":{"type":"string","maxLength":128}}}`),
		Executor:    ExecuteDashboardWidget, CompletesTurn: true}
}

func ExecuteDashboardWidget(ctx context.Context, rt ToolRuntime, call goai.ToolCall) (string, error) {
	if rt.Store == nil || rt.SessionID == "" {
		return "", fmt.Errorf("send_dashboard_widget: runtime session is required")
	}
	for key := range call.Arguments {
		switch key {
		case "html", "content", "title", "open_label", "interactive", "chat_jid", "widget_id":
		default:
			return "", fmt.Errorf("send_dashboard_widget: unknown argument %s", key)
		}
	}
	text := func(key string, fallback string, max int) (string, error) {
		value, present := call.Arguments[key]
		if !present {
			return fallback, nil
		}
		s, ok := value.(string)
		if !ok || len(s) > max {
			return "", fmt.Errorf("send_dashboard_widget: invalid %s", key)
		}
		if strings.TrimSpace(s) == "" {
			return fallback, nil
		}
		return s, nil
	}
	html, err := text("html", "", MaxWidgetHTMLBytes)
	if err != nil {
		return "", err
	}
	if html == "" {
		return "", fmt.Errorf("send_dashboard_widget requires html")
	}
	title, err := text("title", "Generated widget", 160)
	if err != nil {
		return "", err
	}
	content, err := text("content", "Widget ready — open to interact.", 4096)
	if err != nil {
		return "", err
	}
	label, err := text("open_label", "Open widget", 80)
	if err != nil {
		return "", err
	}
	id, err := text("widget_id", store.NowID("widget"), 128)
	if err != nil {
		return "", err
	}
	if !widgetIDPattern.MatchString(id) {
		return "", fmt.Errorf("send_dashboard_widget: invalid widget_id")
	}
	chat, err := text("chat_jid", "gi:"+rt.SessionID, 256)
	if err != nil {
		return "", err
	}
	if chat != "gi:"+rt.SessionID {
		return "", fmt.Errorf("send_dashboard_widget: only the current session is allowed")
	}
	interactive := true
	if v, present := call.Arguments["interactive"]; present {
		var ok bool
		interactive, ok = v.(bool)
		if !ok {
			return "", fmt.Errorf("send_dashboard_widget: interactive must be boolean")
		}
	}
	capabilities := []string{}
	if interactive {
		capabilities = append(capabilities, "interactive")
	}
	block := map[string]any{"type": "generated_widget", "widget_id": id, "title": title, "open_label": label, "interactive": interactive, "capabilities": capabilities, "artifact": map[string]any{"kind": "html", "html": html}}
	message, err := rt.Store.PostDashboardWidget(ctx, rt.SessionID, rt.TurnID, content, block)
	if err != nil {
		return "", err
	}
	if rt.PublishMessage != nil {
		rt.PublishMessage(message)
	}
	result := map[string]any{"status": "posted", "tool": "send_dashboard_widget", "widget_id": id, "chat_jid": chat, "post_id": message.ID}
	if rt.SetDetails != nil {
		rt.SetDetails(result)
	}
	raw, _ := json.Marshal(result)
	return string(raw), nil
}
