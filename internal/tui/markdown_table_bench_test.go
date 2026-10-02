package tui

import (
	"fmt"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
)

// Representative of the long, mixed-script tables in an assistant response.
func complexTableFixture(rows int) string {
	var s strings.Builder
	s.WriteString("### Deployment compatibility matrix\n\n| Component | Version | Local | Remote | Configuration | Notes |\n|:--|:--:|:--:|:--:|:--|:--|\n")
	for i := 0; i < rows; i++ {
		fmt.Fprintf(&s, "| **Component %d 👩🏽‍💻** | `v%d.2` | ✅ 日本語 | ⚠️ 🇵🇹 é | `left \\| right` | *العربية* and **styled text** with `integration_test_transcript_selection_clipboard_dispatch` 🧪 |\n", i, i)
	}
	s.WriteString("\nAfter the table: ordinary prose with **bold**, *italic*, and `code`.\n")
	return s.String()
}

var tableBenchmarkLines []string

func BenchmarkComplexMarkdownTable(b *testing.B) {
	for _, rows := range []int{40, 200} {
		source := complexTableFixture(rows)
		b.Run(fmt.Sprintf("rows=%d", rows), func(b *testing.B) {
			for _, width := range []int{60, 120} {
				b.Run(fmt.Sprintf("project/width=%d", width), func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						tableBenchmarkLines = renderChatMarkdown("assistant", "Gi: ", source, width)
					}
				})
			}
			// A provider delivers complete rows incrementally; include projection
			// and first layout of each changed snapshot, not just a warm scroll.
			snapshots := make([]string, 0, 8)
			for step := 1; step <= 8; step++ {
				snapshots = append(snapshots, complexTableFixture(max(1, rows*step/8)))
			}
			b.Run("stream-eight-snapshots", func(b *testing.B) {
				c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, outputWidth: 120}
				buffer := gotui.NewBuffer(120, 32)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// New turn/frame caches, then reuse them as that turn streams.
					c.blockCache = nil
					c.blocksMemo = transcriptBlocksMemo{}
					for _, snapshot := range snapshots {
						c.transcript = renderChatMarkdown("assistant", "Gi: ", snapshot, 120)
						root := gotui.New(gotui.WithWidth(120), gotui.WithHeight(32), gotui.WithDirection(gotui.Column), gotui.WithScrollable(gotui.ScrollVertical), gotui.WithScrollbarHidden(true))
						c.addTranscriptWindow(root, c.transcriptBlocks(), 120, 32)
						root.ScrollToBottom()
						buffer.Clear()
						root.RenderTo(buffer, 120, 32)
					}
				}
			})
			for _, mode := range []string{"cold", "scroll", "resize"} {
				b.Run(mode, func(b *testing.B) {
					const height = 32
					c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, outputWidth: 120, transcript: renderChatMarkdown("assistant", "Gi: ", source, 120)}
					buf := gotui.NewBuffer(120, height)
					frame := func(i int) {
						if mode == "cold" {
							c.blockCache = nil
							c.blocksMemo = transcriptBlocksMemo{}
						}
						width := 120
						if mode == "resize" && i%2 == 0 {
							width = 60
						}
						c.outputWidth = width
						c.transcriptScroll = i % 200
						root := gotui.New(gotui.WithWidth(width), gotui.WithHeight(height), gotui.WithDirection(gotui.Column), gotui.WithScrollable(gotui.ScrollVertical), gotui.WithScrollbarHidden(true), gotui.WithScrollOffset(0, c.transcriptScroll))
						c.addTranscriptWindow(root, c.transcriptBlocks(), width, height)
						buf.Clear()
						root.RenderTo(buf, width, height)
					}
					frame(0)
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						frame(i)
					}
				})
			}
		})
	}
}
