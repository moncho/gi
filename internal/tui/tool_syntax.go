package tui

import (
	"fmt"
	gotui "github.com/grindlemire/go-tui"
	"path"
	"strings"
)

// Pi 0.99.2 getLanguageFromPath: explicit extensions only, no prose detection.
var fileToolLanguages = map[string]string{
	"ts":         "typescript",
	"tsx":        "typescript",
	"js":         "javascript",
	"jsx":        "javascript",
	"mjs":        "javascript",
	"cjs":        "javascript",
	"py":         "python",
	"rb":         "ruby",
	"rs":         "rust",
	"go":         "go",
	"java":       "java",
	"kt":         "kotlin",
	"swift":      "swift",
	"c":          "c",
	"h":          "c",
	"cpp":        "cpp",
	"cc":         "cpp",
	"cxx":        "cpp",
	"hpp":        "cpp",
	"cs":         "csharp",
	"php":        "php",
	"sh":         "bash",
	"bash":       "bash",
	"zsh":        "bash",
	"fish":       "fish",
	"ps1":        "powershell",
	"sql":        "sql",
	"html":       "html",
	"htm":        "html",
	"css":        "css",
	"scss":       "scss",
	"sass":       "sass",
	"less":       "less",
	"json":       "json",
	"yaml":       "yaml",
	"yml":        "yaml",
	"toml":       "toml",
	"xml":        "xml",
	"md":         "markdown",
	"markdown":   "markdown",
	"dockerfile": "dockerfile",
	"makefile":   "makefile",
	"cmake":      "cmake",
	"lua":        "lua",
	"perl":       "perl",
	"r":          "r",
	"scala":      "scala",
	"clj":        "clojure",
	"ex":         "elixir",
	"exs":        "elixir",
	"erl":        "erlang",
	"hs":         "haskell",
	"ml":         "ocaml",
	"vim":        "vim",
	"graphql":    "graphql",
	"proto":      "protobuf",
	"tf":         "hcl",
	"hcl":        "hcl",
}

func fileToolLanguage(filePath string) string {
	name := strings.ToLower(path.Base(strings.ReplaceAll(filePath, "\\", "/")))
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		name = name[dot+1:]
	}
	return fileToolLanguages[name]
}

// Presentation metadata is independent of the abbreviated call header. Never
// read the filesystem to reconstruct a past write or read.
func setFileToolArguments(meta *transcriptBlockMeta, args any) {
	if meta.Title != "read" && meta.Title != "write" {
		return
	}
	m, ok := args.(map[string]any)
	if !ok {
		return
	}
	if p, ok := m["file_path"].(string); ok {
		meta.ToolPath = plainTerminalOutput(p)
	} else if p, ok := m["path"].(string); ok {
		meta.ToolPath = plainTerminalOutput(p)
	}
	if meta.ToolPath != "" {
		meta.Detail = truncate(strings.Join(strings.Fields(meta.ToolPath), " "), 500)
	}
	if meta.Title == "write" {
		if content, ok := m["content"].(string); ok {
			content = plainTerminalOutput(content)
			meta.ToolContent = &content
		}
	}
}

// Tokenize the complete available source before selecting preview lines. This
// preserves multiline lexer state. Tabs follow Pi's fixed three-space display.
func fileToolSegments(source, filePath string, highlight bool) [][]synSegment {
	source = strings.ReplaceAll(plainTerminalOutput(source), "\r", "")
	source = strings.ReplaceAll(source, "\t", "   ")
	if highlight {
		if lines, ok := highlightCodeLines(source, fileToolLanguage(filePath)); ok {
			return trimFileToolLines(lines)
		}
	}
	lines := make([][]synSegment, 0)
	for _, line := range strings.Split(source, "\n") {
		lines = append(lines, []synSegment{{text: line}})
	}
	return trimFileToolLines(lines)
}

func trimFileToolLines(lines [][]synSegment) [][]synSegment {
	for len(lines) > 0 {
		last := lines[len(lines)-1]
		empty := true
		for _, seg := range last {
			if seg.text != "" {
				empty = false
				break
			}
		}
		if !empty {
			break
		}
		lines = lines[:len(lines)-1]
	}
	return lines
}

// Literal rich-text rows: no Markdown interpretation, no second word wrapping,
// and width measured inside the tool band's padding on every render/resize.
func fileToolRows(segments []synSegment, width int, base gotui.Style) []*gotui.Element {
	width = max(1, width)
	var rows []*gotui.Element
	var spans []gotui.TextSpan
	used := 0
	flush := func() {
		var linked []gotui.TextSpan
		for _, span := range spans {
			linked = append(linked, transcriptLinkSpans(span.Text, span.Style)...)
		}
		rows = append(rows, gotui.New(gotui.WithWidthPercent(100), gotui.WithHeight(1), gotui.WithWrap(false), gotui.WithRichText(linked...)))
		spans = nil
		used = 0
	}
	for _, seg := range segments {
		style := base
		if seg.class != "" {
			style, _ = piSyntaxStyle(base, seg.class)
		}
		for text := seg.text; text != ""; {
			cluster, cells, size := gotui.NextCluster(text)
			if size == 0 {
				break
			}
			text = text[size:]
			if used > 0 && used+cells > width {
				flush()
			}
			if cells > width {
				cluster = "�"
				cells = 1
			} // impossible-width cluster: do not overflow the band
			if n := len(spans); n > 0 && spans[n-1].Style == style {
				spans[n-1].Text += cluster
			} else {
				spans = append(spans, gotui.TextSpan{Text: cluster, Style: style})
			}
			used += cells
		}
	}
	flush()
	return rows
}

func (c *chatTUI) appendFileToolPreview(container *gotui.Element, block transcriptRenderableBlock, source string, highlight bool) {
	lines := fileToolSegments(source, block.ToolPath, highlight)
	limit := len(lines)
	if !block.Expanded && limit > toolPreviewLines {
		limit = toolPreviewLines
	}
	if c.extensionToolModes[block.Header] == "compact" && limit > 1 {
		limit = 1
	}
	if len(lines) == 0 {
		return
	}
	container.AddChild(blankRow())
	for _, line := range lines[:limit] {
		for _, row := range fileToolRows(line, c.transcriptBlockContentWidth("tool"), piFg(piMuted)) {
			container.AddChild(row)
		}
	}
	if remaining := len(lines) - limit; remaining > 0 && c.extensionToolModes[block.Header] != "compact" {
		container.AddChild(textRow(expandHint(fmt.Sprintf("... (%d more lines, ", remaining), "to expand")...))
	}
}
