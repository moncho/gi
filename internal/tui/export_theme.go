package tui

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/rcarmo/gi/internal/sessionexport"
)

// piBackgroundTokens are the colour tokens Pi's themes use as backgrounds
// (theme.js BACKGROUND_TOKENS).
var piBackgroundTokens = map[string]bool{"selectedBg": true, "searchMatchBg": true, "userMessageBg": true, "customMessageBg": true, "toolPendingBg": true, "toolSuccessBg": true, "toolErrorBg": true}

// piThemeFallbacks are the optional tokens and the tokens they fall back to,
// in the order Pi's withThemeColorFallbacks adds them.
var piThemeFallbacks = [][2]string{{"scrollbarTrack", "muted"}, {"scrollbarThumb", "text"}, {"thinkingMax", "thinkingXhigh"}, {"searchMatchBg", "selectedBg"}, {"searchMatchText", "text"}}

// exportTheme is the active theme as Pi's HTML export takes it
// (getResolvedThemeColors and getThemeExportColors).
func exportTheme() sessionexport.Theme {
	name := piActiveTheme
	if t, ok := piBuiltinExportThemes[name]; ok {
		return t
	}
	if name == systemThemeName {
		generated, appearance, ok := generateSystemThemeColors(systemThemeInput{background: piTerminal.colors.background, foreground: piTerminal.colors.foreground, palette: piTerminal.colors.palette, saturation: 1})
		if !ok {
			return piBuiltinExportThemes["dark"]
		}
		keys := make([]string, 0, len(systemTokens))
		colors := map[string]themeColor{}
		for _, token := range systemTokens {
			keys = append(keys, token[0])
			c, err := parseThemeColor(json.RawMessage(`"` + generated[token[0]] + `"`))
			if err != nil {
				c = themeColor{isDef: true}
			}
			colors[token[0]] = c
		}
		// The system theme has no export colours of its own.
		return sessionexport.Theme{Colors: resolvedThemeColors(keys, colors, appearance)}
	}
	t, err := customExportTheme(name)
	if err != nil {
		return piBuiltinExportThemes["dark"]
	}
	return t
}

// customExportTheme is a custom theme file as Pi's export takes it.
func customExportTheme(name string) (sessionexport.Theme, error) {
	var doc map[string]json.RawMessage
	for _, dir := range customThemeDirs() {
		path := filepath.Join(dir, name+".json")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		var err error
		if doc, err = readCustomTheme(name, path); err != nil {
			return sessionexport.Theme{}, err
		}
		break
	}
	if doc == nil {
		return sessionexport.Theme{}, os.ErrNotExist
	}
	colors, err := resolveThemeDocument(doc)
	if err != nil {
		return sessionexport.Theme{}, err
	}
	keys := objectKeys(doc["colors"])
	for _, fallback := range piThemeFallbacks {
		if !contains(keys, fallback[0]) {
			keys = append(keys, fallback[0])
		}
	}
	var appearance string
	_ = json.Unmarshal(doc["appearance"], &appearance)
	t := sessionexport.Theme{Colors: resolvedThemeColors(keys, colors, appearance)}
	var exportSection, vars map[string]json.RawMessage
	_ = json.Unmarshal(doc["export"], &exportSection)
	_ = json.Unmarshal(doc["vars"], &vars)
	// Export colours end up in CSS, which takes hex and oklch() as written but
	// not okhsl().
	resolve := func(key string) string {
		raw, ok := exportSection[key]
		if !ok {
			return ""
		}
		value, err := resolveVarRef(raw, vars, map[string]bool{})
		if err != nil {
			return ""
		}
		var s string
		if json.Unmarshal(value, &s) != nil {
			if c, err := parseThemeColor(value); err == nil && c.isIndex {
				return hexOf(indexedToRgb(c.index))
			}
			return ""
		}
		if strings.HasPrefix(strings.ToLower(s), "okhsl(") {
			if c, err := parseThemeColor(value); err == nil {
				return hexOf(c.rgb)
			}
		}
		return s
	}
	t.PageBg, t.CardBg, t.InfoBg = resolve("pageBg"), resolve("cardBg"), resolve("infoBg")
	return t, nil
}

// resolvedThemeColors is Pi's Theme colors: concrete foreground then
// background tokens in the theme's order, then the terminal-default ones,
// filled from the terminal's colours (or a guess for the theme's
// appearance).
func resolvedThemeColors(keys []string, colors map[string]themeColor, appearance string) [][2]string {
	var concreteFg, concreteBg, defaultFg, defaultBg []string
	var fgL, bgL []float64
	for _, bg := range []bool{false, true} {
		for _, key := range keys {
			if piBackgroundTokens[key] != bg {
				continue
			}
			c := colors[key]
			switch {
			case c.isDef && bg:
				defaultBg = append(defaultBg, key)
			case c.isDef:
				defaultFg = append(defaultFg, key)
			case bg:
				concreteBg = append(concreteBg, key)
			default:
				concreteFg = append(concreteFg, key)
			}
			// Palette colours 0-15 follow the terminal, so they say nothing
			// about the theme's appearance.
			if !c.isDef && (!c.isIndex || c.index >= 16) {
				l := rgbToOklab(themeRGB(c))[0]
				if bg {
					bgL = append(bgL, l)
				} else {
					fgL = append(fgL, l)
				}
			}
		}
	}
	if appearance == "" {
		appearance = detectThemeAppearance(fgL, bgL)
	}
	if appearance == "" {
		appearance = piTerminal.scheme
	}
	foreground, background := rgb{0xe5, 0xe5, 0xe7}, rgb{0, 0, 0}
	if appearance == "light" {
		foreground, background = rgb{0, 0, 0}, rgb{0xff, 0xff, 0xff}
	}
	if c := piTerminal.colors.foreground; c != nil {
		foreground = *c
	}
	if c := piTerminal.colors.background; c != nil {
		background = *c
	}
	out := make([][2]string, 0, len(keys))
	for _, key := range append(concreteFg, concreteBg...) {
		out = append(out, [2]string{key, hexOf(themeRGB(colors[key]))})
	}
	for _, key := range defaultFg {
		out = append(out, [2]string{key, hexOf(foreground)})
	}
	for _, key := range defaultBg {
		out = append(out, [2]string{key, hexOf(background)})
	}
	return out
}

// detectThemeAppearance is Pi's detectAppearance from the average
// lightness of a theme's own colours.
func detectThemeAppearance(fg, bg []float64) string {
	avg := func(ls []float64) (float64, bool) {
		if len(ls) == 0 {
			return 0, false
		}
		sum := 0.0
		for _, l := range ls {
			sum += l
		}
		return sum / float64(len(ls)), true
	}
	f, okF := avg(fg)
	b, okB := avg(bg)
	switch {
	case okF && okB:
		if b < f {
			return "dark"
		}
		return "light"
	case okB:
		if b < 0.5 {
			return "dark"
		}
		return "light"
	case okF:
		if f > 0.5 {
			return "dark"
		}
		return "light"
	}
	return ""
}

func themeRGB(c themeColor) rgb {
	if c.isIndex {
		return indexedToRgb(c.index)
	}
	return c.rgb
}

// indexedToRgb is pi-tui's xterm 256-colour palette.
func indexedToRgb(index int) rgb {
	basic := [16]rgb{{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0}, {0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
		{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0}, {0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255}}
	cube := [6]uint8{0, 95, 135, 175, 215, 255}
	switch {
	case index < 16:
		return basic[index]
	case index < 232:
		i := index - 16
		return rgb{cube[i/36], cube[(i%36)/6], cube[i%6]}
	}
	gray := uint8(8 + (index-232)*10)
	return rgb{gray, gray, gray}
}

// objectKeys are a JSON object's keys in document order, as JSON.parse
// keeps them.
func objectKeys(raw json.RawMessage) []string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return keys
		}
		if key := tok.(string); !contains(keys, key) {
			keys = append(keys, key) // a repeated key keeps its first place
		}
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			return keys
		}
	}
	return keys
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}
