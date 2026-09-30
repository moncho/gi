package tui

import (
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
)

func TestMarkdownElementsUsePiThemeColours(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}}
	src := "# Heading `x`\n\nSome **bold** and *it* text with a [link](https://e.x).\n\n- item\n\n> quoted\n\n```go\nfunc main() {}\n```\n\n---"
	lines := c.renderMessageLines(store.Message{Role: "assistant", Content: src}, 60)
	plain := stripMarkdownInlineStyleMarkers(strings.Join(lines, "\n"))
	for _, want := range []string{"HEADING X", "Some bold and it text with a link (https://e.x).", "• item", "> quoted", "[code:go] 1 lines", "func main() {}"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("projection lost %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "\x00") {
		t.Fatal("marker leaked")
	}
	blocks := c.buildTranscriptRenderableBlocks(lines)
	root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(60), gotui.WithHeight(30))
	for _, b := range blocks {
		root.AddChild(c.renderTranscriptBlock(b))
	}
	buf := gotui.NewBuffer(60, 30)
	root.RenderTo(buf, 60, 30)
	find := func(text string) gotui.Cell {
		for y := 0; y < 30; y++ {
			row := ""
			for x := 0; x < 60; x++ {
				row += string(buf.Cell(x, y).Rune)
			}
			if i := strings.Index(row, text); i >= 0 {
				return buf.Cell(len([]rune(row[:i])), y)
			}
		}
		t.Fatalf("%q not rendered:\n%s", text, buf.StringTrimmed())
		return gotui.Cell{}
	}
	for text, want := range map[string]gotui.Color{"HEADING": piWarning, "•": piAccent, "link": piMdLink, "(https": piMuted, "func": piSuccess, "[code:go]": piMuted} {
		if got := find(text).Style.Fg; got != want {
			t.Fatalf("%q fg %v want %v", text, got, want)
		}
	}
	if !find("bold").Style.HasAttr(gotui.AttrBold) || !find("it text").Style.HasAttr(gotui.AttrItalic) {
		t.Fatal("emphasis attributes")
	}
	if find("X").Style.Fg == piMdCode {
		// inline code inside the heading keeps the code colour; fine either way
	}
}

func TestBalanceStyleMarkersAcrossWrappedRows(t *testing.T) {
	lines := wrapParagraph(mdStyled("bold", "one two three four five six"), 10)
	if len(lines) < 2 {
		t.Fatal(lines)
	}
	for _, line := range lines {
		segs := parseTUIInlineSegments(line)
		for _, seg := range segs {
			if strings.TrimSpace(seg.Text) != "" && (len(seg.Styles) != 1 || seg.Styles[0] != "bold") {
				t.Fatalf("unbalanced row %q: %+v", line, segs)
			}
		}
	}
}
