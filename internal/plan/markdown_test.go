package plan

import (
	"strings"
	"testing"
)

func str(v string) *string { return &v }
func idx(v int) *int       { return &v }
func TestPlanNormalizeParseRender(t *testing.T) {
	source := "> note\r\n> continued\r\n## Heading\r\n- [X] done\r\n1. [-] doing\r\n* [ ] pending"
	normalized, err := Normalize(source)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(normalized, "\r") || !strings.Contains(normalized, "- [x] done") {
		t.Fatal(normalized)
	}
	parsed := Parse(normalized)
	if parsed.Explanation == nil || *parsed.Explanation != "note\ncontinued" || len(parsed.Plan) != 3 {
		t.Fatal(parsed)
	}
	if parsed.Plan[0].Status != "completed" || parsed.Plan[1].Status != "in_progress" || parsed.Plan[2].Status != "pending" {
		t.Fatal(parsed)
	}
	rendered, err := Render(parsed.Explanation, parsed.Plan)
	if err != nil || !strings.HasPrefix(rendered, "> note\n> continued\n\n- [x] done") {
		t.Fatal(rendered, err)
	}
	for _, bad := range []string{"- [-] one\n- [-] two", strings.Repeat("x", MaxBytes+1)} {
		if _, err = Normalize(bad); err == nil {
			t.Fatal("invalid accepted")
		}
	}
	if _, err = Render(nil, []Item{{"one", "in_progress"}, {"two", "in_progress"}}); err == nil {
		t.Fatal("multiple active accepted")
	}
	if _, err = Render(nil, []Item{{" ", "pending"}}); err == nil {
		t.Fatal("empty step accepted")
	}
	if _, err = Render(nil, []Item{{"step", "other"}}); err == nil {
		t.Fatal("unknown status accepted")
	}
}
func TestPlanEditsUseOriginalAnchorsAndRejectOverlap(t *testing.T) {
	source := "## Heading\n- [ ] alpha\n- [ ] beta"
	next, err := EditMarkdown(source, []Edit{{OldText: str("alpha"), NewText: str("first")}, {Operation: "insert_after", AnchorText: str("beta"), Text: str("\n- [x] third")}})
	if err != nil || next != "## Heading\n- [ ] first\n- [ ] beta\n- [x] third" {
		t.Fatal(next, err)
	}
	next, err = EditMarkdown("a", []Edit{{Operation: "append", Text: str("b")}, {Operation: "append", Text: str("c")}, {Operation: "prepend", Text: str("before")}})
	if err != nil || next != "beforeabc" {
		t.Fatal(next, err)
	}
	for _, edits := range [][]Edit{
		{}, {{OldText: str("missing"), NewText: str("x")}},
		{{OldText: str("a"), NewText: str("x")}},
		{{OldText: str("alpha"), NewText: str("x")}, {OldText: str("alph"), NewText: str("y")}},
		{{Operation: "replace", OldText: str("alpha")}},
		{{Operation: "other", Text: str("x")}},
	} {
		if _, err = EditMarkdown(source, edits); err == nil {
			t.Fatal("invalid edits accepted", edits)
		}
	}
	next, err = EditMarkdown(source, []Edit{{Operation: "delete", OldText: str("\n- [ ] beta")}})
	if err != nil || strings.Contains(next, "beta") {
		t.Fatal(next, err)
	}
}
func TestPlanPatchesSequentialAndUnique(t *testing.T) {
	source := "> note\n\n## Heading\n- [ ] alpha\n- [-] beta"
	next, err := PatchMarkdown(source, []Patch{{Operation: "update", Match: "beta", Status: str("completed")}, {Operation: "add", Before: "alpha", Step: str(" first  task "), Status: str("in_progress")}, {Operation: "remove", Index: idx(2)}})
	if err != nil || next != "> note\n\n- [-] first task\n- [x] beta" {
		t.Fatal(next, err)
	}
	for _, patches := range [][]Patch{
		{}, {{Operation: "update", Index: idx(0)}}, {{Operation: "remove", Match: "a"}},
		{{Operation: "update", Match: "missing"}}, {{Operation: "update", Match: "alpha", Status: str("in_progress")}},
		{{Operation: "add", Step: str(" ")}}, {{Operation: "add", Step: str("x"), Before: "missing"}},
		{{Operation: "add", Step: str("x"), Position: "invalid"}}, {{Operation: "update", Index: idx(1), Step: str("")}},
	} {
		if _, err = PatchMarkdown(source, patches); err == nil {
			t.Fatal("invalid patches accepted", patches)
		}
	}
	empty, err := Apply(source, Mutation{Action: "update", Plan: []Item{}})
	if err != nil || empty != "" {
		t.Fatal(empty, err)
	}
	if _, err = Apply(source, Mutation{Action: "write"}); err == nil {
		t.Fatal("write without Markdown")
	}
	if _, err = Apply(source, Mutation{Action: "update"}); err == nil {
		t.Fatal("update without array")
	}
}
