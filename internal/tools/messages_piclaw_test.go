package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

// Golden values from Piclaw's own helpers: bun scripts/golden-messages-search.mjs
func TestPiclawMessagesSearchHelpersMatchGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/piclaw-messages-search.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Content  string
		Fallback []struct {
			Query    string
			Operator bool
			Terms    []string
		}
		SearchTerms []struct {
			Query string
			Terms []string
		}
		Excerpts []struct {
			Query     string
			Width     int
			Text      *string
			Truncated *bool
		}
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	for _, g := range golden.Fallback {
		if got := store.IsSearchOperatorQuery(g.Query); got != g.Operator {
			t.Errorf("operator(%q)=%v want %v", g.Query, got, g.Operator)
		}
		if got := store.SearchFallbackTerms(g.Query, g.Operator); !sameTerms(got, g.Terms) {
			t.Errorf("fallback(%q)=%q want %q", g.Query, got, g.Terms)
		}
	}
	for _, g := range golden.SearchTerms {
		if got := piclawSearchTerms(g.Query); !sameTerms(got, g.Terms) {
			t.Errorf("terms(%q)=%q want %q", g.Query, got, g.Terms)
		}
	}
	for _, g := range golden.Excerpts {
		text, truncated, ok := piclawExcerpt(golden.Content, piclawSearchTerms(g.Query), g.Width)
		if ok != (g.Text != nil) || ok && (text != *g.Text || truncated != *g.Truncated) {
			t.Errorf("excerpt(%q,%d)=%q,%v,%v want %v,%v", g.Query, g.Width, text, truncated, ok, g.Text, g.Truncated)
		}
	}
}

func sameTerms(a, b []string) bool { return len(a) == 0 && len(b) == 0 || reflect.DeepEqual(a, b) }

func TestPiclawMessagesSearchAndGet(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	add := func(id, session, role, content string, payload map[string]any) {
		t.Helper()
		if err := s.AddMessage(ctx, id, session, role, content, payload); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"A", "B"} {
		if _, err := s.CreateSession(ctx, id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	add("u1", "A", "user", "u1 zeta #topic", nil)
	add("r1", "A", "assistant", "r1-zeta", nil)
	add("c1", "A", "assistant", "[tool_call: messages {\"query\":\"zeta\"}]", map[string]any{"kind": "tool_calls"})
	add("t1", "A", "tool_result", "zeta tool output", map[string]any{"kind": "tool_result"})
	add("f1", "B", "user", "foreign zeta", nil)
	add("u2", "A", "user", "u2 zeta beta", nil)
	add("r2", "A", "assistant", "r2-zeta", nil)
	add("u3", "A", "user", "u3 zeta", nil)
	add("r3", "A", "assistant", "", nil)
	rowOf := map[string]int64{}
	for _, session := range []string{"A", "B"} {
		all, err := s.RetrieveMessages(ctx, session, store.MessageRetrievalQuery{Limit: 100, ContentBytes: 100})
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range all.Messages {
			rowOf[row.ID] = row.RowID
		}
	}
	if len(rowOf) != 9 {
		t.Fatal(rowOf)
	}
	var details map[string]any
	rt := ToolRuntime{Store: s, SessionID: "A", SetDetails: func(d map[string]any) { details = d }}
	call := func(args string) string {
		t.Helper()
		var m map[string]any
		if err := json.Unmarshal([]byte(args), &m); err != nil {
			t.Fatal(err)
		}
		out, err := ExecuteMessages(ctx, rt, goai.ToolCall{Arguments: m})
		if err != nil {
			t.Fatalf("%s: %v", args, err)
		}
		return out
	}
	rowsIn := func(text string) []int64 {
		var ids []int64
		for _, m := range regexp.MustCompile(`\[(\d+)\]`).FindAllStringSubmatch(text, -1) {
			id, _ := strconv.ParseInt(m[1], 10, 64)
			ids = append(ids, id)
		}
		return ids
	}
	line := func(id, author, text string) string { return fmt.Sprintf("[%d] %s: %s", rowOf[id], author, text) }

	// Current session only, newest first, conversation view (no tool rows).
	want := strings.Join([]string{"Found 5 messages.", line("u3", "user", "u3 zeta"), line("r2", "assistant", "r2-zeta"), line("u2", "user", "u2 zeta beta"), line("r1", "assistant", "r1-zeta"), line("u1", "user", "u1 zeta #topic")}, "\n")
	if got := call(`{"action":"search","query":"zeta","limit":20}`); got != want {
		t.Fatalf("search:\n%s\nwant:\n%s", got, want)
	}
	if details["count"] != 5 || details["chat_jid"] != "gi:A" {
		t.Fatal(details)
	}
	// Default action is search. Operator queries need every term (operator
	// words dropped); plain queries match any term (Piclaw's "or" mode).
	if got := call(`{"query":"zeta AND beta"}`); got != "Found 1 message.\n"+line("u2", "user", "u2 zeta beta") {
		t.Fatal(got)
	}
	if got := call(`{"query":"beta r1-zeta"}`); !reflect.DeepEqual(rowsIn(got), []int64{rowOf["u2"], rowOf["r1"]}) {
		t.Fatal(got)
	}
	if got := call(`{"query":"beta foreign","chat_jid":"*"}`); !reflect.DeepEqual(rowsIn(got), []int64{rowOf["u2"], rowOf["f1"]}) {
		t.Fatal(got)
	}
	if got := call(`{"action":"search","query":"#topic"}`); got != "Found 1 message.\n"+line("u1", "user", "u1 zeta #topic") {
		t.Fatal(got)
	}
	got := call(fmt.Sprintf(`{"action":"search","query":"zeta","after_row":%d,"limit":2}`, rowOf["u2"]))
	if ids := rowsIn(got); !strings.HasPrefix(got, "Found 2 messages.") || !reflect.DeepEqual(ids, []int64{rowOf["u3"], rowOf["r2"]}) {
		t.Fatal(got)
	}
	if got := call(`{"action":"search","query":"zeta","limit":2,"offset":4}`); got != "Found 1 message.\n"+line("u1", "user", "u1 zeta #topic") {
		t.Fatal(got)
	}
	if got := call(`{"action":"search","query":"zeta","role":"assistant"}`); !reflect.DeepEqual(rowsIn(got), []int64{rowOf["r2"], rowOf["r1"]}) {
		t.Fatal(got)
	}
	// Piclaw single-user scope: "*"/"all" and explicit chats reach other sessions.
	for _, scope := range []string{"*", "all", "gi:B", "B"} {
		if got := call(fmt.Sprintf(`{"action":"search","query":"foreign","chat_jid":%q}`, scope)); got != "Found 1 message.\n"+line("f1", "user", "foreign zeta") {
			t.Fatalf("%s: %s", scope, got)
		}
	}
	if got := call(`{"action":"search","query":"foreign"}`); got != "No matching messages found." {
		t.Fatal(got)
	}
	if got := call(`{"action":"search"}`); got != "Provide query for action=search." {
		t.Fatal(got)
	}
	if got := call(`{"action":"search","query":"beta","excerpt_chars":8}`); got != "Found 1 message.\n"+line("u2", "user", "…eta [[beta]]") {
		t.Fatal(got)
	}

	// get: context in row order within the anchor's session; missing IDs omitted.
	got = call(fmt.Sprintf(`{"action":"get","row_ids":[%d,%d,2000000000],"context_before":1,"context_after":1}`, rowOf["u2"], rowOf["u3"]))
	want = strings.Join([]string{
		"- " + line("u2", "user", "u2 zeta beta"), "  before:", "  " + line("r1", "assistant", "r1-zeta"), "  after:", "  " + line("r2", "assistant", "r2-zeta"), "",
		"- " + line("u3", "user", "u3 zeta"), "  before:", "  " + line("r2", "assistant", "r2-zeta"), "  after:", "  " + line("r3", "assistant", "[empty message]"),
	}, "\n")
	if got != want {
		t.Fatalf("get:\n%s\nwant:\n%s", got, want)
	}
	if !reflect.DeepEqual(details["missing_row_ids"], []int64{2000000000}) {
		t.Fatal(details)
	}
	// As in Piclaw, get without chat_jid selects any session; chat_jid restricts it.
	if got := call(fmt.Sprintf(`{"action":"get","row_ids":[%d]}`, rowOf["f1"])); got != "- "+line("f1", "user", "foreign zeta") {
		t.Fatal(got)
	}
	if got := call(fmt.Sprintf(`{"action":"get","row_ids":[%d],"chat_jid":"gi:A"}`, rowOf["f1"])); got != "No messages found for requested row IDs." {
		t.Fatal(got)
	}
	// Tool-result and tool-call-only rows are not chat posts.
	if got := call(fmt.Sprintf(`{"action":"get","row_ids":[%d,%d]}`, rowOf["c1"], rowOf["t1"])); got != "No messages found for requested row IDs." {
		t.Fatal(got)
	}
	if got := call(`{"action":"get"}`); got != "Provide row_ids for action=get." {
		t.Fatal(got)
	}
}
