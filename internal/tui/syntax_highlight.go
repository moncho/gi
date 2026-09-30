package tui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	gotui "github.com/grindlemire/go-tui"
)

// Pi highlights fenced code (pi-tui Markdown theme.highlightCode) with
// cli-highlight/highlight.js when the fence names a supported language and
// maps token classes onto its syntax* theme colours; unknown or missing
// languages fall back to plain mdCodeBlock. gi uses chroma with the same
// class-to-colour mapping. Like highlight.js in most grammars, punctuation
// and plain operators stay unstyled.

// Pi dark-theme syntax colours (resolved from Pi's theme in truecolor).
var (
	piSyntaxKeyword  = piRGB(105, 173, 208)
	piSyntaxFunction = piRGB(205, 154, 34)
	piSyntaxVariable = piRGB(93, 179, 186)
	piSyntaxString   = piRGB(222, 141, 90)
	piSyntaxNumber   = piRGB(104, 183, 141)
	piSyntaxType     = piRGB(167, 152, 215)
	piSyntaxComment  = piMuted
)

// piSyntaxStyle maps a syn-* marker style onto Pi's syntax colours.
func piSyntaxStyle(base gotui.Style, name string) (gotui.Style, bool) {
	switch name {
	case "syn-keyword":
		return base.Foreground(piSyntaxKeyword), true
	case "syn-function":
		return base.Foreground(piSyntaxFunction), true
	case "syn-variable":
		return base.Foreground(piSyntaxVariable), true
	case "syn-string":
		return base.Foreground(piSyntaxString), true
	case "syn-number":
		return base.Foreground(piSyntaxNumber), true
	case "syn-type":
		return base.Foreground(piSyntaxType), true
	case "syn-comment", "syn-meta":
		return base.Foreground(piSyntaxComment), true
	case "syn-added":
		return base.Foreground(piSuccess), true // toolDiffAdded
	case "syn-removed":
		return base.Foreground(piError), true // toolDiffRemoved
	case "syn-emph":
		return base.Italic(), true
	case "syn-strong":
		return base.Bold(), true
	}
	return base, false
}

// synClass maps a chroma token onto the highlight.js class Pi colours.
func synClass(t chroma.TokenType) string {
	switch {
	case t == chroma.CommentPreproc || t == chroma.CommentPreprocFile || t == chroma.NameDecorator:
		return "syn-meta"
	case t.InCategory(chroma.Comment):
		return "syn-comment"
	case t == chroma.KeywordType || t == chroma.NameBuiltin || t == chroma.NameBuiltinPseudo || t == chroma.NameClass:
		return "syn-type" // type, built_in, class
	case t == chroma.KeywordConstant:
		return "syn-number" // literal (true/false/nil)
	case t.InCategory(chroma.Keyword) || t == chroma.OperatorWord || t == chroma.NameTag:
		return "syn-keyword" // keyword, name (tags)
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic:
		return "syn-function" // title / function
	case t == chroma.NameAttribute || t.InSubCategory(chroma.NameVariable):
		return "syn-variable" // attr, variable
	case t.InSubCategory(chroma.LiteralString):
		return "syn-string" // string, regexp
	case t.InSubCategory(chroma.LiteralNumber):
		return "syn-number"
	case t == chroma.GenericInserted:
		return "syn-added"
	case t == chroma.GenericDeleted:
		return "syn-removed"
	case t == chroma.GenericEmph:
		return "syn-emph"
	case t == chroma.GenericStrong:
		return "syn-strong"
	}
	return ""
}

type synSegment struct{ text, class string }

// highlightCodeLines tokenizes code for a fence language and returns one
// segment list per source line; ok is false when the language is unknown
// (Pi then renders plain mdCodeBlock, never auto-detecting).
func highlightCodeLines(code, lang string) (lines [][]synSegment, ok bool) {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if lang == "" {
		return nil, false
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		return nil, false
	}
	iter, err := chroma.Coalesce(lexer).Tokenise(nil, code)
	if err != nil {
		return nil, false
	}
	lines = [][]synSegment{nil}
	for _, tok := range iter.Tokens() {
		class := synClass(tok.Type)
		parts := strings.Split(tok.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				lines = append(lines, nil)
			}
			if part != "" {
				lines[len(lines)-1] = append(lines[len(lines)-1], synSegment{part, class})
			}
		}
	}
	// chroma ends input with a newline; drop the trailing empty line it adds.
	if n := len(lines); n > 1 && len(lines[n-1]) == 0 && !strings.HasSuffix(code, "\n") {
		lines = lines[:n-1]
	}
	return lines, true
}

// wrapHighlightedLine wraps one highlighted source line to width cells after
// prefix, emitting balanced style markers on every row.
func wrapHighlightedLine(segments []synSegment, width int, prefix string) []string {
	prefixWidth := markdownRenderedWidth(prefix)
	contentWidth := max(2, width-prefixWidth)
	var rows []string
	var row strings.Builder
	used := 0
	row.WriteString(prefix)
	flush := func() {
		rows = append(rows, row.String())
		row.Reset()
		row.WriteString(strings.Repeat(" ", prefixWidth))
		used = 0
	}
	for _, seg := range segments {
		text := seg.text
		var run strings.Builder
		emit := func() {
			if run.Len() == 0 {
				return
			}
			if seg.class == "" {
				row.WriteString(run.String())
			} else {
				row.WriteString(mdStyled(seg.class, run.String()))
			}
			run.Reset()
		}
		for text != "" {
			cluster, cells, size := gotui.NextCluster(text)
			if size == 0 {
				break
			}
			text = text[size:]
			if used > 0 && used+cells > contentWidth {
				emit()
				flush()
			}
			run.WriteString(cluster)
			used += cells
		}
		emit()
	}
	rows = append(rows, row.String())
	return rows
}
