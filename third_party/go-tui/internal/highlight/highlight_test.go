package highlight

import (
	"reflect"
	"testing"
)

func TestTokenizeFallbackAndSplit(t *testing.T) {
	type tc struct {
		lang string
		code string
		want [][]Token
	}
	tests := map[string]tc{
		"unknown lang is plain per line": {
			lang: "rust",
			code: "let x = 1\nfoo",
			want: [][]Token{
				{{KindPlain, "let x = 1"}},
				{{KindPlain, "foo"}},
			},
		},
		"empty code is one empty line": {
			lang: "go",
			code: "",
			want: [][]Token{{}},
		},
		"blank middle line preserved": {
			lang: "rust",
			code: "a\n\nb",
			want: [][]Token{
				{{KindPlain, "a"}},
				{},
				{{KindPlain, "b"}},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := Tokenize(tt.lang, tt.code)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tokenize(%q, %q) = %#v, want %#v", tt.lang, tt.code, got, tt.want)
			}
		})
	}
}

func TestSplitLinesMultilineToken(t *testing.T) {
	// A single token spanning two lines splits into same-kind pieces.
	in := []Token{{KindComment, "/* a\nb */"}, {KindPlain, " x"}}
	got := splitLines(in)
	want := [][]Token{
		{{KindComment, "/* a"}},
		{{KindComment, "b */"}, {KindPlain, " x"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitLines = %#v, want %#v", got, want)
	}
}
