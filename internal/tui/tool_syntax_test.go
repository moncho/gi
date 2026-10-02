package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/store"
)

func TestFileToolLanguage(t *testing.T) {
	for p, want := range map[string]string{"src/main.go": "go", "src/App.TSX": "typescript", "C:\\src\\Makefile": "makefile", "/tmp/Dockerfile": "dockerfile", "config.yml": "yaml", "report.unknown": "", "notes.txt": "", "src/go/main": ""} {
		if got := fileToolLanguage(p); got != want {
			t.Errorf("%s: %q != %q", p, got, want)
		}
	}
}

func TestFileToolLiveArgumentsAndWritePreview(t *testing.T) {
	source := "package main\n\nfunc main() {\n\tprintln(\"**literal** 👩🏽‍💻\")\n}\n" + strings.Repeat("// preview line\n", 8) + "// tail sentinel"
	for _, tool := range []string{"read", "write"} {
		c := &chatTUI{outputWidth: 80}
		args := map[string]any{"path": "src/main.go", "content": source}
		c.renderToolEvent(map[string]any{"type": "tool_started", "tool": tool, "turn_id": "turn", "tool_call_id": "call", "arguments": args}, time.Now())
		output := source
		if tool == "write" {
			output = "Wrote file successfully"
		}
		// Completion lacking arguments must retain the start's source metadata.
		c.renderToolEvent(map[string]any{"type": "tool_finished", "tool": tool, "turn_id": "turn", "tool_call_id": "call", "output": output}, time.Now())
		meta, ok := parseTranscriptBlockMarker(c.transcript[0])
		if !ok || meta.ToolPath != "src/main.go" {
			t.Fatalf("meta: %+v", meta)
		}
		if tool == "write" && (meta.ToolContent == nil || *meta.ToolContent != source) {
			t.Fatal("lost write source")
		}
		for _, expanded := range []bool{false, true} {
			c.transcriptExpanded = map[string]bool{meta.Key: expanded}
			buf, screen, _ := renderToolForTest(t, c, meta, c.readTranscriptBlockBody(meta.Key), 80)
			visible := tool == "write" || expanded
			if strings.Contains(screen, "package main") != visible {
				t.Fatalf("%s expanded=%v: %s", tool, expanded, screen)
			}
			if strings.Contains(screen, "tail sentinel") != expanded {
				t.Fatalf("wrong preview: %s", screen)
			}
			if visible && (!strings.Contains(screen, "   println") || !strings.Contains(screen, "**literal**")) {
				t.Fatalf("source changed: %s", screen)
			}
			if strings.Contains(screen, "Wrote file successfully") {
				t.Fatal("highlighting acknowledgment instead of source")
			}
			if visible {
				assertKeywordColour(t, buf, "package")
			}
		}
	}
}

func assertKeywordColour(t *testing.T, buf *gotui.Buffer, word string) {
	t.Helper()
	for y := 0; y < buf.Height(); y++ {
		for x := 0; x <= buf.Width()-len(word); x++ {
			var text strings.Builder
			for i := 0; i < len(word); i++ {
				text.WriteString(string(buf.Cell(x+i, y).Rune))
			}
			if text.String() == word {
				if buf.Cell(x, y).Style.Fg != piSyntaxKeyword {
					t.Fatalf("keyword not highlighted: %v", buf.Cell(x, y).Style.Fg)
				}
				return
			}
		}
	}
	t.Fatalf("keyword %s absent", word)
}

func TestFileToolHistoryArgumentsTurnScoped(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.CreateSession(ctx, "s", "test", nil); err != nil {
		t.Fatal(err)
	}
	for i, tool := range []string{"read", "write"} {
		turnID := fmt.Sprint(i)
		source := "package main\n" + strings.Repeat(" ", 220) + "// end sentinel"
		args := map[string]any{"file_path": tool + ".go", "content": source}
		calls := map[string]any{"kind": "tool_calls", "turn_id": turnID, "tool_calls": []map[string]any{{"id": "reused", "name": tool, "arguments": args}}}
		if err = s.AddMessage(ctx, "call"+turnID, "s", "assistant", "call", calls); err != nil {
			t.Fatal(err)
		}
		output := source
		if tool == "write" {
			output = "success"
		}
		if err = s.AddMessage(ctx, "result"+turnID, "s", "tool_result", output, map[string]any{"turn_id": turnID, "tool_call_id": "reused", "tool_name": tool}); err != nil {
			t.Fatal(err)
		}
	}
	c := &chatTUI{store: s, sessionID: "s", outputWidth: 80}
	blocks := c.buildTranscriptRenderableBlocks(c.loadTranscript())
	if len(blocks) != 2 {
		t.Fatalf("blocks: %+v", blocks)
	}
	for _, block := range blocks {
		if block.ToolPath != block.Header+".go" {
			t.Fatalf("path lost: %+v", block)
		}
		if block.Header == "write" && (block.ToolContent == nil || !strings.HasSuffix(*block.ToolContent, "// end sentinel")) {
			t.Fatal("write content lost")
		}
		if block.Header == "read" && !strings.HasSuffix(block.Body[1], "// end sentinel") {
			t.Fatal("read truncated at 200 characters")
		}
	}
}

func TestFileToolMultilineAndResize(t *testing.T) {
	source := "package main\n/* comment\ncontinues */\nvar message = `raw\ncontinued`\n\tprintln(\"日本語 👩🏽‍💻\")"
	segments, _ := fileToolSegments(source, "main.go", true)
	if segments[2][0].class != "syn-comment" || segments[4][0].class != "syn-string" {
		t.Fatalf("lost multiline state: %+v", segments)
	}
	for _, width := range []int{80, 24, 38, 80} {
		c := &chatTUI{outputWidth: width, transcriptExpanded: map[string]bool{"r": true}}
		meta := transcriptBlockMeta{Key: "r", Kind: "tool", Title: "read", Status: "ok", ToolPath: "main.go"}
		buf, screen, _ := renderToolForTest(t, c, meta, strings.Split(source, "\n"), width)
		assertKeywordColour(t, buf, "package")
		if !strings.Contains(screen, "👩🏽‍💻") || strings.Contains(screen, "\t") {
			t.Fatalf("width %d: %s", width, screen)
		}
	}
}

func TestFileToolErrorsUnknownExtensionsAndModes(t *testing.T) {
	for _, tool := range []string{"read", "write"} {
		c := &chatTUI{outputWidth: 80}
		meta := transcriptBlockMeta{Key: "e", Kind: "tool", Title: tool, Status: "error", ToolPath: "main.go"}
		_, screen, _ := renderToolForTest(t, c, meta, []string{"permission denied"}, 80)
		if !strings.Contains(screen, "permission denied") {
			t.Fatalf("hidden error: %s", screen)
		}
	}
	for _, mode := range []string{"", "hidden", "compact"} {
		c := &chatTUI{outputWidth: 80, transcriptExpanded: map[string]bool{"u": true}, extensionToolModes: map[string]string{"read": mode}}
		meta := transcriptBlockMeta{Key: "u", Kind: "tool", Title: "read", Status: "ok", ToolPath: "notes.unknown"}
		_, screen, _ := renderToolForTest(t, c, meta, []string{"**literal**", "second line"}, 80)
		if strings.Contains(screen, "**literal**") != (mode != "hidden") || strings.Contains(screen, "second line") != (mode == "") {
			t.Fatalf("mode %q: %s", mode, screen)
		}
	}
}

func TestFileToolSearchAndCopyUseDisplayedSource(t *testing.T) {
	source := "package main\n\t\"**literal**\"\n" + strings.Repeat("x", 60) + "suffix"
	c := &chatTUI{outputWidth: 80, transcriptExpanded: map[string]bool{"w": true}}
	c.appendTranscriptBlock(transcriptBlockMeta{Key: "w", Kind: "tool", Title: "write", ToolPath: "main.go", ToolContent: &source, Status: "ok"}, []string{"success acknowledgment"})
	rows := c.renderedTranscriptRows(24) // search/selection reproject at their width, not stale outputWidth
	var visible strings.Builder
	for _, row := range rows {
		visible.WriteString(row.text)
		visible.WriteByte('\n')
	}
	if !strings.Contains(visible.String(), "**literal**") || !strings.Contains(visible.String(), "suffix") || strings.Contains(visible.String(), "success acknowledgment") || strings.ContainsRune(visible.String(), '\x00') {
		t.Fatalf("wrong searchable source: %s", visible.String())
	}
	for i, row := range rows {
		if start := strings.Index(row.text, "**literal**"); start >= 0 {
			selection := transcriptSelection{active: true, moved: true, rows: rows, width: 24, start: transcriptPoint{row: i, col: start}, end: transcriptPoint{row: i, col: start + 11}}
			if got := selection.text(); got != "**literal**" {
				t.Fatalf("copy changed literal source: %q", got)
			}
			return
		}
	}
	t.Fatal("literal row missing")
}

func TestFileToolImpossibleWidthCluster(t *testing.T) {
	root := gotui.New(gotui.WithWidth(1), gotui.WithHeight(1))
	root.AddChild(fileToolRows([]synSegment{{text: "🙂"}}, 1, piFg(piMuted))[0])
	buf := gotui.NewBuffer(1, 1)
	root.RenderTo(buf, 1, 1)
	if got := buf.StringTrimmed(); got != "\uFFFD" {
		t.Fatalf("too-wide cluster: %q", got)
	}
}

func TestFileToolPendingAndFailedWrite(t *testing.T) {
	c := &chatTUI{outputWidth: 80}
	source := "package written\n" + strings.Repeat("// preview\n", 11)
	meta := transcriptBlockMeta{Key: "pending", Kind: "tool", Title: "write", ToolPath: "main.go", ToolContent: &source, Status: "running"}
	c.appendTranscriptBlock(meta, nil)
	blocks := c.buildTranscriptRenderableBlocks(c.transcript)
	if len(blocks) != 1 || !blocks[0].Expandable {
		t.Fatal("pending write not expandable")
	}
	for _, row := range c.renderedTranscriptRows(80) {
		if strings.Contains(row.text, "package written") && row.blockKey != "pending" {
			t.Fatal("pending content lost hit ownership")
		}
	}
	meta.Status = "error"
	_, screen, _ := renderToolForTest(t, c, meta, []string{"permission denied"}, 80)
	if !strings.Contains(screen, "package written") || !strings.Contains(screen, "permission denied") {
		t.Fatalf("failed write lost source or error: %s", screen)
	}
}
