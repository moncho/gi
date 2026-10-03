package compaction

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	goai "github.com/rcarmo/go-ai"
)

// Prompts and summaries come from Pi's own generateBranchSummary with a
// stub model (scripts/golden-branch-summary.mjs).
func TestBranchSummaryMatchesPi(t *testing.T) {
	raw, err := os.ReadFile("testdata/pi-branch-summary.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Entries []struct {
			Type    string          `json:"type"`
			Summary string          `json:"summary"`
			Message json.RawMessage `json:"message"`
		} `json:"entries"`
		Cases []struct {
			Name                string `json:"name"`
			ContextWindow       int    `json:"contextWindow"`
			MaxTokens           int    `json:"maxTokens"`
			ReserveTokens       int    `json:"reserveTokens"`
			CustomInstructions  string `json:"customInstructions"`
			ReplaceInstructions bool   `json:"replaceInstructions"`
			Request             *struct {
				System, Prompt string
				MaxTokens      int `json:"maxTokens"`
			} `json:"request"`
			Summary string `json:"summary"`
		} `json:"cases"`
		Empty string `json:"empty"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	// The branch as gi's /tree passes it: tool results left out, summaries
	// as their context messages.
	var branch []BranchMessage
	for _, e := range g.Entries {
		switch e.Type {
		case "compaction":
			branch = append(branch, BranchMessage{Message: goai.UserMessage(SummaryPrefix + e.Summary + SummarySuffix), Summary: e.Summary})
		case "branch_summary":
			branch = append(branch, BranchMessage{Message: goai.UserMessage(BranchSummaryPrefix + e.Summary + BranchSummarySuffix), Summary: e.Summary, FromBranch: true})
		case "message":
			var m goai.Message
			if err := json.Unmarshal(e.Message, &m); err != nil {
				t.Fatal(err)
			}
			if m.Role != goai.RoleToolResult {
				branch = append(branch, BranchMessage{Message: m})
			}
		}
	}
	for _, c := range g.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var got *SummaryRequest
			stub := func(_ context.Context, req SummaryRequest) (SummaryResponse, error) {
				got = &req
				return SummaryResponse{Text: "SUMMARY", StopReason: goai.StopReasonStop}, nil
			}
			summary, _, err := GenerateBranchSummary(context.Background(), stub, branch, c.ContextWindow, c.ReserveTokens, c.MaxTokens, c.CustomInstructions, c.ReplaceInstructions)
			if err != nil {
				t.Fatal(err)
			}
			if summary != c.Summary {
				t.Fatalf("summary:\n got %q\nwant %q", summary, c.Summary)
			}
			switch {
			case c.Request == nil && got != nil:
				t.Fatalf("summarized; Pi did not")
			case c.Request == nil:
			case got == nil:
				t.Fatalf("did not summarize")
			case got.SystemPrompt != c.Request.System || got.MaxTokens != c.Request.MaxTokens:
				t.Fatalf("system/maxTokens: %q %d, want %q %d", got.SystemPrompt, got.MaxTokens, c.Request.System, c.Request.MaxTokens)
			case got.Prompt != c.Request.Prompt:
				t.Fatalf("prompt:\n got %q\nwant %q", got.Prompt, c.Request.Prompt)
			}
		})
	}
	if summary, _, _ := GenerateBranchSummary(context.Background(), nil, nil, 1000, 100, 100, "", false); summary != g.Empty {
		t.Fatalf("empty branch: %q, want %q", summary, g.Empty)
	}
}
