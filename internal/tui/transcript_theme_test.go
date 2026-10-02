package tui

import (
	"fmt"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

func TestTranscriptPlainOutputAndUserBackground(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {100, 22}, {140, 36}} {
		for _, kind := range []string{"user", "assistant", "tool", "bash", "local", "error", "thought", "hook", "dispatcher", "subturn", "compact"} {
			for _, expanded := range []bool{false, true} {
				t.Run(fmt.Sprintf("%dx%d/%s/expanded=%v", size[0], size[1], kind, expanded), func(t *testing.T) {
					c := &chatTUI{}
					head, body, hint, _, border := transcriptBlockPalette(kind, "ok", false)
					block := transcriptRenderableBlock{Kind: kind, Status: "ok", Header: "Title", Body: []string{"  source code", "second line", "third line"}, Expandable: true, Expanded: expanded, Border: gotui.BorderRounded, BorderStyle: border, HeaderStyle: head, BodyStyle: body, HintStyle: hint}
					el := c.renderTranscriptBlock(block)
					buf := gotui.NewBuffer(size[0], size[1])
					root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(size[0]), gotui.WithHeight(size[1]))
					root.AddChild(el)
					root.RenderTo(buf, size[0], size[1])
					wantBg := gotui.Color{}
					switch kind {
					case "user":
						wantBg = piUserBg
					case "tool":
						// Pi's ToolExecutionComponent success band.
						wantBg = toolBandColor("ok")
					}
					separator, _, _ := transcriptSpacing(kind)
					for y := 0; y < el.Rect().Height; y++ {
						for x := 0; x < size[0]; x++ {
							cell := buf.Cell(x, y)
							bg := wantBg
							if y < separator {
								bg = gotui.Color{}
							}
							if cell.Style.Bg != bg {
								t.Fatalf("background at %d,%d: got %v, want %v", x, y, cell.Style.Bg, bg)
							}
							// Pi's BashExecutionComponent draws horizontal rules; nothing is boxed.
							if strings.ContainsRune("╭╮╰╯│", cell.Rune) || cell.Rune == '─' && kind != "bash" {
								t.Fatalf("boxed transcript at %d,%d: %c", x, y, cell.Rune)
							}
						}
					}
					text := strings.ReplaceAll(buf.StringTrimmed(), "\u00a0", " ")
					if kind != "thought" && !strings.Contains(text, "  source code") {
						t.Fatalf("lost indentation: %q", text)
					}
				})
			}
		}
	}
}

func TestPiTranscriptRenderedScrollBounds(t *testing.T) {
	for _, width := range []int{60, 100, 140} {
		c := &chatTUI{transcript: []string{"user: " + strings.Repeat("wrapped output ", 100)}, transcriptRef: gotui.NewRef()}
		root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(width), gotui.WithHeight(10), gotui.WithScrollable(gotui.ScrollVertical))
		root.AddChild(c.renderInlineStyledLine(c.transcript[0], gotui.NewStyle()))
		root.RenderTo(gotui.NewBuffer(width, 10), width, 10)
		c.transcriptRef.Set(root)
		if c.transcriptMaxScroll() <= 0 {
			t.Fatal("wrapped text cannot scroll", width)
		}
		c.scrollTranscriptToBottom()
		_, maxY := root.MaxScroll()
		if c.transcriptScroll != maxY || !c.stickToBottom {
			t.Fatal("bottom uses source line count")
		}
		root.ScrollTo(0, maxY)
		c.pageTranscript(-1)
		if c.transcriptScroll != max(0, maxY-6) || c.stickToBottom {
			t.Fatal("page lost rendered offset", c.transcriptScroll, maxY)
		}
		c.scrollTranscriptToTop()
		if c.transcriptScroll != 0 || c.stickToBottom {
			t.Fatal("top following")
		}
	}
}

func TestPiToolOutputToggleRetainsEditor(t *testing.T) {
	c := &chatTUI{}
	c.ensureInput()
	c.input.SetText("newer draft")
	c.input.cursorPos = 3
	c.appendTranscriptBlock(transcriptBlockMeta{Key: "tool", Kind: "tool", Title: "read", Status: "ok"}, []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"})
	c.toggleToolOutput()
	if !c.transcriptExpanded["tool"] {
		t.Fatal("tool not expanded")
	}
	c.toggleToolOutput()
	if c.transcriptExpanded["tool"] {
		t.Fatal("tool not collapsed")
	}
	if c.input.Text() != "newer draft" || c.input.cursorPos != 3 {
		t.Fatal("expand changed editor")
	}
}

func TestPiFullscreenWheelOverEditorScrollsTranscript(t *testing.T) {
	c := &chatTUI{transcriptRef: gotui.NewRef()}
	c.ensureInput()
	c.input.SetText("draft")
	c.input.cursorPos = 2
	el := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(60), gotui.WithHeight(10), gotui.WithScrollable(gotui.ScrollVertical))
	for i := 0; i < 30; i++ {
		el.AddChild(gotui.New(gotui.WithText(fmt.Sprint(i)), gotui.WithHeight(1)))
	}
	el.RenderTo(gotui.NewBuffer(60, 18), 60, 10)
	el.ScrollTo(0, 20)
	c.transcriptRef.Set(el)
	c.transcriptRegion = el
	c.stickToBottom = true
	// Pi "auto" wheel: an isolated notch moves one line.
	if !c.handleTranscriptScrollEvent(gotui.MouseEvent{Button: gotui.MouseWheelUp, X: 5, Y: 15}) || c.transcriptScroll != 19 || c.stickToBottom {
		t.Fatal("editor wheel failed to scroll history", c.transcriptScroll)
	}
	if c.input.Text() != "draft" || c.input.cursorPos != 2 {
		t.Fatal("wheel altered editor")
	}
	c.modelMenuOpen = true
	if c.handleTranscriptScrollEvent(gotui.MouseEvent{Button: gotui.MouseWheelUp, X: 5, Y: 15}) {
		t.Fatal("wheel stole selector input")
	}
}

func TestPiMessageSpacingGroupsMarkdownContinuationRows(t *testing.T) {
	for _, width := range []int{60, 100, 140} {
		c := &chatTUI{}
		c.cfg.AssistantName = "Gi"
		lines := renderMarkdownTranscript("you: ", "First paragraph\n\n- list one\n- list two\n\n```go\nfmt.Println(\"hello\")\n```", width-2)
		lines = append(lines, renderMarkdownTranscript("Gi: ", "Answer paragraph\n\nSecond paragraph\n\n- detail", width-2)...)
		lines = append(lines, "you: another message", "sys: independent notice")
		blocks := c.buildTranscriptRenderableBlocks(lines)
		if len(blocks) != 4 || blocks[0].Kind != "user" || blocks[1].Kind != "assistant" || len(blocks[0].Body) < 4 || len(blocks[1].Body) < 3 {
			t.Fatalf("%d: continuation rows escaped parent message: %+v", width, blocks)
		}
		for _, block := range blocks[:2] {
			el := c.renderTranscriptBlock(block)
			height := el.HeightForWidth(width)
			root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(width), gotui.WithHeight(height))
			root.AddChild(el)
			buf := gotui.NewBuffer(width, height)
			root.RenderTo(buf, width, height)
			text := strings.ReplaceAll(buf.StringTrimmed(), "\u00a0", " ")
			if block.Kind == "user" && !strings.Contains(text, "list two") {
				t.Fatal("lost user body", text)
			}
			if block.Kind == "assistant" && !strings.Contains(text, "Second paragraph") {
				t.Fatal("lost assistant body", text)
			}
			if block.Expandable {
				t.Fatal("ordinary multiline message collapsed")
			}
			first := strings.TrimSpace(strings.Split(text, "\n")[0])
			if first != "" {
				t.Fatalf("missing message top blank: %q", first)
			}
		}
	}
}

func TestAssistantResponseSeparatesFollowingTimelineContent(t *testing.T) {
	for _, tc := range []struct {
		name, next      string
		wantBlankBefore int
	}{
		{"user", "you: next", 2}, // external gap plus the user band's top padding
		{"system", "sys: notice", 1},
		{"assistant", "Gi: next", 1}, // the next assistant has its own spacer
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &chatTUI{}
			c.cfg.AssistantName = "Gi"
			c.transcript = []string{"Gi: answer", tc.next}
			rows := c.transcriptRowsAtWidth(80)
			seenAnswer, blanks := false, 0
			for _, row := range rows {
				text := strings.TrimSpace(row.text)
				if strings.Contains(text, "answer") {
					seenAnswer = true
					continue
				}
				if !seenAnswer {
					continue
				}
				if text == "" {
					blanks++
					continue
				}
				if !strings.Contains(text, strings.TrimPrefix(strings.TrimPrefix(tc.next, "Gi: "), "you: ")) {
					t.Fatalf("unexpected intervening row: %q", text)
				}
				if blanks != tc.wantBlankBefore {
					t.Fatalf("blank rows before %q: got %d, want %d", tc.next, blanks, tc.wantBlankBefore)
				}
				return
			}
			t.Fatalf("missing next message in rows: %+v", rows)
		})
	}
}

func TestAssistantResponseDoesNotAddTrailingGap(t *testing.T) {
	c := &chatTUI{}
	c.cfg.AssistantName = "Gi"
	c.transcript = []string{"Gi: answer"}
	rows := c.transcriptRowsAtWidth(80)
	if len(rows) == 0 || strings.TrimSpace(rows[len(rows)-1].text) != "answer" {
		t.Fatalf("assistant should not leave a trailing spacer: %+v", rows)
	}
}
