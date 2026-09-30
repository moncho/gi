package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"
)

func TestThinkingWrapUsesPaddedWidthAndReflows(t *testing.T) {
	source := strings.Repeat("alpha beta gamma delta epsilon zeta eta theta ", 7)
	c := &chatTUI{outputWidth: 80}
	c.updateThinkingTranscript(source[:len(source)/2], time.Now())
	c.updateThinkingTranscript(source[len(source)/2:], time.Now())
	for _, width := range []int{80, 40, 28, 100, 40} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			var actual []string
			for _, row := range c.transcriptRowsAtWidth(width) {
				if strings.TrimSpace(row.text) != "" {
					actual = append(actual, row.text)
				}
			}
			var want []string
			for _, line := range renderMarkdownTranscript("", source, width-2) {
				want = append(want, " "+line)
			}
			if !reflect.DeepEqual(actual, want) {
				t.Fatalf("thinking double-wrapped or failed reflow at width %d:\ngot  %q\nwant %q", width, actual, want)
			}
			if c.outputWidth != 80 {
				t.Fatal("search/index rendering leaked temporary width")
			}
		})
	}
	c.finishThinkingTranscript(time.Now())
	blocks := c.buildTranscriptRenderableBlocks(c.transcript)
	if len(blocks) != 1 || blocks[0].MarkdownSource != source || blocks[0].Status != "done" {
		t.Fatalf("finished thinking lost source or status: %#v", blocks)
	}
}

func TestThinkingTableKeepsBothMarginsAfterResize(t *testing.T) {
	c := &chatTUI{outputWidth: 100}
	c.updateThinkingTranscript(tableRegression, time.Now())
	for _, width := range []int{38, 60, 100} {
		rows := c.transcriptRowsAtWidth(width)
		borders := 0
		for _, row := range rows {
			if strings.TrimSpace(row.text) == "" {
				continue
			}
			if !strings.HasPrefix(row.text, " ") || gotui.StringWidth(row.text) > width-1 {
				t.Fatalf("lost margins at width %d: %q", width, row.text)
			}
			line := strings.TrimSpace(row.text)
			if strings.HasPrefix(line, "┌") || strings.HasPrefix(line, "├") || strings.HasPrefix(line, "└") {
				borders++
				if !strings.ContainsRune("┐┤┘", []rune(line)[len([]rune(line))-1]) {
					t.Fatalf("clipped border at width %d: %q", width, line)
				}
			}
		}
		if borders < 3 {
			t.Fatalf("missing grid at width %d", width)
		}
	}
}
