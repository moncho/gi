package tui

import (
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/store"
)

func TestPlainTerminalOutput(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"SGR and indentation", "  \x1b[1;31mred\x1b[0m  code\nnext", "  red  code\nnext"},
		{"cursor and erase", "one\x1b[2K\x1b[5Dtwo", "onetwo"},
		{"OSC title and hyperlink", "\x1b]0;bad title\x07\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\ after", "link after"},
		{"DCS and OSC with ST", "a\x1bPdata\x1b\\b\x1b]0;title\x1b\\c", "abc"},
		{"C1 sequences", "a\xc2\x9b31mred\xc2\x9b0m\xc2\x9d0;title\x07 ok", "ared ok"},
		{"bare C1 controls", "a\x9b31mred\x9b0m", "ared"},
		{"Unicode with control bytes in continuation", "中文🙂 €\x1b[31m red\x1b[0m", "中文🙂 € red"},
		{"Unicode tool lines", "TOOL-LINE-01 中文🙂\nTOOL-LINE-02 中文🙂", "TOOL-LINE-01 中文🙂\nTOOL-LINE-02 中文🙂"},
		{"bare controls", "a\x1b7b\x00\x07\x7fc\t indented\rnext", "abc\t indented\rnext"},
		{"broken OSC retains next line", "before\x1b]0;bad\nafter", "before\nafter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := plainTerminalOutput(tc.input); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestToolANSIOutputCollapsedExpandedAndStored(t *testing.T) {
	raw := "  \x1b[31mred\x1b[0m source\n\x1b]0;poison\x07second\nthird\nlast"
	c := &chatTUI{}
	c.renderToolEvent(map[string]any{"type": "tool_finished", "tool": "shell", "tool_call_id": "call", "output": raw}, time.Now())
	check := func(lines []string) {
		t.Helper()
		blocks := c.buildTranscriptRenderableBlocks(lines)
		if len(blocks) != 1 || blocks[0].Kind != "tool" {
			t.Fatalf("blocks: %+v", blocks)
		}
		block := blocks[0]
		if !block.Expandable || !strings.Contains(strings.Join(block.Body, "\n"), "  red source\nsecond\nthird\nlast") {
			t.Fatalf("body: %#v", block.Body)
		}
		for _, expanded := range []bool{false, true} {
			block.Expanded = expanded
			el := c.renderTranscriptBlock(block)
			buf := gotui.NewBuffer(80, 20)
			root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(80), gotui.WithHeight(20))
			root.AddChild(el)
			root.Render(buf, 80, 20)
			text := strings.ReplaceAll(buf.StringTrimmed(), "\u00a0", " ")
			if !strings.Contains(text, "red source") || strings.Contains(text, "poison") || strings.ContainsRune(text, '\x1b') {
				t.Fatalf("expanded=%v screen: %q", expanded, text)
			}
			if expanded && !strings.Contains(text, "last") || !expanded && strings.Contains(text, "last") {
				t.Fatalf("expanded=%v screen: %q", expanded, text)
			}
		}
	}
	check(c.transcript)
	// Historical transcripts can contain raw escape codes: sanitize at render too.
	stored := c.renderToolResultLines(store.Message{ID: "stored", Role: "tool_result", Content: raw, Payload: map[string]any{"tool_name": "shell"}})
	check(stored)
	legacy := []string{encodeTranscriptBlockMarker(transcriptBlockMeta{Key: "legacy", Kind: "tool", Title: "shell"}), "│ " + strings.Split(raw, "\n")[0], "│ second", "│ third", "│ last"}
	check(legacy)
	if got := c.renderMessageLine(store.Message{Role: "tool_result", Content: "\x1b[31mred\x1b[0m", Payload: map[string]any{"tool_name": "shell"}}); strings.ContainsRune(got, '\x1b') {
		t.Fatalf("summary: %q", got)
	}
}

func TestLocalBashOutputSanitizedBeforePreview(t *testing.T) {
	c := &chatTUI{}
	lines := c.bashBlockLines("printf", "\x1b[31m  red\x1b[0m\n\x1b]0;title\x07next", "ok", nil, time.Now(), time.Now())
	if got := strings.Join(lines, "\n"); strings.ContainsRune(got, '\x1b') || !strings.Contains(got, "│   red\n│ next") {
		t.Fatalf("bash block: %q", got)
	}
	if got := toolOutputBodyLines("\x1b[31mred\x1b[0m"); len(got) != 1 || got[0] != "red" {
		t.Fatalf("live output: %#v", got)
	}
}

func TestStoredSingleLineToolOutputPreservesSourceSpacing(t *testing.T) {
	c := &chatTUI{}
	lines := c.renderToolResultLines(store.Message{ID: "one", Role: "tool_result", Content: "    if ready {  return 42 }\x1b[0m", Payload: map[string]any{"tool_name": "shell"}})
	if len(lines) != 2 || lines[1] != "│     if ready {  return 42 }" {
		t.Fatalf("single-line source spacing: %#v", lines)
	}
	if got := toolOutputBodyLines("    if ready {  return 42 }\x1b[0m"); len(got) != 1 || got[0] != "    if ready {  return 42 }" {
		t.Fatalf("live source spacing: %#v", got)
	}
}
