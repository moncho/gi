package tui

import (
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/topics"
)

func TestStreamingMarkdownRerendersEachDeltaWithoutSpeakerLabels(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, outputWidth: 80, draftLineIndex: -1, stickToBottom: true}
	chunks := []string{"# Heading", "\n\n- **bold**", " `code`", "\n\n```go\n", "  keep spaces\n", "```", "\n\n| Name | Value |\n| --- | --- |\n| A | B |"}
	for _, chunk := range chunks {
		c.handleEvent(map[string]any{"type": "agent_draft_delta", "delta": chunk})
		if c.draftLineCount != len(c.transcript) || c.draftLineIndex != 0 {
			t.Fatalf("draft span lost on %q: index=%d count=%d lines=%q", chunk, c.draftLineIndex, c.draftLineCount, c.transcript)
		}
		stored := c.renderMessageLines(store.Message{Role: "assistant", Content: c.draft}, c.transcriptRenderWidth())
		if strings.Join(c.transcript, "\n") != strings.Join(stored, "\n") {
			t.Fatalf("delta %q not projected like stored Markdown: draft=%q stored=%q", chunk, c.transcript, stored)
		}
		screen := renderMarkdownScreen(t, c)
		if strings.Contains(screen, "Gi: ") || strings.Contains(screen, "you: ") {
			t.Fatalf("speaker label leaked on %q: %q", chunk, screen)
		}
	}
	before := renderMarkdownScreen(t, c)
	for _, want := range []string{"HEADING", "• bold code", "  keep spaces", "│ A    │ B     │"} {
		if !strings.Contains(before, want) {
			t.Fatalf("missing %q in streamed screen: %q", want, before)
		}
	}
	if !strings.Contains(before, "```go") || strings.Contains(before, "**bold**") || strings.Contains(before, "`code`") {
		t.Fatalf("raw Markdown in streaming output: %q", before)
	}
	c.handleEvent(map[string]any{"type": "new_post", "data": map[string]any{"content": c.draft}})
	if c.draftLineIndex != -1 || renderMarkdownScreen(t, c) != before {
		t.Fatalf("final response changed streamed rendering: %q", renderMarkdownScreen(t, c))
	}
}

func TestVisibleUserMarkdownHasNoSpeakerLabel(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, outputWidth: 80, draftLineIndex: -1}
	c.appendUserPrompt("# Heading\n\n- `item`", false)
	screen := renderMarkdownScreen(t, c)
	if strings.Contains(screen, "you: ") || !strings.Contains(screen, "HEADING") || !strings.Contains(screen, "• item") {
		t.Fatalf("user Markdown: %q", screen)
	}
}

func TestThinkingSpinnerSuppressedWhileToolIsRunning(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, draftLineIndex: -1}
	c.showThinkingIndicator(time.Now())
	c.renderToolEvent(map[string]any{"type": "tool_started", "tool": "shell", "turn_id": "turn_1", "tool_call_id": "call_1"}, time.Now())
	if transcriptContainsBlockKind(c.transcript, "thinking_indicator") {
		t.Fatalf("thinking spinner alongside running tool: %q", c.transcript)
	}
	c.showThinkingIndicator(time.Now())
	if transcriptContainsBlockKind(c.transcript, "thinking_indicator") {
		t.Fatalf("thinking spinner reappeared during tool: %q", c.transcript)
	}
	c.renderToolEvent(map[string]any{"type": "tool_finished", "tool": "shell", "turn_id": "turn_1", "tool_call_id": "call_1", "output": "ok"}, time.Now())
	c.showThinkingIndicator(time.Now())
	if !transcriptContainsBlockKind(c.transcript, "thinking_indicator") {
		t.Fatalf("thinking spinner not restored after tool: %q", c.transcript)
	}
}

// Repeated turn/session status broadcasts arrive while tools run. Neither
// status should add a second spinner, including when only one of several
// concurrent tools has completed.
func TestThinkingSpinnerStaysHiddenThroughParallelToolStatusEvents(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, sessionID: "session_1", draftLineIndex: -1}
	publish := func(topic string, payload map[string]any) {
		c.handleTopicEvent(topics.Envelope{Topic: topic, SessionID: c.sessionID, Payload: payload, Timestamp: time.Now()})
	}
	publish("turn.status", map[string]any{"status": "running"})
	if !transcriptContainsBlockKind(c.transcript, "thinking_indicator") {
		t.Fatal("expected thinking before tools start")
	}
	for _, call := range []string{"a", "b"} {
		publish("runtime.tool", map[string]any{"type": "tool_started", "tool": "shell", "turn_id": "turn_1", "tool_call_id": call})
	}
	publish("turn.status", map[string]any{"status": "running"})
	publish("runtime.session", map[string]any{"type": "session_running", "status": "running"})
	if transcriptContainsBlockKind(c.transcript, "thinking_indicator") {
		t.Fatalf("thinking indicator returned while tools run: %q", c.transcript)
	}
	if screen := renderMarkdownScreen(t, c); strings.Contains(screen, "Thinking...") || !strings.Contains(screen, "$ ...") {
		t.Fatalf("tool-only progress not visible in timeline: %q", screen)
	}
	publish("runtime.tool", map[string]any{"type": "tool_finished", "tool": "shell", "turn_id": "turn_1", "tool_call_id": "a"})
	publish("turn.status", map[string]any{"status": "running"})
	if transcriptContainsBlockKind(c.transcript, "thinking_indicator") {
		t.Fatalf("thinking indicator returned while second tool runs: %q", c.transcript)
	}
	publish("runtime.tool", map[string]any{"type": "tool_finished", "tool": "shell", "turn_id": "turn_1", "tool_call_id": "b"})
	publish("turn.status", map[string]any{"status": "running"})
	if !transcriptContainsBlockKind(c.transcript, "thinking_indicator") {
		t.Fatalf("expected thinking indicator between tools and reply: %q", c.transcript)
	}
}

func renderMarkdownScreen(t *testing.T, c *chatTUI) string {
	t.Helper()
	root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(80), gotui.WithHeight(35))
	for _, block := range c.buildTranscriptRenderableBlocks(c.transcript) {
		root.AddChild(c.renderTranscriptBlock(block))
	}
	buf := gotui.NewBuffer(80, 35)
	root.RenderTo(buf, 80, 35)
	return strings.ReplaceAll(buf.StringTrimmed(), "\u00a0", " ")
}
