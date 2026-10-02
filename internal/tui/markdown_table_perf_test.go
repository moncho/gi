package tui

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// The former projection pipeline, retained as a correctness oracle.
func referenceChatMarkdown(role, prefix, markdown string, width int) []string {
	lines := renderMarkdownTranscript(prefix, markdown, width)
	if (role != "user" && role != "assistant") || !strings.Contains(markdown, "|") {
		return lines
	}
	root := tuiMarkdown.Parser().Parse(text.NewReader([]byte(markdown)))
	hasTable := false
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if _, ok := n.(*extast.Table); ok && entering {
			hasTable = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if !hasTable {
		return lines
	}
	body := renderMarkdownTranscript("", markdown, max(1, width-1))
	out := []string{encodeTranscriptBlockMarker(transcriptBlockMeta{Key: "markdown-table", Kind: role, MarkdownSource: markdown})}
	for _, line := range body {
		out = append(out, "│ "+line)
	}
	return out
}
func TestComplexMarkdownTableProjectionEquivalent(t *testing.T) {
	for _, source := range []string{complexTableFixture(8), "ordinary | prose with **bold**", "plain text", "| A | B |\n|---|---|\n| é | 👩🏽‍💻 🇵🇹 |"} {
		for _, width := range []int{16, 38, 60, 120} {
			for _, role := range []string{"assistant", "user", "plain"} {
				if got, want := renderChatMarkdown(role, "speaker: ", source, width), referenceChatMarkdown(role, "speaker: ", source, width); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s/%d projection differs", role, width)
				}
			}
		}
	}
}

func complexTableTimeline(width int) *chatTUI {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, outputWidth: width}
	c.transcript = renderChatMarkdown("assistant", "Gi: ", complexTableFixture(10), width)
	c.transcript = append(c.transcript, renderChatMarkdown("user", "you: ", "USER-BOUNDARY first line\n\nsecond line 👩🏽‍💻", width)...)
	c.appendTranscriptBlock(transcriptBlockMeta{Key: "separate-tool", Kind: "tool", Title: "read", Status: "ok"}, []string{"TOOL-BOUNDARY"})
	c.transcript = append(c.transcript, renderChatMarkdown("user", "you: ", "| USER-TABLE | Input |\n|---|---|\n| 🇵🇹 | literal \\| pipe |", width)...)
	c.transcript = append(c.transcript, renderChatMarkdown("assistant", "Gi: ", "ASSISTANT-BOUNDARY response", width)...)
	return c
}
func TestComplexMarkdownTableUserBoundary(t *testing.T) {
	original := piActiveTheme
	t.Cleanup(func() { applyPiTheme(original) })
	for _, theme := range []string{"dark", "light"} {
		applyPiTheme(theme)
		for _, width := range []int{38, 60, 120} {
			t.Run(fmt.Sprintf("%s/%d", theme, width), func(t *testing.T) {
				c := complexTableTimeline(width)
				blocks := c.transcriptBlocks()
				kinds := make([]string, len(blocks))
				for i, b := range blocks {
					kinds[i] = b.Kind
				}
				if !reflect.DeepEqual(kinds, []string{"assistant", "user", "tool", "user", "assistant"}) {
					t.Fatalf("message boundary lost: %v", kinds)
				}
				root := gotui.New(gotui.WithWidth(width), gotui.WithDirection(gotui.Column))
				previous := ""
				for _, b := range blocks {
					root.AddChild(c.renderTranscriptBlockAfter(b, previous))
					previous = b.Kind
				}
				height := root.HeightForWidth(width)
				root.SetHeight(gotui.Fixed(height))
				buf := gotui.NewBuffer(width, height)
				root.RenderTo(buf, width, height)
				foundUser, foundTool, foundAssistant := false, false, false
				for y := 0; y < height; y++ {
					row := bufferRow(buf, y)
					switch {
					case strings.Contains(row, "USER-BOUNDARY"), strings.Contains(row, "second line"), strings.Contains(row, "USER-TABLE"):
						foundUser = true
						for x := 0; x < width; x++ {
							if buf.Cell(x, y).Style.Bg != piUserBg {
								t.Fatalf("user not in its own full-width band at %d,%d", x, y)
							}
						}
					case strings.Contains(row, "TOOL-BOUNDARY"):
						foundTool = true
						if buf.Cell(0, y).Style.Bg != piToolSuccessBg {
							t.Fatal("tool band lost")
						}
					case strings.Contains(row, "ASSISTANT-BOUNDARY"):
						foundAssistant = true
						if !buf.Cell(0, y).Style.Bg.IsDefault() {
							t.Fatal("assistant inherited user band")
						}
					}
				}
				if !foundUser || !foundTool || !foundAssistant {
					t.Fatal("timeline content dropped")
				}
				// Complete cell equality includes backgrounds, links and wide continuations.
				for _, offset := range []int{0, 10, height - 24, height - 8} {
					c.transcriptScroll = max(0, offset)
					full := complexTimelineBuffer(c, false, width, 24)
					window := complexTimelineBuffer(c, true, width, 24)
					for y := 0; y < 24; y++ {
						for x := 0; x < width; x++ {
							if full.Cell(x, y) != window.Cell(x, y) {
								t.Fatalf("window differs offset=%d cell=%d,%d", offset, x, y)
							}
						}
					}
				}
			})
		}
	}
}
func complexTimelineBuffer(c *chatTUI, windowed bool, width, height int) *gotui.Buffer {
	root := gotui.New(gotui.WithWidth(width), gotui.WithHeight(height), gotui.WithDirection(gotui.Column), gotui.WithScrollable(gotui.ScrollVertical), gotui.WithScrollbarHidden(true), gotui.WithScrollOffset(0, c.transcriptScroll))
	blocks := c.transcriptBlocks()
	if windowed {
		c.addTranscriptWindow(root, blocks, width, height)
	} else {
		previous := ""
		for _, b := range blocks {
			root.AddChild(c.renderTranscriptBlockAfter(b, previous))
			previous = b.Kind
		}
	}
	buf := gotui.NewBuffer(width, height)
	root.RenderTo(buf, width, height)
	return buf
}

// Real terminal frame diffs through table/user/tool boundaries; no provider,
// production process or user's terminal is involved.
func TestComplexMarkdownTableTerminal(t *testing.T) {
	if os.Getenv("GI_COMPLEX_TABLE_PTY") == "" {
		t.Skip("make test-tui-complex-tables-pty")
	}
	original := piActiveTheme
	t.Cleanup(func() { applyPiTheme(original) })
	for _, theme := range []string{"dark", "light"} {
		applyPiTheme(theme)
		for _, width := range []int{60, 120} {
			t.Run(fmt.Sprintf("%s/%d", theme, width), func(t *testing.T) {
				socket := fmt.Sprintf("gi-table-perf-%d-%s-%d", os.Getpid(), theme, width)
				tm := func(args ...string) string {
					t.Helper()
					out, err := exec.Command("tmux", append([]string{"-L", socket, "-f", "/dev/null"}, args...)...).CombinedOutput()
					if err != nil {
						t.Fatalf("tmux: %v %s", err, out)
					}
					return string(out)
				}
				tm("new-session", "-d", "-s", "test", "-x", fmt.Sprint(width), "-y", "24", "sleep 120")
				defer exec.Command("tmux", "-L", socket, "kill-server").Run()
				tty := strings.TrimSpace(tm("display-message", "-p", "-t", "test", "#{pane_tty}"))
				output, err := os.OpenFile(tty, os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer output.Close()
				term := gotui.NewANSITerminalWithCaps(output, nil, gotui.Capabilities{Colors: gotui.ColorTrue, Unicode: true})
				term.HideCursor()
				c := complexTableTimeline(width)
				total := 0
				previous := ""
				for _, b := range c.transcriptBlocks() {
					total += c.renderTranscriptBlockAfter(b, previous).HeightForWidth(width)
					previous = b.Kind
				}
				offsets := []int{0, 10, 35, 70}
				for n := max(0, total-65); n <= total-24; n += 3 {
					offsets = append(offsets, n)
				}
				offsets = append(offsets, total-24)
				for n := len(offsets) - 2; n >= 0; n-- {
					offsets = append(offsets, offsets[n])
				}
				buffer := gotui.NewBuffer(width, 24)
				for step, offset := range offsets {
					c.transcriptScroll = offset
					root := gotui.New(gotui.WithWidth(width), gotui.WithHeight(24), gotui.WithDirection(gotui.Column), gotui.WithScrollable(gotui.ScrollVertical), gotui.WithScrollbarHidden(true), gotui.WithScrollOffset(0, offset))
					c.addTranscriptWindow(root, c.transcriptBlocks(), width, 24)
					buffer.Clear()
					root.RenderTo(buffer, width, 24)
					gotui.RenderRows(term, buffer, step == 0)
					normalize := func(s string) string { return strings.TrimRight(strings.ReplaceAll(s, "\u00a0", " "), "\n") }
					want := normalize(buffer.StringTrimmed())
					deadline := time.Now().Add(time.Second)
					for {
						got := normalize(tm("capture-pane", "-p", "-t", "test"))
						if got == want {
							break
						}
						if time.Now().After(deadline) {
							t.Fatalf("offset %d: terminal differs:\n%q\n%q", offset, got, want)
						}
						time.Sleep(10 * time.Millisecond)
					}
				}
				t.Logf("%d real-terminal frames", len(offsets))
			})
		}
	}
}
