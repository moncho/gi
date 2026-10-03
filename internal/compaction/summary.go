package compaction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	goai "github.com/rcarmo/go-ai"
)

// Pi's compaction (core/compaction/compaction.js, utils.js): the cut point
// keeps about keepRecentTokens of recent messages without separating a tool
// result from its call; the older messages are serialized and summarised by
// the session model with Pi's structured prompt, updating the previous
// summary when there is one; a turn split by the cut gets a separate prefix
// summary; files read and modified are appended.

// SummarizationSystemPrompt is Pi's SUMMARIZATION_SYSTEM_PROMPT.
const SummarizationSystemPrompt = `You are a context summarization assistant. Your task is to read a conversation between a user and an AI assistant, then produce a structured summary following the exact format specified.

Do NOT continue the conversation. Do NOT respond to any questions in the conversation. ONLY output the structured summary.`

const summarizationPrompt = `The messages above are a conversation to summarize. Create a structured context checkpoint summary that another LLM will use to continue the work.

Use this EXACT format:

## Goal
[What is the user trying to accomplish? Can be multiple items if the session covers different tasks.]

## Constraints & Preferences
- [Any constraints, preferences, or requirements mentioned by user]
- [Or "(none)" if none were mentioned]

## Progress
### Done
- [x] [Completed tasks/changes]

### In Progress
- [ ] [Current work]

### Blocked
- [Issues preventing progress, if any]

## Key Decisions
- **[Decision]**: [Brief rationale]

## Next Steps
1. [Ordered list of what should happen next]

## Critical Context
- [Any data, examples, or references needed to continue]
- [Or "(none)" if not applicable]

Keep each section concise. Preserve exact file paths, function names, and error messages.`

const updateSummarizationInstructions = `Update the existing structured summary with new information. RULES:
- PRESERVE all existing information from the previous summary
- ADD new progress, decisions, and context from the new messages
- UPDATE the Progress section: move items from "In Progress" to "Done" when completed
- UPDATE "Next Steps" based on what was accomplished
- PRESERVE exact file paths, function names, and error messages
- If something is no longer relevant, you may remove it

Use this EXACT format:

## Goal
[Preserve existing goals, add new ones if the task expanded]

## Constraints & Preferences
- [Preserve existing, add new ones discovered]

## Progress
### Done
- [x] [Include previously done items AND newly completed items]

### In Progress
- [ ] [Current work - update based on progress]

### Blocked
- [Current blockers - remove if resolved]

## Key Decisions
- **[Decision]**: [Brief rationale] (preserve all previous, add new)

## Next Steps
1. [Update based on current state]

## Critical Context
- [Preserve important context, add new if needed]

Keep each section concise. Preserve exact file paths, function names, and error messages.`

const updateSummarizationPrompt = "The messages above are NEW conversation messages to incorporate into the existing summary provided in <previous-summary> tags.\n\n" + updateSummarizationInstructions

const turnPrefixSummarizationPrompt = `The messages above are earlier context from an ongoing conversation. Later messages are stored separately and do not need to be reconstructed.

Create a concise checkpoint of the user's request and the progress shown above. This checkpoint will be placed before the later messages so the conversation can continue with the necessary context.

## Original Request
[What did the user ask for?]

## Progress So Far
- [Key decisions and work completed in these messages]

## Context Needed to Continue
- [Information from these messages needed to understand the later work]

Only summarize information explicitly present above. Do not infer or recreate later messages.`

// EstimatePiTokens is Pi's estimateTokens: characters / 4, counting text,
// thinking, tool-call names and arguments, and 4800 characters per image.
func EstimatePiTokens(msg goai.Message) int {
	chars := 0
	for _, b := range msg.Content {
		switch b.Type {
		case "text":
			chars += len([]rune(b.Text))
		case "thinking":
			if msg.Role == goai.RoleAssistant {
				chars += len([]rune(b.Thinking))
			}
		case "toolCall":
			if msg.Role == goai.RoleAssistant {
				chars += len([]rune(b.Name)) + len([]rune(jsonString(b.Arguments)))
			}
		case "image":
			if msg.Role != goai.RoleAssistant {
				chars += 4800
			}
		}
	}
	return (chars + 3) / 4
}

func jsonString(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if enc.Encode(v) != nil {
		return ""
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// CutPoint is Pi's findCutPoint over messages[start:]: walking back from the
// newest message until keepRecentTokens are kept, it cuts at a user or
// assistant message (never a tool result). A cut at an assistant message
// splits its turn; turnStart is then the turn's user message.
type CutPoint struct {
	FirstKept, TurnStart int
	SplitTurn            bool
}

func FindCutPoint(messages []goai.Message, start, keepRecentTokens int) CutPoint {
	var cuts []int
	for i := start; i < len(messages); i++ {
		if r := messages[i].Role; r == goai.RoleUser || r == goai.RoleAssistant {
			cuts = append(cuts, i)
		}
	}
	if len(cuts) == 0 {
		return CutPoint{FirstKept: start, TurnStart: -1}
	}
	cut := cuts[0]
	accumulated := 0
	for i := len(messages) - 1; i >= start; i-- {
		tokens := EstimatePiTokens(messages[i])
		if tokens == 0 {
			continue
		}
		accumulated += tokens
		if accumulated >= keepRecentTokens {
			cut = cuts[len(cuts)-1]
			for _, c := range cuts {
				if c >= i {
					cut = c
					break
				}
			}
			break
		}
	}
	if messages[cut].Role == goai.RoleUser {
		return CutPoint{FirstKept: cut, TurnStart: -1}
	}
	turnStart := -1
	for i := cut; i >= start; i-- {
		if messages[i].Role == goai.RoleUser {
			turnStart = i
			break
		}
	}
	return CutPoint{FirstKept: cut, TurnStart: turnStart, SplitTurn: turnStart != -1}
}

func contentText(msg goai.Message) string {
	var parts []string
	for _, b := range msg.Content {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func truncateForSummary(text string, max int) string {
	r := []rune(text)
	if len(r) <= max {
		return text
	}
	return fmt.Sprintf("%s\n\n[... %d more characters truncated]", string(r[:max]), len(r)-max)
}

// SerializeConversation is Pi's serializeConversation: role-tagged text so
// the model summarises instead of continuing; tool results are cut to 2000
// characters.
func SerializeConversation(messages []goai.Message) string {
	var parts []string
	for _, msg := range messages {
		switch msg.Role {
		case goai.RoleUser:
			if text := contentText(msg); text != "" {
				parts = append(parts, "[User]: "+text)
			}
		case goai.RoleAssistant:
			var thinking, calls []string
			hasText := false
			for _, b := range msg.Content {
				switch b.Type {
				case "thinking":
					thinking = append(thinking, b.Thinking)
				case "toolCall":
					keys := make([]string, 0, len(b.Arguments))
					for k := range b.Arguments {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					args := make([]string, 0, len(keys))
					for _, k := range keys {
						args = append(args, k+"="+jsonString(b.Arguments[k]))
					}
					calls = append(calls, b.Name+"("+strings.Join(args, ", ")+")")
				case "text":
					hasText = true
				}
			}
			if len(thinking) > 0 {
				parts = append(parts, "[Assistant thinking]: "+strings.Join(thinking, "\n"))
			}
			if hasText {
				parts = append(parts, "[Assistant]: "+contentText(msg))
			}
			if len(calls) > 0 {
				parts = append(parts, "[Assistant tool calls]: "+strings.Join(calls, "; "))
			}
		case goai.RoleToolResult:
			if text := contentText(msg); text != "" {
				parts = append(parts, "[Tool result]: "+truncateForSummary(text, 2000))
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// FileOps are the files a compacted range read and modified.
type FileOps struct {
	Read, Modified map[string]bool
}

func newFileOps() FileOps { return FileOps{Read: map[string]bool{}, Modified: map[string]bool{}} }

func (f FileOps) add(tool string, args map[string]any) {
	path, _ := args["path"].(string)
	if path == "" {
		return
	}
	switch tool {
	case "read":
		f.Read[path] = true
	case "write", "edit":
		f.Modified[path] = true
	}
}

func (f FileOps) collect(messages []goai.Message) {
	for _, m := range messages {
		if m.Role != goai.RoleAssistant {
			continue
		}
		for _, b := range m.Content {
			if b.Type == "toolCall" {
				f.add(b.Name, b.Arguments)
			}
		}
	}
}

var fileTagPattern = regexp.MustCompile(`(?s)<(read-files|modified-files)>\n(.*?)\n</(?:read-files|modified-files)>`)

// collectFromSummary recovers the file lists a previous summary carried
// (Pi keeps them in the compaction entry's details; gi in the summary text).
func (f FileOps) collectFromSummary(summary string) {
	for _, m := range fileTagPattern.FindAllStringSubmatch(summary, -1) {
		for _, path := range strings.Split(m[2], "\n") {
			if path = strings.TrimSpace(path); path == "" {
				continue
			}
			if m[1] == "read-files" {
				f.Read[path] = true
			} else {
				f.Modified[path] = true
			}
		}
	}
}

// Lists is Pi's computeFileLists: read-only files, then modified files.
func (f FileOps) Lists() (readFiles, modifiedFiles []string) {
	for p := range f.Modified {
		modifiedFiles = append(modifiedFiles, p)
	}
	for p := range f.Read {
		if !f.Modified[p] {
			readFiles = append(readFiles, p)
		}
	}
	sort.Strings(readFiles)
	sort.Strings(modifiedFiles)
	return readFiles, modifiedFiles
}

// FormatFileOperations is Pi's formatFileOperations.
func FormatFileOperations(readFiles, modifiedFiles []string) string {
	var sections []string
	if len(readFiles) > 0 {
		sections = append(sections, "<read-files>\n"+strings.Join(readFiles, "\n")+"\n</read-files>")
	}
	if len(modifiedFiles) > 0 {
		sections = append(sections, "<modified-files>\n"+strings.Join(modifiedFiles, "\n")+"\n</modified-files>")
	}
	if len(sections) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(sections, "\n\n")
}

// SummaryRequest is one summarization call.
type SummaryRequest struct {
	SystemPrompt, Prompt string
	MaxTokens            int
}

// SummaryResponse is the model's answer.
type SummaryResponse struct {
	Text         string
	Usage        *goai.Usage
	StopReason   goai.StopReason
	ErrorMessage string
	ToolCall     bool
}

// Summarizer calls the session model without tools.
type Summarizer func(context.Context, SummaryRequest) (SummaryResponse, error)

func summarizationFailure(resp SummaryResponse, label string) error {
	switch {
	case resp.StopReason == goai.StopReasonError:
		msg := resp.ErrorMessage
		if msg == "" {
			msg = "Unknown error"
		}
		return fmt.Errorf("%s failed: %s", label, msg)
	case resp.StopReason == goai.StopReasonLength:
		return fmt.Errorf("%s failed: generation hit the token cap and the summary is incomplete", label)
	case resp.ToolCall:
		return fmt.Errorf("%s attempted to call a tool", label)
	}
	return nil
}

func maxSummaryTokens(fraction float64, reserveTokens, modelMaxTokens int) int {
	n := int(fraction * float64(reserveTokens))
	if modelMaxTokens > 0 && modelMaxTokens < n {
		n = modelMaxTokens
	}
	return n
}

// GenerateSummary is Pi's generateSummaryWithUsage.
func GenerateSummary(ctx context.Context, summarize Summarizer, messages []goai.Message, reserveTokens, modelMaxTokens int, instructions, previousSummary string) (string, *goai.Usage, error) {
	base := summarizationPrompt
	if previousSummary != "" {
		base = updateSummarizationPrompt
	}
	if instructions != "" {
		base += "\n\nAdditional focus: " + instructions
	}
	prompt := "<conversation>\n" + SerializeConversation(messages) + "\n</conversation>\n\n"
	if previousSummary != "" {
		prompt += "<previous-summary>\n" + previousSummary + "\n</previous-summary>\n\n"
	}
	prompt += base
	resp, err := summarize(ctx, SummaryRequest{SystemPrompt: SummarizationSystemPrompt, Prompt: prompt, MaxTokens: maxSummaryTokens(0.8, reserveTokens, modelMaxTokens)})
	if err != nil {
		return "", nil, fmt.Errorf("Summarization failed: %w", err)
	}
	if err := summarizationFailure(resp, "Summarization"); err != nil {
		return "", resp.Usage, err
	}
	return resp.Text, resp.Usage, nil
}

// generateTurnPrefixSummary is Pi's generateTurnPrefixSummary.
func generateTurnPrefixSummary(ctx context.Context, summarize Summarizer, messages []goai.Message, reserveTokens, modelMaxTokens int) (string, *goai.Usage, error) {
	prompt := "# Conversation\n" + SerializeConversation(messages) + "\n\n# Instructions\n" + turnPrefixSummarizationPrompt
	resp, err := summarize(ctx, SummaryRequest{SystemPrompt: SummarizationSystemPrompt, Prompt: prompt, MaxTokens: maxSummaryTokens(0.5, reserveTokens, modelMaxTokens)})
	if err != nil {
		return "", nil, fmt.Errorf("Turn prefix summarization failed: %w", err)
	}
	if err := summarizationFailure(resp, "Turn prefix summarization"); err != nil {
		return "", resp.Usage, err
	}
	return resp.Text, resp.Usage, nil
}

// CombineUsage adds two usages (Pi's combineUsage).
func CombineUsage(a, b *goai.Usage) *goai.Usage {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := *a
	out.Input += b.Input
	out.Output += b.Output
	out.CacheRead += b.CacheRead
	out.CacheWrite += b.CacheWrite
	out.CacheWrite1h += b.CacheWrite1h
	out.Reasoning += b.Reasoning
	out.TotalTokens += b.TotalTokens
	out.Cost.Input += b.Cost.Input
	out.Cost.Output += b.Cost.Output
	out.Cost.CacheRead += b.Cost.CacheRead
	out.Cost.CacheWrite += b.Cost.CacheWrite
	out.Cost.Total += b.Cost.Total
	return &out
}

// Compact is Pi's compact for a prepared plan: the history summary (an
// update of the previous one), a turn-prefix summary for a split turn, and
// the file lists.
func Compact(ctx context.Context, summarize Summarizer, plan Plan, reserveTokens, modelMaxTokens int, instructions string) (string, *goai.Usage, error) {
	var summary string
	var usage *goai.Usage
	if plan.Split.SplitTurn && len(plan.TurnPrefix) > 0 {
		history := plan.PreviousSummary
		if history == "" {
			history = "No prior history."
		}
		if len(plan.History) > 0 {
			text, u, err := GenerateSummary(ctx, summarize, plan.History, reserveTokens, modelMaxTokens, instructions, plan.PreviousSummary)
			if err != nil {
				return "", u, err
			}
			history, usage = text, u
		}
		prefix, u, err := generateTurnPrefixSummary(ctx, summarize, plan.TurnPrefix, reserveTokens, modelMaxTokens)
		usage = CombineUsage(usage, u)
		if err != nil {
			return "", usage, err
		}
		summary = history + "\n\n---\n\n**Turn Context (split turn):**\n\n" + prefix
	} else {
		text, u, err := GenerateSummary(ctx, summarize, plan.History, reserveTokens, modelMaxTokens, instructions, plan.PreviousSummary)
		if err != nil {
			return "", u, err
		}
		summary, usage = text, u
	}
	readFiles, modifiedFiles := plan.FileOps.Lists()
	return summary + FormatFileOperations(readFiles, modifiedFiles), usage, nil
}

// Plan is Pi's prepareCompaction over gi's context messages: the previous
// summary (gi's leading summary message), the messages to summarise, the
// prefix of a split turn, and the files touched.
type Plan struct {
	PreviousSummary string
	History         []goai.Message
	TurnPrefix      []goai.Message
	Split           CutPoint
	FileOps         FileOps
}

// PreviousSummaryOf returns the summary in a leading compaction message.
func PreviousSummaryOf(messages []goai.Message) (string, bool) {
	if len(messages) == 0 || messages[0].Role != goai.RoleUser {
		return "", false
	}
	text := contentText(messages[0])
	if !strings.HasPrefix(text, SummaryPrefix) || !strings.HasSuffix(text, SummarySuffix) {
		return "", false
	}
	return text[len(SummaryPrefix) : len(text)-len(SummarySuffix)], true
}

// PlanCompaction prepares a compaction of messages keeping about
// keepRecentTokens; ok is false when there is nothing to summarise.
func PlanCompaction(messages []goai.Message, keepRecentTokens int) (Plan, bool) {
	start := 0
	plan := Plan{FileOps: newFileOps()}
	if prev, ok := PreviousSummaryOf(messages); ok {
		plan.PreviousSummary, start = prev, 1
		plan.FileOps.collectFromSummary(prev)
	}
	plan.Split = FindCutPoint(messages, start, keepRecentTokens)
	historyEnd := plan.Split.FirstKept
	if plan.Split.SplitTurn {
		historyEnd = plan.Split.TurnStart
		plan.TurnPrefix = messages[plan.Split.TurnStart:plan.Split.FirstKept]
	}
	plan.History = messages[start:historyEnd]
	if len(plan.History) == 0 && len(plan.TurnPrefix) == 0 {
		return plan, false
	}
	plan.FileOps.collect(plan.History)
	plan.FileOps.collect(plan.TurnPrefix)
	return plan, true
}
