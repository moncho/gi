package inference

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	goai "github.com/rcarmo/go-ai"
)

// Opt-in live request; no credential or response body is logged on failure.
func TestLiveCopilotMinimalProbe(t *testing.T) {
	if !strings.EqualFold(os.Getenv("GI_RUN_LIVE_COPILOT_PROBE"), "1") {
		t.Skip("set GI_RUN_LIVE_COPILOT_PROBE=1 for a bounded live provider request")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := StreamWithTools(ctx, "github-copilot/gpt-5.4-mini", &goai.Context{Messages: []goai.Message{goai.UserMessage("Reply with the single word READY.")}}, nil)
	if err != nil {
		t.Fatalf("live provider request failed: %v", err)
	}
	if result == nil || strings.TrimSpace(result.Text) == "" {
		t.Fatal("live provider returned no text")
	}
	t.Logf("live provider returned text (%d bytes)", len(result.Text))
}
