package tui

import (
	"regexp"
	"strings"

	gotui "github.com/grindlemire/go-tui"
)

// Markdown element styles (Pi's getMarkdownTheme). The projector marks styled
// runs in the transcript text with zero-width markers, like inline code:
//
//	\x00gi-md:<style>\x00 … \x00gi-md-end\x00
//
// Markers nest and are balanced per line after wrapping, so a styled run can
// span wrapped rows. Search, selection and copy work on rendered text, where
// markers are never visible.

const markdownStyleEnd = "\x00gi-md-end\x00"

func markdownStyleStart(name string) string { return "\x00gi-md:" + name + "\x00" }

// mdStyled wraps text in a style run; empty text stays empty.
func mdStyled(name, text string) string {
	if text == "" {
		return ""
	}
	return markdownStyleStart(name) + text + markdownStyleEnd
}

var markdownMarkerPattern = regexp.MustCompile("\x00gi-[^\x00]*\x00")

// piMarkdownStyle maps a marker style onto Pi's dark-theme Markdown colours.
func piMarkdownStyle(base gotui.Style, name string) gotui.Style {
	switch name {
	case "heading":
		return base.Foreground(piWarning).Bold() // mdHeading
	case "codeblock":
		return base.Foreground(piSuccess) // mdCodeBlock
	case "codeborder", "quoteborder", "quote", "hr", "url":
		return base.Foreground(piMuted) // mdCodeBlockBorder, mdQuote*, mdHr, mdLinkUrl
	case "bullet":
		return base.Foreground(piAccent) // mdListBullet
	case "link":
		return base.Foreground(piMdLink).Underline()
	case "bold":
		return base.Bold()
	case "italic":
		return base.Italic()
	case "strike":
		return base.Strikethrough()
	}
	return base
}

var piMdLink = piRGB(105, 173, 208)

// markerToken scans a marker at s[0]; it returns the marker, whether it opens
// or closes a run, and the style name ("code" for inline code).
func markerToken(s string) (marker string, open bool, name string, ok bool) {
	switch {
	case strings.HasPrefix(s, markdownInlineCodeStart):
		return markdownInlineCodeStart, true, "code", true
	case strings.HasPrefix(s, markdownInlineCodeEnd):
		return markdownInlineCodeEnd, false, "code", true
	case strings.HasPrefix(s, markdownStyleEnd):
		return markdownStyleEnd, false, "", true
	case strings.HasPrefix(s, "\x00gi-md:"):
		end := strings.IndexByte(s[1:], 0)
		if end < 0 {
			return "", false, "", false
		}
		marker = s[:end+2]
		return marker, true, strings.TrimSuffix(strings.TrimPrefix(marker, "\x00gi-md:"), "\x00"), true
	}
	return "", false, "", false
}

// balanceStyleMarkers closes runs still open at the end of each line and
// reopens them at the start of the next, after wrapping split a run.
func balanceStyleMarkers(lines []string) []string {
	var stack []string // open markers
	closing := func() string {
		var b strings.Builder
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i] == markdownInlineCodeStart {
				b.WriteString(markdownInlineCodeEnd)
			} else {
				b.WriteString(markdownStyleEnd)
			}
		}
		return b.String()
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		prefix := strings.Join(stack, "")
		for j := 0; j < len(line); {
			if marker, open, _, ok := markerToken(line[j:]); ok {
				if open {
					stack = append(stack, marker)
				} else if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				j += len(marker)
				continue
			}
			j++
		}
		out[i] = prefix + line + closing()
	}
	return out
}

// upperOutsideMarkers upper-cases text without touching marker names.
func upperOutsideMarkers(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if marker, _, _, ok := markerToken(s[i:]); ok {
			b.WriteString(marker)
			i += len(marker)
			continue
		}
		j := i + 1
		for j < len(s) && s[j] != 0 {
			j++
		}
		b.WriteString(strings.ToUpper(s[i:j]))
		i = j
	}
	return b.String()
}
