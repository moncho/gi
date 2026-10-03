package sessionexport

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Pi's HTML export template and the libraries it inlines (template/NOTICE).
//
//go:embed template
var templateFS embed.FS

// Theme is the theme an HTML export takes its colours from: Pi's resolved
// theme colours as CSS values, in the theme's order, and the theme's
// explicit export colours ("" derives them from userMessageBg).
type Theme struct {
	Colors                 [][2]string
	PageBg, CardBg, InfoBg string
}

// SessionData is what Pi's template renders: the session header, its
// entries and the leaf entry.
type SessionData struct {
	Header  Entry   `json:"header"`
	Entries []Entry `json:"entries"`
	LeafID  any     `json:"leafId"`
}

// RenderHTML is Pi's generateHtml: the template with the theme's CSS
// variables, the session data and the inlined libraries.
func RenderHTML(header Entry, entries []Entry, theme Theme) ([]byte, error) {
	read := func(name string) string {
		data, _ := templateFS.ReadFile("template/" + name)
		return string(data)
	}
	var leaf any // Pi's getLeafId: the last entry, null for none
	if len(entries) > 0 {
		leaf = entries[len(entries)-1]["id"]
	}
	var data bytes.Buffer
	enc := json.NewEncoder(&data)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(SessionData{Header: header, Entries: entries, LeafID: leaf}); err != nil {
		return nil, err
	}
	userBg := "#343541"
	for _, c := range theme.Colors {
		if c[0] == "userMessageBg" && c[1] != "" {
			userBg = c[1]
		}
	}
	derived := deriveExportColors(userBg)
	pick := func(explicit, fallback string) string {
		if explicit != "" {
			return explicit
		}
		return fallback
	}
	pageBg, cardBg, infoBg := pick(theme.PageBg, derived[0]), pick(theme.CardBg, derived[1]), pick(theme.InfoBg, derived[2])
	lines := make([]string, 0, len(theme.Colors)+3)
	for _, c := range theme.Colors {
		lines = append(lines, "--"+c[0]+": "+c[1]+";")
	}
	lines = append(lines, "--exportPageBg: "+pageBg+";", "--exportCardBg: "+cardBg+";", "--exportInfoBg: "+infoBg+";")
	css := read("template.css")
	css = jsReplace(css, "{{THEME_VARS}}", strings.Join(lines, "\n      "))
	css = jsReplace(css, "{{BODY_BG}}", pageBg)
	css = jsReplace(css, "{{CONTAINER_BG}}", cardBg)
	css = jsReplace(css, "{{INFO_BG}}", infoBg)
	page := read("template.html")
	page = jsReplace(page, "{{CSS}}", css)
	page = jsReplace(page, "{{JS}}", read("template.js"))
	page = jsReplace(page, "{{SESSION_DATA}}", base64.StdEncoding.EncodeToString(bytes.TrimSuffix(data.Bytes(), []byte("\n"))))
	page = jsReplace(page, "{{MARKED_JS}}", read("vendor/marked.min.js"))
	page = jsReplace(page, "{{HIGHLIGHT_JS}}", read("vendor/highlight.min.js"))
	return []byte(page), nil
}

// jsReplace is JavaScript's String.prototype.replace with a string pattern,
// as Pi assembles the page: the first match only, and the replacement's $$,
// $&, $` and $' patterns substituted. Pi's own output carries those
// substitutions (template.js's "$${totalCost}" loses its dollar sign), so gi's
// does too.
func jsReplace(s, pattern, replacement string) string {
	i := strings.Index(s, pattern)
	if i < 0 {
		return s
	}
	var b strings.Builder
	for j := 0; j < len(replacement); j++ {
		if replacement[j] != '$' || j+1 == len(replacement) {
			b.WriteByte(replacement[j])
			continue
		}
		switch replacement[j+1] {
		case '$':
			b.WriteByte('$')
		case '&':
			b.WriteString(pattern)
		case '`':
			b.WriteString(s[:i])
		case '\'':
			b.WriteString(s[i+len(pattern):])
		default:
			b.WriteByte('$')
			continue
		}
		j++
	}
	return s[:i] + b.String() + s[i+len(pattern):]
}

var (
	hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{2})([0-9a-fA-F]{2})([0-9a-fA-F]{2})$`)
	rgbColor = regexp.MustCompile(`^rgb\s*\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)$`)
)

// parseCSSColor is Pi's export parseColor: #rrggbb or rgb(r, g, b).
func parseCSSColor(color string) ([3]float64, bool) {
	base := 16
	m := hexColor.FindStringSubmatch(color)
	if m == nil {
		base = 10
		m = rgbColor.FindStringSubmatch(color)
	}
	if m == nil {
		return [3]float64{}, false
	}
	var c [3]float64
	for i := range c {
		v, _ := strconv.ParseInt(m[i+1], base, 64)
		c[i] = float64(v)
	}
	return c, true
}

// jsRound is JavaScript's Math.round.
func jsRound(x float64) float64 { return math.Floor(x + 0.5) }

// deriveExportColors is Pi's deriveExportColors: page, card and info
// backgrounds from a base colour.
func deriveExportColors(base string) [3]string {
	c, ok := parseCSSColor(base)
	if !ok {
		return [3]string{"rgb(24, 24, 30)", "rgb(30, 30, 36)", "rgb(60, 55, 40)"}
	}
	linear := func(v float64) float64 {
		v /= 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	adjust := func(factor float64) string {
		ch := func(v float64) int { return int(math.Min(255, math.Max(0, jsRound(v*factor)))) }
		return fmt.Sprintf("rgb(%d, %d, %d)", ch(c[0]), ch(c[1]), ch(c[2]))
	}
	r, g, b := int(c[0]), int(c[1]), int(c[2])
	if 0.2126*linear(c[0])+0.7152*linear(c[1])+0.0722*linear(c[2]) > 0.5 {
		return [3]string{adjust(0.96), base, fmt.Sprintf("rgb(%d, %d, %d)", min(255, r+10), min(255, g+5), max(0, b-20))}
	}
	return [3]string{adjust(0.7), adjust(0.85), fmt.Sprintf("rgb(%d, %d, %d)", min(255, r+20), min(255, g+15), b)}
}
