package compaction

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	goai "github.com/rcarmo/go-ai"
)

type piCompactionGolden struct {
	Messages   []goai.Message
	Serialized string
	Estimates  []int
	Cases      []struct {
		Name, Instructions, PreviousSummary, Summary string
		Split                                         bool
		ReserveTokens                                 int
		Prompts                                       []struct {
			System, Prompt string
			MaxTokens      int
		}
	}
}

// Prompts, serialization, estimates and the merged summary come from Pi's
// own compaction code with a stub model (scripts/golden-compaction.mjs).
func TestCompactionMatchesPi(t *testing.T) {
	raw, err := os.ReadFile("testdata/pi-compaction.json")
	if err != nil {
		t.Fatal(err)
	}
	var g piCompactionGolden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	if got := SerializeConversation(g.Messages); got != g.Serialized {
		t.Fatalf("serialized:\n got %q\nwant %q", got, g.Serialized)
	}
	for i, m := range g.Messages {
		if got := EstimatePiTokens(m); got != g.Estimates[i] {
			t.Fatalf("estimate %d: %d, want %d", i, got, g.Estimates[i])
		}
	}
	history, prefix := g.Messages[:6], g.Messages[6:]
	files := map[string]FileOps{
		"initial": {Read: map[string]bool{"src/parse.go": true}, Modified: map[string]bool{"src/parse.go": true}},
		"update":  {Read: map[string]bool{"README.md": true}, Modified: map[string]bool{}},
		"split":   {Read: map[string]bool{}, Modified: map[string]bool{"src/parse_test.go": true}},
	}
	for _, c := range g.Cases {
		var prompts []SummaryRequest
		stub := func(_ context.Context, req SummaryRequest) (SummaryResponse, error) {
			prompts = append(prompts, req)
			return SummaryResponse{Text: "SUMMARY " + string(rune('0'+len(prompts))), StopReason: goai.StopReasonStop, Usage: &goai.Usage{Input: 10, Output: 5}}, nil
		}
		plan := Plan{PreviousSummary: c.PreviousSummary, History: history, FileOps: files[c.Name]}
		if c.Split {
			plan.TurnPrefix, plan.Split = prefix, CutPoint{SplitTurn: true}
		}
		summary, usage, err := Compact(context.Background(), stub, plan, c.ReserveTokens, 4000, c.Instructions)
		if err != nil {
			t.Fatal(err)
		}
		if summary != c.Summary {
			t.Fatalf("%s summary:\n got %q\nwant %q", c.Name, summary, c.Summary)
		}
		if len(prompts) != len(c.Prompts) {
			t.Fatalf("%s: %d prompts, want %d", c.Name, len(prompts), len(c.Prompts))
		}
		for i, p := range c.Prompts {
			if prompts[i].SystemPrompt != p.System || prompts[i].Prompt != p.Prompt || prompts[i].MaxTokens != p.MaxTokens {
				t.Fatalf("%s prompt %d:\n got %q (%d)\nwant %q (%d)", c.Name, i, prompts[i].Prompt, prompts[i].MaxTokens, p.Prompt, p.MaxTokens)
			}
		}
		if want := 15 * len(prompts); usage == nil || usage.Input+usage.Output != want {
			t.Fatalf("%s usage %+v", c.Name, usage)
		}
	}
}

// The cut keeps recent messages, never separates a tool result from its
// call, and splits a turn cut at an assistant message (Pi's findCutPoint).
func TestPlanCompactionCutsLikePi(t *testing.T) {
	big := func(role goai.Role, n int) goai.Message {
		text := make([]byte, n*4)
		for i := range text {
			text[i] = 'x'
		}
		if role == goai.RoleToolResult {
			return goai.Message{Role: role, Content: []goai.ContentBlock{{Type: "text", Text: string(text)}}}
		}
		if role == goai.RoleAssistant {
			return goai.Message{Role: role, Content: []goai.ContentBlock{{Type: "text", Text: string(text)}}}
		}
		return goai.UserMessage(string(text))
	}
	prev := goai.UserMessage(SummaryPrefix + "earlier\n\n<read-files>\nold.go\n</read-files>" + SummarySuffix)
	msgs := []goai.Message{prev, big(goai.RoleUser, 100), big(goai.RoleAssistant, 100), big(goai.RoleUser, 100), big(goai.RoleAssistant, 300), big(goai.RoleToolResult, 500), big(goai.RoleAssistant, 100)}
	plan, ok := PlanCompaction(msgs, 600)
	if !ok || plan.PreviousSummary != "earlier\n\n<read-files>\nold.go\n</read-files>" {
		t.Fatalf("plan %+v", plan)
	}
	// Walking back: 100 + 500 (tool result, not a cut point) reaches 600 at
	// index 5; the nearest cut at or after it is the assistant at 6.
	if plan.Split.FirstKept != 6 || !plan.Split.SplitTurn || plan.Split.TurnStart != 3 || len(plan.History) != 2 || len(plan.TurnPrefix) != 3 {
		t.Fatalf("cut %+v history %d prefix %d", plan.Split, len(plan.History), len(plan.TurnPrefix))
	}
	if read, _ := plan.FileOps.Lists(); len(read) != 1 || read[0] != "old.go" {
		t.Fatalf("file lists from the previous summary: %v", read)
	}
	if _, ok := PlanCompaction(msgs[:3], 100000); ok {
		t.Fatal("nothing to summarise when everything fits")
	}
}
