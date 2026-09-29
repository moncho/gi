package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestToolOutputPreviewWindow(t *testing.T) {
	if ToolOutputPreview("") != nil {
		t.Fatal("empty output")
	}
	if p := ToolOutputPreview("  \r\n  "); p["output_preview"] != "  \n  " || p["output_total_lines"] != 2 || p["output_preview_lines"] != 2 || p["output_truncated"] != false {
		t.Fatalf("whitespace must remain visible: %v", p)
	}
	if p := ToolOutputPreview("  first  \r\nsecond\r\n"); p["output_preview"] != "  first  \nsecond\n" || p["output_total_lines"] != 3 || p["output_preview_lines"] != 3 {
		t.Fatalf("surrounding whitespace and trailing newline: %v", p)
	}
	if p := ToolOutputPreview("first\n"); p["output_preview"] != "first\n" || p["output_preview_lines"] != 2 || p["output_total_lines"] != 2 || p["output_truncated"] != false {
		t.Fatalf("trailing blank line: %v", p)
	}
	var lines []string
	for i := 0; i < 120; i++ {
		lines = append(lines, fmt.Sprintf("line-%03d", i))
	}
	p := ToolOutputPreview(strings.Join(lines, "\r\n"))
	if !strings.HasPrefix(p["output_preview"].(string), "line-020") || p["output_total_lines"] != 120 || p["output_preview_lines"] != 100 || p["output_truncated"] != true {
		t.Fatal(p)
	}
	p = ToolOutputPreview(strings.Repeat("β🙂", 4000))
	text := p["output_preview"].(string)
	if !utf8.ValidString(text) || p["output_truncated"] != true {
		t.Fatal("invalid byte window")
	}
	// Classic slices bytes before decoding. The decoded string can exceed
	// 12 KiB when a partial leading rune is replaced by U+FFFD.
	p = ToolOutputPreview(strings.Repeat("β", 6145) + "x")
	text = p["output_preview"].(string)
	if len(text) != 12*1024+2 || !strings.HasPrefix(text, "�") || !strings.HasSuffix(text, "x") || p["output_truncated"] != true {
		t.Fatalf("UTF-8 cutoff: bytes=%d prefix=%q payload=%v", len(text), []rune(text)[:min(5, len([]rune(text)))], p["output_truncated"])
	}
	for _, tc := range []struct {
		text, prefix string
		bytes        int
	}{
		{strings.Repeat("€", 4097) + "x", "��€", 12*1024 + 4},
		{strings.Repeat("🙂", 3073) + "x", "���🙂", 12*1024 + 6},
	} {
		p = ToolOutputPreview(tc.text)
		text = p["output_preview"].(string)
		if len(text) != tc.bytes || !strings.HasPrefix(text, tc.prefix) || !strings.HasSuffix(text, "x") || !utf8.ValidString(text) {
			t.Fatalf("multibyte cutoff: bytes=%d prefix=%q", len(text), []rune(text)[:min(5, len([]rune(text)))])
		}
	}
}

func TestToolOutputSnapshotOccurrenceAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, _ = s.CreateSession(ctx, "a", "a", nil)
	_, _ = s.CreateTurn(ctx, "t", "a", "prompt", nil)
	for _, occ := range []string{"old", "new"} {
		if err = s.AppendTurnEvent(ctx, "t", "a", "tool.started", map[string]any{"tool": "shell", "tool_call_id": "reused", "occurrence_id": occ}); err != nil {
			t.Fatal(err)
		}
		p := ToolOutputPreview(occ + " output")
		p["tool_call_id"], p["occurrence_id"] = "reused", occ
		if err = s.AppendTurnEvent(ctx, "t", "a", "tool.output", p); err != nil {
			t.Fatal(err)
		}
	}
	stale := ToolOutputPreview("late stale output")
	stale["tool_call_id"], stale["occurrence_id"] = "reused", "old"
	_ = s.AppendTurnEvent(ctx, "t", "a", "tool.output", stale)
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.SessionActivity(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	tool := a["tool"].(map[string]any)
	if tool["output_preview"] != "new output" || tool["occurrence_id"] != "new" {
		t.Fatal(tool)
	}
	_, _ = s.CreateSession(ctx, "b", "b", nil)
	other, _ := s.SessionActivity(ctx, "b")
	if other["tool"] != nil {
		t.Fatal("cross-session output")
	}
}
