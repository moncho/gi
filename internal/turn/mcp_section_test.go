package turn

import (
	"context"
	"testing"

	goai "github.com/rcarmo/go-ai"
)

func sectionMessages(n int) []goai.Message {
	out := make([]goai.Message, n)
	for i := range out {
		out[i] = goai.UserMessage(string(rune('a' + i)))
	}
	return out
}

// The section's value at session start is in the system prompt; a change is
// inserted once before the prompt that introduced it and stays there; a
// compacted-away anchor folds the value into the system prompt (Pi).
func TestMCPSectionPlacementLikePi(t *testing.T) {
	s := openTestStore(t)
	e := New(s)
	defer e.Close()
	ctx := context.Background()
	if _, err := s.CreateSession(ctx, "sec", "Sections", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	plan := func(current string, ids []string, offset int) (string, []goai.Message) {
		t.Helper()
		msgs := sectionMessages(len(ids) + offset)
		p := e.planMCPSectionFor(ctx, "sec", current, ids, offset, msgs)
		return p.apply("SYS", msgs)
	}
	sys, msgs := plan("A", []string{"m1"}, 0)
	if sys != "SYS\n\n<mcp_servers>\nA\n</mcp_servers>" || len(msgs) != 1 {
		t.Fatalf("new session: %q %d", sys, len(msgs))
	}
	if sys, msgs = plan("A", []string{"m1", "m2", "m3"}, 0); len(msgs) != 3 || sys != "SYS\n\n<mcp_servers>\nA\n</mcp_servers>" {
		t.Fatal("unchanged section must not add messages or change the prompt")
	}
	sys, msgs = plan("B", []string{"m1", "m2", "m3"}, 0)
	if sys != "SYS\n\n<mcp_servers>\nA\n</mcp_servers>" || len(msgs) != 4 || msgs[2].Role != goai.RoleSystem || *msgs[2].Sections["mcp_servers"] != "B" {
		t.Fatalf("change: prompt %q messages %+v", sys, msgs)
	}
	// Next turn: the update keeps its position, so the prefix stays cached.
	if _, msgs = plan("B", []string{"m1", "m2", "m3", "m4", "m5"}, 0); len(msgs) != 6 || msgs[2].Role != goai.RoleSystem {
		t.Fatalf("update moved: %+v", msgs)
	}
	// Removal is a nil section value.
	if _, msgs = plan("", []string{"m1", "m2", "m3", "m4", "m5"}, 0); len(msgs) != 7 || msgs[5].Role != goai.RoleSystem || msgs[5].Sections["mcp_servers"] != nil {
		t.Fatalf("removal: %+v", msgs)
	}
	// After compaction removed m1..m3 (summary at index 0), the B update folds
	// into the system prompt; the later removal keeps its place before m5.
	sys, msgs = plan("", []string{"m4", "m5"}, 1)
	if sys != "SYS\n\n<mcp_servers>\nB\n</mcp_servers>" || len(msgs) != 4 || msgs[2].Role != goai.RoleSystem {
		t.Fatalf("compaction: %q %+v", sys, msgs)
	}
	// A mid-turn compaction (anchors no longer match) moves the latest value
	// into the system prompt instead of inserting at stale positions.
	p := e.planMCPSectionFor(ctx, "sec", "", []string{"m4", "m5"}, 1, sectionMessages(3))
	sys, msgs = p.apply("SYS", []goai.Message{goai.UserMessage("summary-only")})
	if sys != "SYS" || len(msgs) != 1 {
		t.Fatalf("mid-turn compaction: %q %d", sys, len(msgs))
	}
	// MCP enabled mid-session: appended, not added to the system prompt.
	if _, err := s.CreateSession(ctx, "late", "Late", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	lp := e.planMCPSectionFor(ctx, "late", "C", []string{"x1", "x2", "x3"}, 0, sectionMessages(3))
	sys, msgs = lp.apply("SYS", sectionMessages(3))
	if sys != "SYS" || len(msgs) != 4 || *msgs[2].Sections["mcp_servers"] != "C" {
		t.Fatalf("mid-session: %q %+v", sys, msgs)
	}
}
