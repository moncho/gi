package tui

import (
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
)

func TestInlineCodeKeepsSentenceAndSpacesAcrossStyling(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}}
	for _, tc := range []struct {
		markdown, plain string
	}{
		{"Before `mono` after.", "Gi: Before mono after."},
		{"Before `mono`, after.", "Gi: Before mono, after."},
		{"A`mono`B", "Gi: AmonoB"},
		{"Use `  --flag  ` now", "Gi: Use  --flag  now"},
	} {
		lines := c.renderMessageLines(store.Message{Role: "assistant", Content: tc.markdown}, 50)
		if got := stripMarkdownInlineStyleMarkers(strings.Join(lines, "\n")); got != tc.plain {
			t.Fatalf("projection of %q: %q, want %q", tc.markdown, got, tc.plain)
		}
		blocks := c.buildTranscriptRenderableBlocks(lines)
		el := c.renderTranscriptBlock(blocks[0])
		root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(50), gotui.WithHeight(5))
		root.AddChild(el)
		buf := gotui.NewBuffer(50, 5)
		root.Render(buf, 50, 5)
		// The terminal displays non-breaking spaces like ordinary spaces. They
		// prevent go-tui's word wrapper from eating code's boundary spaces.
		screen := strings.ReplaceAll(buf.StringTrimmed(), "\u00a0", " ")
		visible := strings.TrimPrefix(tc.plain, "Gi: ")
		if !strings.Contains(screen, visible) || strings.Contains(screen, "Gi: ") {
			t.Fatalf("ANSI-styled rendering of %q broke the sentence or showed a speaker label: %q", tc.markdown, screen)
		}
		if tc.markdown == "Before `mono` after." {
			// Styling belongs to the code cells, not the text after them.
			if code, after := buf.Cell(1+len("Before "), 1), buf.Cell(1+len("Before mono "), 1); code.Style.Fg != gotui.BrightBlack || after.Style.Fg == gotui.BrightBlack {
				t.Fatalf("inline code style bled into following text: code=%+v after=%+v", code, after)
			}
		}
	}
}

func TestUserPromptMarkdownRenderedImmediatelyAndAfterReload(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}}
	text := "# My input\n\n- **first** `item`\n- second item"
	c.appendUserPrompt(text, false)
	if got := c.renderMessageLines(store.Message{Role: "user", Content: text}, c.transcriptRenderWidth()); !equalTranscriptLines(got, c.transcript) {
		t.Fatalf("live prompt differs from stored message: live=%q stored=%q", c.transcript, got)
	}
	c.appendUserPrompt(text, true)
	blocks := c.buildTranscriptRenderableBlocks(c.transcript)
	if len(blocks) != 2 || blocks[0].Kind != "user" || blocks[1].Kind != "user" || len(blocks[1].Body) < 3 {
		t.Fatalf("live and queued user Markdown escaped their message band: %+v", blocks)
	}
	for _, block := range blocks {
		plain := stripMarkdownInlineStyleMarkers(strings.Join(append([]string{block.Header}, block.Body...), "\n"))
		for _, want := range []string{"MY INPUT", "• first item", "• second item"} {
			if !strings.Contains(plain, want) {
				t.Fatalf("Markdown input missing %q in %q", want, plain)
			}
		}
		if strings.Contains(plain, "**first**") || strings.Contains(plain, "`item`") {
			t.Fatalf("raw Markdown leaked into user transcript: %q", plain)
		}
	}
}

func equalTranscriptLines(a, b []string) bool {
	return strings.Join(a, "\n") == strings.Join(b, "\n")
}
