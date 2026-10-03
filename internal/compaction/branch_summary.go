package compaction

import (
	"context"
	"errors"
	"fmt"

	goai "github.com/rcarmo/go-ai"
)

// Pi's branch summarization (core/compaction/branch-summarization.js): when
// /tree leaves a branch, the session model summarizes it so the context is
// not lost.

// BranchSummaryPrefix and BranchSummarySuffix wrap a branch summary in the
// model context (Pi's BRANCH_SUMMARY_PREFIX/SUFFIX).
const (
	BranchSummaryPrefix = "The following is a summary of a branch that this conversation came back from:\n\n<summary>\n"
	BranchSummarySuffix = "</summary>"
)

// DefaultBranchSummaryReserveTokens is Pi's branchSummary.reserveTokens.
const DefaultBranchSummaryReserveTokens = 16384

const branchSummaryPreamble = "The user explored a different conversation branch before returning here.\nSummary of that exploration:\n\n"

const branchSummaryPrompt = `Create a structured summary of this conversation branch for context when returning later.

Use this EXACT format:

## Goal
[What was the user trying to accomplish in this branch?]

## Constraints & Preferences
- [Any constraints, preferences, or requirements mentioned]
- [Or "(none)" if none were mentioned]

## Progress
### Done
- [x] [Completed tasks/changes]

### In Progress
- [ ] [Work that was started but not finished]

### Blocked
- [Issues preventing progress, if any]

## Key Decisions
- **[Decision]**: [Brief rationale]

## Next Steps
1. [What should happen next to continue this work]

Keep each section concise. Preserve exact file paths, function names, and error messages.`

// BranchMessage is one message of the branch being left, oldest first.
// Summary is the text of a compaction or branch summary (Pi estimates its
// size from the text alone and keeps it when it nearly fits); FromBranch
// marks a branch summary, whose file lists carry over.
type BranchMessage struct {
	Message    goai.Message
	Summary    string
	FromBranch bool
}

func (m BranchMessage) tokens() int {
	if m.Summary != "" {
		return (len([]rune(m.Summary)) + 3) / 4
	}
	return EstimatePiTokens(m.Message)
}

// prepareBranchMessages is Pi's prepareBranchEntries: the newest messages
// that fit the token budget (0: no limit) and the files they touched.
func prepareBranchMessages(branch []BranchMessage, tokenBudget int) ([]goai.Message, FileOps) {
	ops := newFileOps()
	for _, m := range branch {
		if m.FromBranch {
			ops.collectFromSummary(m.Summary)
		}
	}
	var messages []goai.Message
	total := 0
	for i := len(branch) - 1; i >= 0; i-- {
		m := branch[i]
		ops.collect([]goai.Message{m.Message})
		tokens := m.tokens()
		if tokenBudget > 0 && total+tokens > tokenBudget {
			if m.Summary != "" && float64(total) < float64(tokenBudget)*0.9 {
				messages = append([]goai.Message{m.Message}, messages...)
			}
			break
		}
		messages = append([]goai.Message{m.Message}, messages...)
		total += tokens
	}
	return messages, ops
}

// GenerateBranchSummary is Pi's generateBranchSummary. contextWindow 0 is
// Pi's 128000; instructions add a focus, or replace the prompt when replace
// is set.
func GenerateBranchSummary(ctx context.Context, summarize Summarizer, branch []BranchMessage, contextWindow, reserveTokens, modelMaxTokens int, instructions string, replace bool) (string, *goai.Usage, error) {
	if contextWindow <= 0 {
		contextWindow = 128000
	}
	messages, ops := prepareBranchMessages(branch, contextWindow-reserveTokens)
	if len(messages) == 0 {
		return "No content to summarize", nil, nil
	}
	prompt := branchSummaryPrompt
	switch {
	case replace && instructions != "":
		prompt = instructions
	case instructions != "":
		prompt += "\n\nAdditional focus: " + instructions
	}
	maxTokens := 4096
	if modelMaxTokens > 0 && modelMaxTokens < maxTokens {
		maxTokens = modelMaxTokens
	}
	resp, err := summarize(ctx, SummaryRequest{
		SystemPrompt: SummarizationSystemPrompt,
		Prompt:       "<conversation>\n" + SerializeConversation(messages) + "\n</conversation>\n\n" + prompt,
		MaxTokens:    maxTokens,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "", nil, err
		}
		return "", nil, fmt.Errorf("Branch summarization failed: %w", err)
	}
	if resp.StopReason == goai.StopReasonAborted {
		return "", resp.Usage, context.Canceled
	}
	if err := summarizationFailure(resp, "Branch summarization"); err != nil {
		return "", resp.Usage, err
	}
	readFiles, modifiedFiles := ops.Lists()
	return branchSummaryPreamble + resp.Text + FormatFileOperations(readFiles, modifiedFiles), resp.Usage, nil
}
