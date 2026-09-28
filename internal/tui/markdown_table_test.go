package tui

import (
	"encoding/json"
	"fmt"
	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

const tableRegression = "| Command | Result |\n| --- | --- |\n| `go test ./internal/tui` | Tests pass with terminal columns preserved |\n| 界面 🙂 | é and wide glyphs stay aligned |\n"

func TestMarkdownTableGridUsesTerminalColumns(t *testing.T) {
	for _, width := range []int{20, 30, 56, 96, 136} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			lines := renderMarkdownTranscript("", tableRegression, width)
			if !strings.HasPrefix(lines[0], "┌") {
				t.Fatalf("table became prose/list at width %d: %q", width, lines)
			}
			border := gotui.StringWidth(lines[0])
			if border > width {
				t.Fatalf("border %d > %d", border, width)
			}
			for _, line := range lines {
				if gotui.StringWidth(stripMarkdownInlineStyleMarkers(line)) != border {
					t.Fatalf("misaligned %d != %d: %q", gotui.StringWidth(stripMarkdownInlineStyleMarkers(line)), border, line)
				}
			}
			joined := strings.Join(lines, "\n")
			if !strings.Contains(joined, "界面 🙂") || !strings.Contains(joined, "é") {
				t.Fatal("unicode lost", joined)
			}
			if !strings.Contains(joined, markdownInlineCodeStart) {
				t.Fatal("inline code style lost")
			}
		})
	}
}
func TestMarkdownTableStreamingEveryPrefixStaysBounded(t *testing.T) {
	for i := strings.Index(tableRegression, "\n| `") + 1; i <= len(tableRegression); i++ {
		if i < len(tableRegression) && tableRegression[i]&0xc0 == 0x80 {
			continue
		}
		lines := renderMarkdownTranscript("", tableRegression[:i], 56)
		for _, line := range lines {
			if gotui.StringWidth(stripMarkdownInlineStyleMarkers(line)) > 56 {
				t.Fatalf("prefix %d overflows: %q", i, line)
			}
		}
	}
}

func TestMarkdownTableSourceReflowsActualRenderedGridOnResize(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}}
	c.transcript = renderChatMarkdown("assistant", "Gi: ", tableRegression, 136)
	for _, width := range []int{38, 60, 100, 140} {
		c.outputWidth = width
		blocks := c.buildTranscriptRenderableBlocks(c.transcript)
		if len(blocks) != 1 || blocks[0].MarkdownSource != tableRegression {
			t.Fatal(blocks)
		}
		root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(width))
		root.AddChild(c.renderTranscriptBlock(blocks[0]))
		root.Calculate(width, 80)
		b := gotui.NewBuffer(width, 80)
		root.Render(b, width, 80)
		screen := strings.ReplaceAll(b.StringTrimmed(), "\u00a0", " ")
		grid := 0
		for _, line := range strings.Split(screen, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			r, _ := utf8.DecodeRuneInString(line)
			if strings.ContainsRune("┌├└│", r) {
				grid++
				if !strings.ContainsRune("┐┤┘│", []rune(line)[len([]rune(line))-1]) {
					t.Fatalf("split border width%d: %q", width, line)
				}
			} else {
				t.Fatalf("stray wrapped table row width%d: %q", width, line)
			}
		}
		if grid < 7 || strings.Contains(screen, "gi:block") {
			t.Fatal(screen)
		}
	}
}

func TestMarkdownTableMatchesInstalledPiGeometry(t *testing.T) {
	path := os.Getenv("PI_TABLE_ORACLE")
	if path == "" {
		t.Skip("make test-tui-tables supplies installed oracle")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Markdown string `json:"markdown"`
		Cases    []struct {
			Width int      `json:"width"`
			Lines []string `json:"lines"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(data, &oracle); err != nil {
		t.Fatal(err)
	}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, test := range oracle.Cases {
		got := renderMarkdownTranscript("", oracle.Markdown, test.Width)
		for i := range got {
			got[i] = stripMarkdownInlineStyleMarkers(got[i])
		}
		want := make([]string, len(test.Lines))
		for i, line := range test.Lines {
			want[i] = strings.TrimRight(ansi.ReplaceAllString(line, ""), " ") // Pi pads the full viewport outside the grid.
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("width%d: got\n%s\nPi\n%s", test.Width, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}

func TestMarkdownTableSearchUsesResizedGridWithoutMetadata(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{AssistantName: "Gi"}, transcriptRef: gotui.NewRef()}
	c.transcript = renderChatMarkdown("assistant", "Gi: ", tableRegression, 136)
	c.ensureInput()
	c.input.SetText("unsent")
	c.toggleTranscriptSearch()
	c.search.input.SetText("界面")
	for _, width := range []int{38, 60, 100} {
		c.refreshTranscriptSearch(width)
		if len(c.search.matches) != 1 {
			t.Fatalf("width%d matches %d", width, len(c.search.matches))
		}
		for _, row := range c.search.rows {
			if strings.Contains(row.text, "gi:block") {
				t.Fatal("metadata searchable")
			}
		}
	}
	c.closeTranscriptSearch()
	if c.input.Text() != "unsent" {
		t.Fatal(c.input.Text())
	}
}
