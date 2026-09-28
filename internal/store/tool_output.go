package store

import (
	"strings"
	"unicode/utf8"
)

// ToolOutputPreview follows the installed Piclaw 3.2.4 status window: the last
// 100 lines, then at most 12 KiB. This is a preview, never model history.
func ToolOutputPreview(text string) map[string]any {
	text = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"))
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	total := len(lines)
	if len(lines) > 100 {
		lines = lines[len(lines)-100:]
	}
	preview := strings.Join(lines, "\n")
	truncated := total > len(lines)
	if len(preview) > 12*1024 {
		preview = preview[len(preview)-12*1024:]
		for len(preview) > 0 && !utf8.RuneStart(preview[0]) {
			preview = preview[1:]
		}
		truncated = true
	}
	return map[string]any{"output_preview": preview, "output_total_lines": total, "output_preview_lines": strings.Count(preview, "\n") + 1, "output_truncated": truncated}
}
