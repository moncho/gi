package tui

import (
	"github.com/rcarmo/gi/internal/topics"
	"strings"
	"testing"
)

func TestProviderRetryStatusKeepsTUIRunningWithoutErrorPost(t *testing.T) {
	c := sessionTestChat(t)
	c.input.SetText("new unsent draft")
	title := "Provider request timed out — retrying (attempt 1/3, 2s delay)"
	c.handleTopicEvent(topics.Envelope{Topic: "turn.status", SessionID: c.sessionID, Payload: map[string]any{"type": "agent_status", "phase": "retry_wait", "status": "running", "title": title}})
	if !c.running || c.status != title || c.input.Text() != "new unsent draft" {
		t.Fatal(c.running, c.status, c.input.Text())
	}
	blocks := c.buildTranscriptRenderableBlocks(c.transcript)
	found := false
	for _, b := range blocks {
		if b.Kind == "error" {
			t.Fatal("retry displayed as error")
		}
		if strings.Contains(b.Header, "retrying") {
			found = true
		}
	}
	if !found {
		t.Fatalf("retry title absent %#v", blocks)
	}
	c.handleEvent(map[string]any{"type": "new_post", "sender": "agent", "data": map[string]any{"type": "agent_response", "content": "recovered"}})
	if c.running {
		t.Fatal("success still running")
	}
	for _, b := range c.buildTranscriptRenderableBlocks(c.transcript) {
		if strings.Contains(b.Header, "retrying") {
			t.Fatal("retry status survived completion")
		}
	}
}
