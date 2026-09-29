package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
)

// Exercises real ANSI frame diffs, rather than comparing two width calculations
// from the same renderer. Each test owns its tmux server and never touches the
// user's terminal or sessions.
func TestMarkdownTableScrollTerminal(t *testing.T) {
	if os.Getenv("GI_TABLE_SCROLL_PTY") == "" {
		t.Skip("make test-tui-table-scroll enables the tmux regression")
	}
	for _, width := range []int{38, 60, 100, 140} {
		t.Run(fmt.Sprint(width), func(t *testing.T) { testMarkdownTableScrollTerminal(t, width) })
	}
}

func testMarkdownTableScrollTerminal(t *testing.T, width int) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatal("tmux is required by test-tui-table-scroll")
	}
	socket := fmt.Sprintf("gi-table-scroll-%d-%d", os.Getpid(), width)
	tm := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("tmux", append([]string{"-L", socket, "-f", "/dev/null"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("tmux %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	tm("new-session", "-d", "-s", "test", "-x", fmt.Sprint(width), "-y", "24", "sleep 120")
	defer exec.Command("tmux", "-L", socket, "kill-server").Run()
	tty := strings.TrimSpace(tm("display-message", "-p", "-t", "test", "#{pane_tty}"))
	out, err := os.OpenFile(tty, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	term := gotui.NewANSITerminalWithCaps(out, nil, gotui.Capabilities{Colors: gotui.ColorTrue, Unicode: true})
	term.HideCursor()
	const markdown = "| Component | Local | Remote | Notes |\n|---|---|---|---|\n| **Terminal clipboard** | ✅ Supported¹ | ⚠️ Conditional | Sends to the terminal |\n| Native clipboard | ❌ No | — | `left \\| right` |\n| Unicode 日本語 / العربية / 🧪 | é | 👩‍💻 | Flags 🇵🇹 |\n| Metrics ↑ ↓ | ≤ 50.0 | ≥ 25.0 | **Pass** — 0.00% |\n"
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, outputWidth: width}
	c.transcript = renderChatMarkdown("assistant", "Gi: ", strings.Repeat("Before the table\n\n", 6)+markdown+strings.Repeat("\nAfter the table\n", 20)+markdown, width)
	blocks := c.buildTranscriptRenderableBlocks(c.transcript)
	root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(width), gotui.WithHeight(23), gotui.WithScrollable(gotui.ScrollVertical), gotui.WithScrollbarHidden(true))
	for _, block := range blocks {
		root.AddChild(c.renderTranscriptBlock(block))
	}
	root.Calculate(width, 24)
	gotui.RenderTree(gotui.NewBuffer(width, 24), root)
	_, maxY := root.MaxScroll()
	if maxY < 24 {
		t.Fatalf("fixture must scroll through tables: maxY=%d", maxY)
	}
	t.Logf("checking %d frames", 2*maxY+1)
	buffer := gotui.NewBuffer(width, 24)
	normalize := func(s string) string { return strings.TrimRight(strings.ReplaceAll(s, "\u00a0", " "), "\n") }
	for step := 0; step <= 2*maxY; step++ {
		offset := step
		if offset > maxY {
			offset = 2*maxY - step
		}
		root.ScrollTo(0, offset)
		buffer.Clear()
		root.Calculate(width, 24)
		gotui.RenderTree(buffer, root)
		term.Flush(buffer.Diff())
		buffer.Swap()
		want := normalize(buffer.StringTrimmed())
		var got string
		deadline := time.Now().Add(time.Second)
		for {
			got = normalize(tm("capture-pane", "-p", "-t", "test"))
			if got == want || time.Now().After(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if got != want {
			t.Fatalf("scroll %d step %d:\ngot:\n%s\nwant:\n%s", offset, step, got, want)
		}
	}
}

// Runs in the ordinary Go suite too: table padding must survive the rich-text
// renderer without synthetic NBSPs or an extra line-wrap pass.
func TestMarkdownTablePreservesASCIIPadding(t *testing.T) {
	c := &chatTUI{}
	for _, line := range []string{
		"│ Flags 🇵🇹  │ 日本語 🧪  │",
		"│ " + markdownInlineCodeStart + "Flags 🇵🇹  " + markdownInlineCodeEnd + " │ ⚠️  │",
		"    " + markdownInlineCodeStart + "Flags 🇵🇹  " + markdownInlineCodeEnd + " done",
	} {
		plain := stripMarkdownInlineStyleMarkers(line)
		width := gotui.StringWidth(plain)
		root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(width), gotui.WithHeight(2))
		root.AddChild(c.renderInlineStyledLine(line, gotui.NewStyle()))
		buffer := gotui.NewBuffer(width, 2)
		root.Calculate(width, 2)
		gotui.RenderTree(buffer, root)
		if got := strings.TrimRight(buffer.StringTrimmed(), "\n"); got != plain {
			t.Fatalf("preformatted row changed: got %q, want %q", got, plain)
		}
	}
}
