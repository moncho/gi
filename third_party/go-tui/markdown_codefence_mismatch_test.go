package tui

import (
	"testing"

	"github.com/grindlemire/go-tui/internal/markdown"
)

type fakeHighlighter struct{ lines [][]TextSpan }

func (f fakeHighlighter) Highlight(lang, code string) [][]TextSpan { return f.lines }

func TestRenderCodeFenceMismatchedLineCount(t *testing.T) {
	th := DefaultMarkdownTheme()
	th.CodeHighlighter = fakeHighlighter{lines: [][]TextSpan{
		{{Text: "one"}}, // only one line of spans...
	}}
	m := NewMarkdown(WithMarkdownSource(""), WithMarkdownTheme(th))
	block := markdown.Block{Kind: markdown.KindCodeFence, Lang: "go", Lines: []string{"one", "two", "three"}}
	box := m.renderCodeFence(block) // must not panic
	inner := box.children[0]
	if len(inner.children) != 3 {
		t.Fatalf("want 3 line elements, got %d", len(inner.children))
	}
	// Line 0 highlighted (rich text), lines 1-2 fall back to plain text.
	if len(inner.children[0].RichText()) == 0 {
		t.Error("line 0 should have rich-text spans")
	}
	if len(inner.children[2].RichText()) != 0 {
		t.Error("line 2 should fall back to plain text")
	}
}
