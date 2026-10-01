package highlight

import (
	"strings"
	"testing"
)

func TestTokenizeContractAndNoPanic(t *testing.T) {
	type tc struct{ lang, code string }
	tests := map[string]tc{
		"go unterminated string":    {"go", `x := "abc`},
		"go unclosed block comment": {"go", "a /* unclosed\nstill"},
		"go unclosed raw string":    {"go", "`raw\nstill open"},
		"js unclosed template":      {"js", "`tmpl\nstill"},
		"bash unclosed brace var":   {"bash", "echo ${FOO"},
		"json unterminated string":  {"json", `{"k": "v`},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			lines := Tokenize(tt.lang, tt.code) // must not panic
			srcLines := strings.Split(tt.code, "\n")
			if len(lines) != len(srcLines) {
				t.Fatalf("line count = %d, want %d", len(lines), len(srcLines))
			}
			for i, toks := range lines {
				var b strings.Builder
				for _, tok := range toks {
					b.WriteString(tok.Text)
				}
				if b.String() != srcLines[i] {
					t.Errorf("line %d reconstructed %q, want %q", i, b.String(), srcLines[i])
				}
			}
		})
	}
}

func TestSplitLinesTrailingNewline(t *testing.T) {
	// "a\n" is two lines: "a" then an empty line.
	got := Tokenize("rust", "a\n")
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d: %#v", len(got), got)
	}
	if len(got[1]) != 0 {
		t.Errorf("trailing line should be empty, got %#v", got[1])
	}
}
