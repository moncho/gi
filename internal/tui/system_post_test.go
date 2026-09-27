package tui

import (
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
)

func TestStreamedErrorAndDurableSystemPostHaveOneErrorPresentation(t *testing.T) {
	const detail = "Codex error: Unsupported parameter: max_output_tokens"
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, draftLineIndex: -1, transcriptExpanded: map[string]bool{}}
	c.handleEvent(map[string]any{"type": "error", "error": detail})
	c.handleEvent(map[string]any{"type": "new_post", "sender": "system", "data": map[string]any{"type": "system_message", "content": "Inference error: " + detail}})
	blocks := c.buildTranscriptRenderableBlocks(c.transcript)
	if len(blocks) != 1 || blocks[0].Kind != "error" {
		t.Fatalf("expected one error, got %#v", blocks)
	}
	if strings.Contains(strings.Join(c.transcript, "\n"), "Gi: Inference error") {
		t.Fatal("system error labelled as assistant")
	}
	if c.running {
		t.Fatal("terminal error left turn active")
	}
	// Reopening the durable record must keep the same role presentation.
	c.transcript = c.renderMessageLines(store.Message{Role: "system", Content: "Inference error: " + detail}, 100)
	blocks = c.buildTranscriptRenderableBlocks(c.transcript)
	if len(blocks) != 1 || blocks[0].Kind != "error" {
		t.Fatalf("reload changed error role: %#v", blocks)
	}
}

func TestSystemPostDoesNotBecomeAssistantWhileAssistantProseKeepsItsRole(t *testing.T) {
	for _, tc := range []struct{ kind, sender, want string }{{"system_message", "system", "system"}, {"agent_response", "agent", "assistant"}} {
		c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, draftLineIndex: -1, transcriptExpanded: map[string]bool{}}
		c.handleEvent(map[string]any{"type": "new_post", "sender": tc.sender, "data": map[string]any{"type": tc.kind, "content": "A regular notice"}})
		blocks := c.buildTranscriptRenderableBlocks(c.transcript)
		if len(blocks) != 1 || blocks[0].Kind != tc.want {
			t.Fatalf("%s: %#v", tc.kind, blocks)
		}
	}
}
