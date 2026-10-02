package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"

	"github.com/rcarmo/gi/internal/config"
)

type piRenderGolden struct {
	Wraps []struct {
		Text  string
		Width int
		Lines []string
	}
	Diffs []struct {
		Diff  string
		Lines [][]struct {
			Text    string
			Inverse bool
		}
	}
}

func loadPiRenderGolden(t *testing.T) piRenderGolden {
	t.Helper()
	raw, err := os.ReadFile("testdata/pi-file-tool-render.json")
	if err != nil {
		t.Fatal(err)
	}
	var g piRenderGolden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// Golden lines from pi-tui's wrapTextWithAnsi (scripts/golden-file-tool-render.mjs).
func TestPiWrapLineMatchesPiTUI(t *testing.T) {
	for _, c := range loadPiRenderGolden(t).Wraps {
		var got []string
		for _, line := range piWrapLine([]gotui.TextSpan{{Text: c.Text}}, c.Width) {
			var b strings.Builder
			for _, s := range line {
				b.WriteString(s.Text)
			}
			got = append(got, b.String())
		}
		if strings.Join(got, "|") != strings.Join(c.Lines, "|") {
			t.Fatalf("wrap %q at %d:\n got %q\nwant %q", c.Text, c.Width, got, c.Lines)
		}
	}
}

// Golden runs (text, inverse) from Pi's renderDiff.
func TestRenderDiffMatchesPi(t *testing.T) {
	for _, c := range loadPiRenderGolden(t).Diffs {
		lines := renderDiffLines(c.Diff)
		if len(lines) != len(c.Lines) {
			t.Fatalf("%q: %d lines, want %d", c.Diff, len(lines), len(c.Lines))
		}
		for i, line := range lines {
			type run struct {
				text    string
				inverse bool
			}
			var got []run
			for _, s := range line {
				inv := s.Style.HasAttr(gotui.AttrReverse)
				if s.Text == "" {
					continue
				}
				if n := len(got); n > 0 && got[n-1].inverse == inv {
					got[n-1].text += s.Text
				} else {
					got = append(got, run{s.Text, inv})
				}
			}
			var want []run
			for _, r := range c.Lines[i] {
				want = append(want, run{r.Text, r.Inverse})
			}
			if len(got) != len(want) {
				t.Fatalf("%q line %d: got %+v want %+v", c.Diff, i, got, want)
			}
			for j := range got {
				if got[j] != want[j] {
					t.Fatalf("%q line %d: got %+v want %+v", c.Diff, i, got, want)
				}
			}
		}
	}
}

func TestFileToolHeadersFollowPi(t *testing.T) {
	home, _ := os.UserHomeDir()
	root := t.TempDir()
	c := &chatTUI{cfg: config.RuntimeConfig{WorkspaceRoot: root}}
	plain := func(spans []gotui.TextSpan) string {
		var b strings.Builder
		for _, s := range spans {
			b.WriteString(s.Text)
		}
		return b.String()
	}
	meta := transcriptBlockMeta{Title: "read"}
	setFileToolArguments(&meta, map[string]any{"path": filepath.Join(home, "src/x.go"), "offset": float64(5), "limit": float64(10)})
	for _, c2 := range []struct {
		block transcriptRenderableBlock
		want  string
	}{
		{transcriptRenderableBlock{Header: "read", ToolPath: meta.ToolPath, ToolRange: meta.ToolRange}, "read ~/src/x.go:5-14"},
		{transcriptRenderableBlock{Header: "read", ToolPath: "a.go", ToolRange: ":7"}, "read a.go:7"},
		{transcriptRenderableBlock{Header: "write", ToolPath: "b.txt"}, "write b.txt"},
		{transcriptRenderableBlock{Header: "edit"}, "edit ..."},
		{transcriptRenderableBlock{Header: "read", ToolPath: "skills/deploy/SKILL.md"}, "[skill] deploy (ctrl+o to expand)"},
		{transcriptRenderableBlock{Header: "read", ToolPath: "sub/AGENTS.md", ToolRange: ":1-5"}, "read resource sub/AGENTS.md:1-5 (ctrl+o to expand)"},
		{transcriptRenderableBlock{Header: "read", ToolPath: "vfs://reference/tools/read.md"}, "read docs tools/read.md (ctrl+o to expand)"},
		{transcriptRenderableBlock{Header: "read", ToolPath: "sub/AGENTS.md", Expanded: true}, "read sub/AGENTS.md"},
	} {
		if got := plain(c.fileToolCallSpans(c2.block)); got != c2.want {
			t.Fatalf("got %q want %q", got, c2.want)
		}
	}
	if r := readLineRange(map[string]any{"limit": float64(20)}); r != ":1-20" {
		t.Fatalf("limit only: %q", r)
	}
}

func TestReadShowsPiTruncationNotice(t *testing.T) {
	c := &chatTUI{outputWidth: 80, transcriptExpanded: map[string]bool{"r": true}}
	meta := transcriptBlockMeta{Key: "r", Kind: "tool", Title: "read", Status: "ok", ToolPath: "big.txt"}
	setFileToolDetails(&meta, map[string]any{"truncation": map[string]any{"truncated": true, "truncatedBy": "lines", "outputLines": float64(2000), "totalLines": float64(3000), "maxLines": float64(2000)}}, "")
	_, screen, _ := renderToolForTest(t, c, meta, []string{"line 1"}, 80)
	if !strings.Contains(screen, "[Truncated: showing 2000 of 3000 lines (2000 line limit)]") {
		t.Fatalf("notice missing: %s", screen)
	}
	meta.Key = "collapsed"
	_, screen, _ = renderToolForTest(t, c, meta, []string{"line 1"}, 80)
	if strings.Contains(screen, "Truncated") || strings.Contains(screen, "line 1") {
		t.Fatalf("collapsed read shows its result: %s", screen)
	}
	if n := readTruncationNotice(map[string]any{"truncation": map[string]any{"truncated": true, "firstLineExceedsLimit": true, "maxBytes": float64(51200)}}); n != "[First line exceeds 50.0KB limit]" {
		t.Fatal(n)
	}
	if n := readTruncationNotice(map[string]any{"truncation": map[string]any{"truncated": true, "truncatedBy": "bytes", "outputLines": float64(247)}}); n != "[Truncated: 247 lines shown (50.0KB limit)]" {
		t.Fatal(n)
	}
}

func TestWritePreviewHintAndErrorFollowPi(t *testing.T) {
	content := strings.Repeat("line\n", 14)
	c := &chatTUI{outputWidth: 80, transcriptExpanded: map[string]bool{}}
	meta := transcriptBlockMeta{Key: "w", Kind: "tool", Title: "write", Status: "error", ToolPath: "notes.txt", ToolContent: &content}
	buf, screen, _ := renderToolForTest(t, c, meta, []string{"EACCES: permission denied"}, 80)
	if !strings.Contains(screen, "... (4 more lines, 14 total, ctrl+o to expand)") {
		t.Fatalf("hint: %s", screen)
	}
	assertTextColour(t, buf, "EACCES", piError)
	assertTextColour(t, buf, "line", piToolOutput)
}

func assertTextColour(t *testing.T, buf *gotui.Buffer, word string, want gotui.Color) {
	t.Helper()
	for y := 0; y < buf.Height(); y++ {
		for x := 0; x <= buf.Width()-len(word); x++ {
			var text strings.Builder
			for i := 0; i < len(word); i++ {
				text.WriteString(string(buf.Cell(x+i, y).Rune))
			}
			if text.String() == word {
				if buf.Cell(x, y).Style.Fg != want {
					t.Fatalf("%s: colour %v, want %v", word, buf.Cell(x, y).Style.Fg, want)
				}
				return
			}
		}
	}
	t.Fatalf("%s absent", word)
}

func TestEditShowsPiDiffPreviewAndErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.go"), []byte("package f\n\nvar x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &chatTUI{outputWidth: 60, cfg: config.RuntimeConfig{WorkspaceRoot: root}, transcriptExpanded: map[string]bool{}}
	args := map[string]any{"path": "f.go", "edits": []any{map[string]any{"oldText": "var x = 1", "newText": "var x = 2"}}}
	c.renderToolEvent(map[string]any{"type": "tool_started", "tool": "edit", "turn_id": "t", "tool_call_id": "a", "arguments": args}, time.Now())
	meta, _ := parseTranscriptBlockMarker(c.transcript[0])
	if meta.EditDiff == nil || *meta.EditDiff != " 1 package f\n 2 \n-3 var x = 1\n+3 var x = 2" {
		t.Fatalf("preview: %+v", meta)
	}
	buf, screen, _ := renderToolForTest(t, c, meta, nil, 60)
	if !strings.Contains(screen, "edit f.go") || !strings.Contains(screen, "-3 var x = 1") || !strings.Contains(screen, "+3 var x = 2") {
		t.Fatalf("diff: %s", screen)
	}
	// Pending with a diff preview already has the success background (Pi).
	if bg := buf.Cell(0, 1).Style.Bg; bg != piToolSuccessBg {
		t.Fatalf("band %v", bg)
	}

	// A preview error shows in the box; the same result error is not repeated.
	bad := map[string]any{"path": "f.go", "edits": []any{map[string]any{"oldText": "nope", "newText": "y"}}}
	c.renderToolEvent(map[string]any{"type": "tool_started", "tool": "edit", "turn_id": "t", "tool_call_id": "b", "arguments": bad}, time.Now())
	want := "Could not find the exact text in f.go. The old text must match exactly including all whitespace and newlines."
	c.renderToolEvent(map[string]any{"type": "tool_failed", "tool": "edit", "turn_id": "t", "tool_call_id": "b", "output": want, "error": want}, time.Now())
	span := c.transcriptBlockSpans[c.transcriptToolBlocks[c.toolRuntimeBlockKey(map[string]any{"turn_id": "t", "tool_call_id": "b"}, "edit")]]
	meta, _ = parseTranscriptBlockMarker(c.transcript[span.HeaderIndex])
	buf, screen, _ = renderToolForTest(t, c, meta, c.readTranscriptBlockBody(meta.Key), 60)
	if strings.Count(screen, "Could not find") != 1 {
		t.Fatalf("error shown %d times: %s", strings.Count(screen, "Could not find"), screen)
	}
	if bg := buf.Cell(0, 1).Style.Bg; bg != piToolErrorBg {
		t.Fatalf("band %v", bg)
	}

	// A different result error goes below the box, without its background.
	meta.EditError = ""
	other := "Operation aborted"
	meta.EditDiff = &[]string{" 1 a"}[0]
	buf, screen, height := renderToolForTest(t, c, meta, []string{other}, 60)
	if !strings.Contains(screen, other) {
		t.Fatalf("result error missing: %s", screen)
	}
	for y := 0; y < height; y++ {
		if strings.Contains(rowText(buf, y), other) && buf.Cell(1, y).Style.Bg == piToolSuccessBg {
			t.Fatal("result error inside the box")
		}
	}
}

func rowText(buf *gotui.Buffer, y int) string {
	var b strings.Builder
	for x := 0; x < buf.Width(); x++ {
		b.WriteRune(buf.Cell(x, y).Rune)
	}
	return b.String()
}

func TestResumedEditUsesStoredDiff(t *testing.T) {
	c := &chatTUI{outputWidth: 60, transcriptExpanded: map[string]bool{}}
	meta := transcriptBlockMeta{Kind: "tool", Title: "edit", Status: "ok"}
	setFileToolArguments(&meta, map[string]any{"path": "f.go"})
	setFileToolDetails(&meta, map[string]any{"diff": "-1 a\n+1 b"}, "Successfully replaced 1 block(s) in f.go.")
	if meta.EditDiff == nil || *meta.EditDiff != "-1 a\n+1 b" || meta.ToolPath != "f.go" {
		t.Fatalf("%+v", meta)
	}
	meta.Key = "e"
	_, screen, _ := renderToolForTest(t, c, meta, []string{"Successfully replaced 1 block(s) in f.go."}, 60)
	if strings.Contains(screen, "Successfully") || !strings.Contains(screen, "+1 b") {
		t.Fatalf("%s", screen)
	}
}
