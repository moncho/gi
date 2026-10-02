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
	if meta.Title != "read" && meta.Title != "write" && meta.Title != "edit" {
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
	if meta.Title == "read" {
		meta.ToolRange = readLineRange(m)
	}
	if meta.Title == "write" {
		if content, ok := m["content"].(string); ok {
			content = plainTerminalOutput(content)
			meta.ToolContent = &content
		}
	}
}

// fileToolSegments is Pi's highlightCode for a read or write preview: with a
// language from the path, highlighted lines whose unclassified text keeps
// the terminal's colour (mdCodeBlock when the language has no highlighter);
// without one, toolOutput lines. Tokenizing the whole source keeps multiline
// lexer state; tabs are Pi's three spaces; trailing empty lines are trimmed.
func fileToolSegments(source, filePath string, highlight bool) ([][]synSegment, gotui.Style) {
	source = strings.ReplaceAll(plainTerminalOutput(source), "\r", "")
	source = strings.ReplaceAll(source, "\t", "   ")
	base := piFg(piToolOutput)
	if lang := fileToolLanguage(filePath); highlight && lang != "" {
		if lines, ok := highlightCodeLines(source, lang); ok {
			return trimFileToolLines(lines), gotui.NewStyle()
		}
		base = piFg(piMdCodeBlock)
	}
	lines := make([][]synSegment, 0)
	for _, line := range strings.Split(source, "\n") {
		lines = append(lines, []synSegment{{text: line}})
	}
	return trimFileToolLines(lines), base
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

// fileToolRows lays out one preview line as Pi's Text does (word wrapped
// to the tool band's content width).
func fileToolRows(segments []synSegment, width int, base gotui.Style) []*gotui.Element {
	spans := make([]gotui.TextSpan, 0, len(segments))
	for _, seg := range segments {
		style := base
		if seg.class != "" {
			style, _ = piSyntaxStyle(base, seg.class)
		}
		spans = append(spans, gotui.TextSpan{Text: seg.text, Style: style})
	}
	return piTextRows(spans, width)
}

// appendFileToolPreview is the body of Pi's read result and write call: a
// blank line, then up to 10 lines while collapsed and the expand hint
// (write's also counts the total).
func (c *chatTUI) appendFileToolPreview(container *gotui.Element, block transcriptRenderableBlock, source string, highlight bool) {
	lines, base := fileToolSegments(source, block.ToolPath, highlight)
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
		for _, row := range fileToolRows(line, c.transcriptBlockContentWidth("tool"), base) {
			container.AddChild(row)
		}
	}
	if remaining := len(lines) - limit; remaining > 0 && c.extensionToolModes[block.Header] != "compact" {
		prefix := fmt.Sprintf("... (%d more lines, ", remaining)
		if block.Header == "write" {
			prefix = fmt.Sprintf("... (%d more lines, %d total, ", remaining, len(lines))
		}
		container.AddChild(textRow(expandHint(prefix, "to expand")...))
	}
}

// appendErrorText is a result shown whole in the error colour (Pi's write
// result).
func (c *chatTUI) appendErrorText(container *gotui.Element, text string) {
	if text = strings.TrimSpace(text); text == "" {
		return
	}
	container.AddChild(blankRow())
	for _, line := range strings.Split(text, "\n") {
		for _, row := range piTextRows([]gotui.TextSpan{{Text: line, Style: piFg(piError)}}, c.transcriptBlockContentWidth("tool")) {
			container.AddChild(row)
		}
	}
}
