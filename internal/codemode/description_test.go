package codemode

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type piDescriptionGolden struct {
	Tools       []Declaration     `json:"tools"`
	Full        string            `json:"full"`
	Budget      string            `json:"budget"`
	ScriptCalls map[string]string `json:"scriptCalls"`
}

// The description matches Pi's createCodemodeDescription byte for byte
// (golden produced by Pi's own code; models API off as in gi).
func TestDescriptionMatchesPi(t *testing.T) {
	raw, err := os.ReadFile("testdata/pi-description.json")
	if err != nil {
		t.Fatal(err)
	}
	var g piDescriptionGolden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	rendered, err := testEngine(t).RenderDeclarations(context.Background(), g.Tools)
	if err != nil {
		t.Fatal(err)
	}
	namespaces := map[string]Namespace{
		"mcp__hub__search_code": {Name: "mcp__hub", Description: "Code hosting"},
		"mcp__hub__upper":       {Name: "mcp__hub", Description: "Code hosting"},
		"mcp__docs__lookup":     {Name: "mcp__docs"},
	}
	if got := Description(g.Tools, rendered, DescriptionOptions{Namespaces: namespaces}); got != g.Full {
		t.Fatalf("full description differs:\n%s", diffAt(got, g.Full))
	}
	budget := 120
	got := Description(g.Tools, rendered, DescriptionOptions{Namespaces: namespaces, InlineBudget: &budget, Deferred: map[string]bool{"mcp__docs__lookup": true}})
	if got != g.Budget {
		t.Fatalf("budgeted description differs:\n%s", diffAt(got, g.Budget))
	}
	for _, d := range g.Tools {
		if got := ScriptCallDescription(d, rendered); got != g.ScriptCalls[d.Name] {
			t.Fatalf("%s: script call description %q, want %q", d.Name, got, g.ScriptCalls[d.Name])
		}
	}
}

func diffAt(got, want string) string {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	lo := max(0, i-80)
	return "got:  …" + got[lo:min(len(got), i+120)] + "\nwant: …" + want[lo:min(len(want), i+120)]
}

func TestParseSourceLikePi(t *testing.T) {
	code, opts, err := ParseSource("// @options: {\"max_output_tokens\": 2000, \"timeout_ms\": 30000}\nreturn 1;")
	if err != nil || code != "\nreturn 1;" || *opts.MaxOutputTokens != 2000 || *opts.TimeoutMs != 30000 {
		t.Fatalf("options: %q %+v %v", code, opts, err)
	}
	if code, _, err := ParseSource("return 2;"); err != nil || code != "return 2;" {
		t.Fatalf("plain: %q %v", code, err)
	}
	for input, want := range map[string]string{
		"   ":                               "Expected JavaScript source text (non-empty).",
		"// @options: {\"x\": 1}\nreturn 1": "@options only supports `max_output_tokens` and `timeout_ms`; got `x`",
		"// @options: {\"timeout_ms\": 0}\nreturn 1":  "@options field `timeout_ms` must be a positive integer up to 2147483647",
		"// @options: {\"max_output_tokens\": 1}\n  ": "The @options line must be followed by JavaScript source on subsequent lines",
		"// @options: [1]\nreturn 1":                  "@options must be a JSON object with supported fields",
	} {
		if _, _, err := ParseSource(input); err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Fatalf("%q: %v (want %q)", input, err, want)
		}
	}
}
