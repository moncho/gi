package tools

import (
	"math"
	"reflect"
	"testing"
)

// Golden tokens and scores come from Pi's own tool-search module.
func TestToolSearchMatchesPi(t *testing.T) {
	if got := Tokenize("searchGitHubIssues for the HTTPServer classes and boxes"); !reflect.DeepEqual(got, []string{"search", "git", "hub", "issue", "http", "server", "class", "box"}) {
		t.Fatalf("tokens %v", got)
	}
	gh := &SearchNamespace{Name: "mcp__github", Description: "GitHub API"}
	docs := []SearchDocument{
		NewSearchDocument("mcp__github__search_issues", "Search issues and pull requests",
			map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "GitHub search syntax"}}}, gh),
		NewSearchDocument("mcp__github__get_file", "Read a file from a repository",
			map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}}, gh),
		NewSearchDocument("mcp__docs__search", "Search the product documentation", map[string]any{}, &SearchNamespace{Name: "mcp__docs", Description: "Docs"}),
	}
	want := []SearchMatch{{"mcp__github__search_issues", 2.901572157723193}, {"mcp__docs__search", 0.7882572077932966}, {"mcp__github__get_file", 0.7385771316718702}}
	got := RankBM25("search github issues", docs, 8)
	if len(got) != len(want) {
		t.Fatalf("matches %v", got)
	}
	for i := range want {
		if got[i].Name != want[i].Name || math.Abs(got[i].Score-want[i].Score) > 1e-9 {
			t.Fatalf("match %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if one := RankBM25("read file", docs, 1); len(one) != 1 || one[0].Name != "mcp__github__get_file" || math.Abs(one[0].Score-2.522132364887296) > 1e-9 {
		t.Fatalf("limit 1: %+v", one)
	}
	if RankBM25("the and of", docs, 8) != nil {
		t.Fatal("stop-word query must match nothing")
	}
}
