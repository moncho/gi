package turn

import (
	"context"
	"encoding/json"
	"log"
	"reflect"
	"sync"

	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

// The mcp_servers section follows Pi: its value when the session starts is
// part of the system prompt, and a later change is appended to the
// conversation once, as a system message updating the section, so earlier
// messages stay cached. gi records the first value and every update in the
// session state; an update is anchored to the user message whose turn
// introduced it and inserted before that message when requests are built.
// go-ai sends such messages as mid-conversation system messages or folds them
// into the system prompt for providers without support.

const mcpSectionStateKey = "mcp_servers_context"

type mcpSectionUpdate struct {
	Before string `json:"before"` // message ID the update precedes
	Text   string `json:"text"`   // empty: the section was removed
}

type mcpSectionRecord struct {
	Initial *string            `json:"initial,omitempty"`
	Updates []mcpSectionUpdate `json:"updates,omitempty"`
}

func (r mcpSectionRecord) latest() string {
	if n := len(r.Updates); n > 0 {
		return r.Updates[n-1].Text
	}
	if r.Initial != nil {
		return *r.Initial
	}
	return ""
}

// mcpSectionInsert is an update positioned in a turn's messages.
type mcpSectionInsert struct {
	index  int          // insert before convCtx.Messages[index]
	anchor goai.Message // message expected at index (detects mid-turn compaction)
	text   string
}

// mcpSectionPlan is the section layout for the running turn of a session.
type mcpSectionPlan struct {
	systemSection string // goes into the system prompt
	inserts       []mcpSectionInsert
}

var mcpSectionPlans sync.Map // sessionID -> *mcpSectionPlan

func renderMCPSection(text string) string {
	if text == "" {
		return ""
	}
	return "<mcp_servers>\n" + text + "\n</mcp_servers>"
}

// planMCPSection records a changed section for this turn and decides where
// every recorded value goes: the system prompt (the initial value, or the
// latest value whose anchor was compacted away) or before its anchor message.
// messageIDs are the snapshot messages, in convCtx order after offset.
func (e *Engine) planMCPSection(ctx context.Context, sessionID string, messageIDs []string, offset int, messages []goai.Message) *mcpSectionPlan {
	if e.mcp == nil {
		return nil
	}
	return e.planMCPSectionFor(ctx, sessionID, e.currentMCPSectionText(), messageIDs, offset, messages)
}

// currentMCPSectionText is the section body (without tags), or "".
func (e *Engine) currentMCPSectionText() string {
	section := e.mcpServersSection()
	if section == "" {
		return ""
	}
	return section[len("<mcp_servers>\n") : len(section)-len("\n</mcp_servers>")]
}

func (e *Engine) planMCPSectionFor(ctx context.Context, sessionID, current string, messageIDs []string, offset int, messages []goai.Message) *mcpSectionPlan {
	sess, err := e.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil
	}
	var rec mcpSectionRecord
	recorded := false
	if raw, ok := sess.State[mcpSectionStateKey]; ok {
		if b, err := json.Marshal(raw); err == nil && json.Unmarshal(b, &rec) == nil {
			recorded = true
		}
	}
	prompt := ""
	for i := len(messageIDs) - 1; i >= 0; i-- {
		if messageIDs[i] != "" {
			prompt = messageIDs[i]
			break
		}
	}
	changed := false
	switch {
	case !recorded && len(messageIDs) <= 1 && offset == 0:
		// A new session: the value at its start is part of the system prompt.
		rec.Initial, changed = &current, true
	case !recorded:
		// MCP appeared mid-session: append rather than change the prompt.
		empty := ""
		rec.Initial, changed = &empty, true
		if current != "" && prompt != "" {
			rec.Updates = append(rec.Updates, mcpSectionUpdate{Before: prompt, Text: current})
		}
	case current != rec.latest() && prompt != "":
		rec.Updates, changed = append(rec.Updates, mcpSectionUpdate{Before: prompt, Text: current}), true
	}
	if changed {
		if err := e.store.TouchSessionState(ctx, sessionID, map[string]any{mcpSectionStateKey: rec}); err != nil {
			log.Printf("mcp: record section: %v", err)
		}
	}
	plan := &mcpSectionPlan{}
	if rec.Initial != nil {
		plan.systemSection = *rec.Initial
	}
	index := map[string]int{}
	for i, id := range messageIDs {
		index[id] = i + offset
	}
	for _, u := range rec.Updates {
		if i, ok := index[u.Before]; ok && i < len(messages) {
			plan.inserts = append(plan.inserts, mcpSectionInsert{index: i, anchor: messages[i], text: u.Text})
		} else {
			// The anchor was compacted away: its value now belongs in the
			// system prompt (compaction rewrites the prefix anyway).
			plan.systemSection = u.Text
			plan.inserts = nil
		}
	}
	return plan
}

// withMCPSections inserts the plan's section updates into request messages.
// After a mid-turn compaction the anchors no longer match; the latest value
// then goes into the system prompt instead.
func (p *mcpSectionPlan) apply(systemPrompt string, messages []goai.Message) (string, []goai.Message) {
	if p == nil {
		return systemPrompt, messages
	}
	for _, ins := range p.inserts {
		if ins.index >= len(messages) || !reflect.DeepEqual(messages[ins.index], ins.anchor) {
			latest := p.inserts[len(p.inserts)-1].text
			return withSystemSection(systemPrompt, latest), messages
		}
	}
	out := make([]goai.Message, 0, len(messages)+len(p.inserts))
	next := 0
	for i, m := range messages {
		for next < len(p.inserts) && p.inserts[next].index == i {
			text := p.inserts[next].text
			var value *string
			if text != "" {
				value = &text
			}
			out = append(out, goai.Message{Role: goai.RoleSystem, Sections: map[string]*string{"mcp_servers": value}})
			next++
		}
		out = append(out, m)
	}
	return withSystemSection(systemPrompt, p.systemSection), out
}

func withSystemSection(systemPrompt, section string) string {
	if section == "" {
		return systemPrompt
	}
	return systemPrompt + "\n\n" + renderMCPSection(section)
}

// snapshotMessageIDs lists snapshot message IDs and the convCtx offset (1
// when a compaction summary precedes them).
func snapshotMessageIDs(snapshot store.ContextSnapshot) ([]string, int) {
	ids := make([]string, len(snapshot.Messages))
	for i, m := range snapshot.Messages {
		ids[i] = m.ID
	}
	offset := 0
	if snapshot.Summary != "" {
		offset = 1
	}
	return ids, offset
}
