package compaction

import (
	"fmt"
	"strings"

	goai "github.com/rcarmo/go-ai"
)

const SummaryPrefix = "The conversation history before this point was compacted into the following summary:\n\n<summary>\n"
const SummarySuffix = "\n</summary>"

type Preparation struct {
	ContextTokens       int              `json:"context_tokens"`
	ThresholdTokens     int              `json:"threshold_tokens"`
	KeepRecentTokens    int              `json:"keep_recent_tokens"`
	ReserveTokens       int              `json:"reserve_tokens"`
	MessagesBefore      int              `json:"messages_before"`
	MessagesToSummarize int              `json:"messages_to_summarize"`
	RecentMessages      int              `json:"recent_messages"`
	Strategy            string           `json:"strategy"`
	Transcript          string           `json:"transcript"`
	RecentTranscript    string           `json:"recent_transcript"`
	Messages            []map[string]any `json:"messages"`
}

// Prepare describes a compaction with Pi's cut point (PlanCompaction): the
// first MessagesToSummarize messages (a previous summary included) are
// replaced; none when the recent-message budget covers everything.
func Prepare(messages []goai.Message, contextTokens, keepRecentTokens, reserveTokens, thresholdTokens int, strategy string) Preparation {
	keepStart := 0
	if plan, ok := PlanCompaction(messages, keepRecentTokens); ok {
		keepStart = plan.Split.FirstKept
	}
	prep := Preparation{ContextTokens: contextTokens, ThresholdTokens: thresholdTokens, KeepRecentTokens: keepRecentTokens, ReserveTokens: reserveTokens, MessagesBefore: len(messages), MessagesToSummarize: keepStart, RecentMessages: len(messages) - keepStart, Strategy: strategy}
	prep.Transcript = SerializeMessages(messages[:keepStart])
	prep.RecentTranscript = SerializeMessages(messages[keepStart:])
	prep.Messages = MessageDTOs(messages[:keepStart])
	return prep
}

func DefaultSummary(prep Preparation) string {
	transcript := strings.TrimSpace(prep.Transcript)
	if transcript == "" {
		return "Earlier conversation contained no text content."
	}
	const max = 12000
	if len(transcript) > max {
		transcript = transcript[len(transcript)-max:]
	}
	return fmt.Sprintf("Earlier conversation was compacted by gi's default heuristic. It contained %d messages and about %d tokens before compaction. Preserve user requirements, decisions, file paths, tests, and pending work from this transcript excerpt:\n\n%s", prep.MessagesToSummarize, prep.ContextTokens, transcript)
}

func EstimateMessagesTokens(messages []goai.Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateMessageTokens(msg)
	}
	return total
}

func EstimateMessageTokens(msg goai.Message) int {
	return EstimateTokens(goai.GetTextContent(&msg)) + 4
}

func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len([]rune(text))/4 + 1
}

func SerializeMessages(messages []goai.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		text := strings.TrimSpace(goai.GetTextContent(&msg))
		if text == "" {
			continue
		}
		fmt.Fprintf(&sb, "%s: %s\n\n", msg.Role, text)
	}
	return sb.String()
}

func MessageDTOs(messages []goai.Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		out = append(out, map[string]any{"role": string(msg.Role), "text": goai.GetTextContent(&msg), "tokens": EstimateMessageTokens(msg)})
	}
	return out
}
