package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/config"
)

// Pi's custom themes (theme.js, theme-json.js): JSON files in the agent
// dir's themes/ folder, named by their "name" field and loaded by file name.
// Colours are hex (#rgb or #rrggbb), okhsl(...) or oklch(...) strings, 256
// colour indexes, "" for the terminal's default, or names of "vars".
// Golden: scripts/golden-custom-theme.mjs.

// customThemeDirs are where custom themes live: themes/ in each user
// config dir (gi's, then Pi's); the first theme of a name wins.
var customThemeDirs = func() []string {
	var dirs []string
	for _, dir := range config.UserConfigDirs() {
		dirs = append(dirs, filepath.Join(dir, "themes"))
	}
	return dirs
}

// themeColor is a resolved theme colour: an RGB value, a 256-colour index,
// or the terminal's default.
type themeColor struct {
	rgb     rgb
	index   int // -1 unless a 256-colour index
	isIndex bool
	isDef   bool
}

func (c themeColor) color() gotui.Color {
	switch {
	case c.isDef:
		return gotui.DefaultColor()
	case c.isIndex:
		return gotui.ANSIColor(uint8(c.index))
	}
	return piRGB(c.rgb.r, c.rgb.g, c.rgb.b)
}

// piThemeColorKeys are the colour tokens Pi requires of a theme
// (theme-json.js); the scrollbar, search match and thinkingMax ones are
// optional with fallbacks.
var piThemeColorKeys = []string{
	"accent", "border", "borderAccent", "borderMuted", "success", "error", "warning", "muted", "dim", "text", "thinkingText",
	"selectedBg", "userMessageBg", "userMessageText", "customMessageBg", "customMessageText", "customMessageLabel",
	"toolPendingBg", "toolSuccessBg", "toolErrorBg", "toolTitle", "toolOutput",
	"mdHeading", "mdLink", "mdLinkUrl", "mdCode", "mdCodeBlock", "mdCodeBlockBorder", "mdQuote", "mdQuoteBorder", "mdHr", "mdListBullet",
	"toolDiffAdded", "toolDiffRemoved", "toolDiffContext",
	"syntaxComment", "syntaxKeyword", "syntaxFunction", "syntaxVariable", "syntaxString", "syntaxNumber", "syntaxType", "syntaxOperator", "syntaxPunctuation",
	"thinkingOff", "thinkingMinimal", "thinkingLow", "thinkingMedium", "thinkingHigh", "thinkingXhigh",
	"bashMode",
}

var piOptionalThemeColorKeys = []string{"scrollbarTrack", "scrollbarThumb", "searchMatchBg", "searchMatchText", "thinkingMax"}

type customThemeFile struct {
	path string
	json map[string]json.RawMessage
}

// readCustomTheme parses and validates a theme file (Pi's
// parseThemeJsonContent with validateThemeJson).
func readCustomTheme(label, path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = []byte(strings.TrimPrefix(string(data), "\ufeff"))
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil || doc == nil {
		if err == nil {
			err = errors.New("not an object")
		}
		return nil, fmt.Errorf("Failed to parse theme %s: %v", label, err)
	}
	return doc, validateThemeJSON(label, doc)
}

// validateThemeJSON is Pi's validateThemeJson for the shapes gi checks:
// required colour tokens, the type of every colour value and the name.
func validateThemeJSON(label string, doc map[string]json.RawMessage) error {
	var other []string
	var name string
	if raw, ok := doc["name"]; !ok {
		other = append(other, "  - /: must have required properties name")
	} else if json.Unmarshal(raw, &name) != nil {
		other = append(other, "  - /name: must be string")
	}
	var colors map[string]json.RawMessage
	var missing []string
	if raw, ok := doc["colors"]; !ok {
		other = append(other, "  - /: must have required properties colors")
	} else if json.Unmarshal(raw, &colors) != nil || colors == nil {
		other = append(other, "  - /colors: must be object")
	} else {
		for _, key := range piThemeColorKeys {
			if _, ok := colors[key]; !ok {
				missing = append(missing, key)
			}
		}
		keys := make([]string, 0, len(colors))
		for key := range colors {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if (slices.Contains(piThemeColorKeys, key) || slices.Contains(piOptionalThemeColorKeys, key)) && !validColorValue(colors[key]) {
				other = append(other, "  - /colors/"+key+": must be string", "  - /colors/"+key+": must be integer", "  - /colors/"+key+": must match a schema in anyOf")
			}
		}
	}
	if raw, ok := doc["vars"]; ok {
		var vars map[string]json.RawMessage
		if json.Unmarshal(raw, &vars) != nil || vars == nil {
			other = append(other, "  - /vars: must be object")
		}
	}
	if len(missing) > 0 || len(other) > 0 {
		msg := fmt.Sprintf("Invalid theme %q:\n", label)
		if len(missing) > 0 {
			sort.Strings(missing)
			msg += "\nMissing required color tokens:\n  - " + strings.Join(missing, "\n  - ")
			msg += "\n\nPlease add these colors to your theme's \"colors\" object."
			msg += "\nSee the built-in themes (dark.json, light.json) for reference values."
		}
		if len(other) > 0 {
			msg += "\n\nOther errors:\n" + strings.Join(other, "\n")
		}
		return errors.New(msg)
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("Invalid theme name %q: theme names cannot contain \"/\" because it is reserved for automatic light/dark theme settings.", name)
	}
	return nil
}

// validColorValue is Pi's ColorValueSchema: a string or an integer 0-255.
func validColorValue(raw json.RawMessage) bool {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return true
	}
	var n float64
	return json.Unmarshal(raw, &n) == nil && n == float64(int(n)) && n >= 0 && n <= 255
}

// customThemeFiles are the valid custom themes by name (Pi's
// getCustomThemeInfos: invalid ones are left out).
func customThemeFiles() map[string]string {
	out := map[string]string{}
	for _, dir := range customThemeDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			doc, err := readCustomTheme(path, path)
			if err != nil {
				continue
			}
			if _, err := resolveThemeDocument(doc); err != nil {
				continue
			}
			var name string
			_ = json.Unmarshal(doc["name"], &name)
			if _, seen := out[name]; name != "" && !seen {
				out[name] = path
			}
		}
	}
	return out
}

// availablePiThemes is Pi's getAvailableThemes: system first, then the
// rest by name.
func availablePiThemes() []string {
	names := []string{systemThemeName}
	for name := range piBuiltinThemes {
		names = append(names, name)
	}
	for name := range customThemeFiles() {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		switch {
		case names[i] == systemThemeName:
			return true
		case names[j] == systemThemeName:
			return false
		}
		return names[i] < names[j]
	})
	return names
}

// loadCustomTheme is Pi's loadThemeJson and createTheme for a custom theme
// name: the file <name>.json in a themes dir.
func loadCustomTheme(name string) (map[string]themeColor, error) {
	for _, dir := range customThemeDirs() {
		path := filepath.Join(dir, name+".json")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		doc, err := readCustomTheme(name, path)
		if err != nil {
			return nil, err
		}
		return resolveThemeDocument(doc)
	}
	return nil, fmt.Errorf("Theme not found: %s", name)
}

// resolveThemeDocument is Pi's createTheme: fallbacks, vars, then colours.
func resolveThemeDocument(doc map[string]json.RawMessage) (map[string]themeColor, error) {
	var colors, vars map[string]json.RawMessage
	_ = json.Unmarshal(doc["colors"], &colors)
	if raw, ok := doc["vars"]; ok {
		_ = json.Unmarshal(raw, &vars)
	}
	fallback := map[string]string{"scrollbarTrack": "muted", "scrollbarThumb": "text", "thinkingMax": "thinkingXhigh", "searchMatchBg": "selectedBg", "searchMatchText": "text"}
	for key, from := range fallback {
		if _, ok := colors[key]; !ok {
			colors[key] = colors[from]
		}
	}
	keys := make([]string, 0, len(colors))
	for key := range colors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := map[string]themeColor{}
	// Pi resolves every var reference before it parses any colour.
	resolved := map[string]json.RawMessage{}
	for _, key := range keys {
		value, err := resolveVarRef(colors[key], vars, map[string]bool{})
		if err != nil {
			return nil, err
		}
		resolved[key] = value
	}
	for _, key := range keys {
		c, err := parseThemeColor(resolved[key])
		if err != nil {
			return nil, err
		}
		out[key] = c
	}
	return out, nil
}

// resolveVarRef is Pi's resolveVarRefs.
func resolveVarRef(value json.RawMessage, vars map[string]json.RawMessage, visited map[string]bool) (json.RawMessage, error) {
	var s string
	if json.Unmarshal(value, &s) != nil || s == "" || strings.HasPrefix(s, "#") || okColorPrefix.MatchString(s) {
		return value, nil
	}
	if visited[s] {
		return nil, fmt.Errorf("Circular variable reference detected: %s", s)
	}
	next, ok := vars[s]
	if !ok {
		return nil, fmt.Errorf("Variable reference not found: %s", s)
	}
	visited[s] = true
	return resolveVarRef(next, vars, visited)
}

var (
	okColorPrefix = regexp.MustCompile(`(?i)^ok(lch|hsl)\(`)
	numberPattern = `[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:e[+-]?\d+)?`
	hexPattern    = regexp.MustCompile(`(?i)^#([\da-f]{3}|[\da-f]{6})$`)
	oklchPattern  = regexp.MustCompile(`(?i)^oklch\(\s*(` + numberPattern + `)(%)?\s+(` + numberPattern + `)\s+(` + numberPattern + `)(?:deg)?\s*\)$`)
	okhslPattern  = regexp.MustCompile(`(?i)^okhsl\(\s*(` + numberPattern + `)(?:deg)?\s+(` + numberPattern + `)(%)?\s+(` + numberPattern + `)(%)?\s*\)$`)
)

// parseThemeColor is pi-tui's parseColor, with "" as the default colour.
func parseThemeColor(raw json.RawMessage) (themeColor, error) {
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return themeColor{index: int(n), isIndex: true}, nil
	}
	var s string
	_ = json.Unmarshal(raw, &s)
	if s == "" {
		return themeColor{isDef: true}, nil
	}
	num := func(v string) float64 { f, _ := strconv.ParseFloat(v, 64); return f }
	if m := hexPattern.FindStringSubmatch(s); m != nil {
		digits := m[1]
		if len(digits) == 3 {
			digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
		}
		v, _ := strconv.ParseUint(digits, 16, 32)
		return themeColor{rgb: rgb{uint8(v >> 16), uint8(v >> 8), uint8(v)}}, nil
	}
	if m := oklchPattern.FindStringSubmatch(s); m != nil {
		l := num(m[1])
		if m[2] != "" {
			l /= 100
		}
		h := num(m[4])
		h = mathMod(h, 360)
		return themeColor{rgb: oklchToRgb(oklch{l: l, c: num(m[3]), h: h})}, nil
	}
	if m := okhslPattern.FindStringSubmatch(s); m != nil {
		sat, light := num(m[2]), num(m[4])
		if m[3] != "" {
			sat /= 100
		}
		if m[5] != "" {
			light /= 100
		}
		return themeColor{rgb: okhslToRgb(num(m[1]), sat, light)}, nil
	}
	return themeColor{}, fmt.Errorf("Invalid color value: %s", s)
}

func mathMod(x, m float64) float64 {
	r := x - m*float64(int(x/m))
	if r < 0 {
		r += m
	}
	return r
}
