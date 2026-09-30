package tui

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"golang.org/x/term"
)

// Terminal theme detection and Pi theme selection (issue #12), ported from
// Pi's theme.js / system-theme.js and pi-tui terminal-colors.js:
//
//  1. Ask the terminal for its foreground/background (OSC 10/11) and colour
//     scheme (CSI ?996n), terminated by a DA1 query every terminal answers.
//  2. A reported background decides light/dark (Pi's terminalAppearance);
//     otherwise the scheme report, then COLORFGBG, then dark.
//  3. Pi's theme setting picks the theme: a name, or a "light/dark" pair
//     resolved by the detected scheme. Unset: the detected scheme's built-in
//     theme (Pi derives a palette-based "system" theme there; not yet ported).

type rgb struct{ r, g, b uint8 }

// terminalColors is what the terminal reported; nil fields were not reported.
type terminalColors struct {
	foreground, background *rgb
	scheme                 string // "dark", "light" or "" (CSI ?997;{1,2}n)
}

// piThemeQueryTimeout matches Pi's TERMINAL_QUERY_TIMEOUT_MS order of
// magnitude; replies normally arrive in a few milliseconds.
const piThemeQueryTimeout = 150 * time.Millisecond

const terminalThemeQuery = "\x1b]10;?\x07\x1b]11;?\x07\x1b[?996n\x1b[c"

var (
	oscColorReply    = regexp.MustCompile(`\x1b\](1[01]);([^\x07\x1b]*)(?:\x07|\x1b\\)`)
	schemeReport     = regexp.MustCompile(`\x1b\[\?997;([12])n`)
	deviceAttributes = regexp.MustCompile(`\x1b\[\?[\d;]*c`)
)

// queryTerminalColors asks the controlling terminal for its colours before
// the UI takes over input. It never blocks longer than timeout and reports
// nothing when there is no terminal or it does not answer.
func queryTerminalColors(timeout time.Duration) terminalColors {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return terminalColors{}
	}
	defer tty.Close()
	fd := int(tty.Fd())
	if !term.IsTerminal(fd) {
		return terminalColors{}
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return terminalColors{}
	}
	defer term.Restore(fd, state)
	if _, err := tty.WriteString(terminalThemeQuery); err != nil {
		return terminalColors{}
	}
	deadline := time.Now().Add(timeout)
	var buf []byte
	chunk := make([]byte, 256)
	for time.Now().Before(deadline) {
		if err := tty.SetReadDeadline(deadline); err != nil {
			break // not pollable: give up rather than block
		}
		n, err := tty.Read(chunk)
		buf = append(buf, chunk[:n]...)
		if deviceAttributes.Match(buf) || err != nil {
			break // DA1 answers last: every earlier reply has arrived
		}
	}
	return parseTerminalReplies(string(buf))
}

// parseTerminalReplies extracts OSC 10/11 colours and the scheme report.
func parseTerminalReplies(data string) terminalColors {
	var out terminalColors
	for _, m := range oscColorReply.FindAllStringSubmatch(data, -1) {
		c, ok := parseOSCColor(m[2])
		if !ok {
			continue
		}
		if m[1] == "10" {
			out.foreground = &c
		} else {
			out.background = &c
		}
	}
	if m := schemeReport.FindStringSubmatch(data); m != nil {
		out.scheme = map[string]string{"1": "dark", "2": "light"}[m[1]]
	}
	return out
}

// parseOSCColor ports pi-tui parseOscColorValue: #rrggbb, #rrrrggggbbbb, or
// rgb:/rgba: with 1-4 hex digits per channel.
func parseOSCColor(raw string) (rgb, bool) {
	value := strings.TrimSpace(raw)
	channel := func(hex string) (uint8, bool) {
		if hex == "" || len(hex) > 4 {
			return 0, false
		}
		v, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return 0, false
		}
		max := math.Pow(16, float64(len(hex))) - 1
		return uint8(math.Round(float64(v) / max * 255)), true
	}
	var parts []string
	if strings.HasPrefix(value, "#") {
		hex := value[1:]
		switch len(hex) {
		case 6:
			parts = []string{hex[0:2], hex[2:4], hex[4:6]}
		case 12:
			parts = []string{hex[0:4], hex[4:8], hex[8:12]}
		default:
			return rgb{}, false
		}
	} else {
		lower := strings.ToLower(value)
		lower = strings.TrimPrefix(strings.TrimPrefix(lower, "rgba:"), "rgb:")
		parts = strings.Split(lower, "/")
		if len(parts) < 3 {
			return rgb{}, false
		}
		parts = parts[:3]
	}
	r, ok1 := channel(parts[0])
	g, ok2 := channel(parts[1])
	b, ok3 := channel(parts[2])
	return rgb{r, g, b}, ok1 && ok2 && ok3
}

func relativeLuminance(c rgb) float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.r) + 0.7152*lin(c.g) + 0.0722*lin(c.b)
}

func wcagContrast(a, b rgb) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

// oklabLightness is OKLab L of an sRGB colour.
func oklabLightness(c rgb) float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	r, g, b := lin(c.r), lin(c.g), lin(c.b)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
}

// terminalAppearance ports Pi's system-theme terminalAppearance: the
// direction of the terminal's own foreground when text can be readable that
// way, otherwise dark when white text contrasts more than black.
func terminalAppearance(background rgb, foreground *rgb) string {
	white, black := rgb{255, 255, 255}, rgb{0, 0, 0}
	whiteContrast, blackContrast := wcagContrast(white, background), wcagContrast(black, background)
	if foreground != nil {
		fl, bl := oklabLightness(*foreground), oklabLightness(background)
		if math.Abs(fl-bl) > 0.05 {
			appearance, best := "light", blackContrast
			if fl > bl {
				appearance, best = "dark", whiteContrast
			}
			if best >= 4.5 {
				return appearance
			}
		}
	}
	if whiteContrast >= blackContrast {
		return "dark"
	}
	return "light"
}

// colorFgBgTheme classifies COLORFGBG's background index like Pi/Vim:
// 0-6 and 8 dark, 7 and 9-15 light.
func colorFgBgTheme(value string) string {
	fields := strings.Split(value, ";")
	bg := strings.TrimSpace(fields[len(fields)-1])
	if len(bg) == 0 || len(bg) > 2 {
		return ""
	}
	index, err := strconv.Atoi(bg)
	if err != nil || index > 15 {
		return ""
	}
	if index <= 6 || index == 8 {
		return "dark"
	}
	return "light"
}

// detectTerminalTheme ports Pi's detectTerminalTheme.
func detectTerminalTheme(colors terminalColors, colorFgBg string) string {
	if colors.background != nil {
		return terminalAppearance(*colors.background, colors.foreground)
	}
	if colors.scheme != "" {
		return colors.scheme
	}
	if t := colorFgBgTheme(colorFgBg); t != "" {
		return t
	}
	return "dark"
}

// resolveThemeSetting ports Pi's resolveThemeSetting; "" means unset/invalid.
func resolveThemeSetting(setting, terminalTheme string) string {
	setting = strings.TrimSpace(setting)
	if i := strings.Index(setting, "/"); i >= 0 {
		light, dark := strings.TrimSpace(setting[:i]), strings.TrimSpace(setting[i+1:])
		if strings.Contains(dark, "/") || light == "" || dark == "" {
			return ""
		}
		if terminalTheme == "light" {
			return light
		}
		return dark
	}
	return setting
}

// selectPiTheme picks the built-in theme for a setting and detected scheme.
// Unknown names (custom Pi themes are not loaded yet) fall back to the
// detected scheme's built-in theme.
func selectPiTheme(setting, terminalTheme string) string {
	if name := resolveThemeSetting(setting, terminalTheme); name != "" {
		if _, ok := piBuiltinThemes[name]; ok {
			return name
		}
	}
	if terminalTheme == "light" {
		return "light"
	}
	return "dark"
}

// piActiveTheme is the applied built-in theme name.
var piActiveTheme = "dark"

// applyPiTheme sets gi's colour palette from a built-in Pi theme.
func applyPiTheme(name string) bool {
	t, ok := piBuiltinThemes[name]
	if !ok {
		return false
	}
	c := func(key string) gotui.Color {
		v := t[key]
		return piRGB(v[0], v[1], v[2])
	}
	piText, piMuted, piDim, piAccent = c("text"), c("muted"), c("dim"), c("accent")
	piError, piWarning, piSuccess = c("error"), c("warning"), c("success")
	piThinkingText, piUserBg, piSearchMatchBg = c("thinkingText"), c("userMessageBg"), c("searchMatchBg")
	piMdCode, piMdLink, piBashMode = c("mdCode"), c("mdLink"), c("bashMode")
	piBorder, piBorderMuted = c("border"), c("borderMuted")
	piThinkingOff, piThinkingMinimal, piThinkingLow = c("thinkingOff"), c("thinkingMinimal"), c("thinkingLow")
	piThinkingMedium, piThinkingHigh, piThinkingXhigh, piThinkingMax = c("thinkingMedium"), c("thinkingHigh"), c("thinkingXhigh"), c("thinkingMax")
	piSyntaxKeyword, piSyntaxFunction, piSyntaxVariable = c("syntaxKeyword"), c("syntaxFunction"), c("syntaxVariable")
	piSyntaxString, piSyntaxNumber, piSyntaxType, piSyntaxComment = c("syntaxString"), c("syntaxNumber"), c("syntaxType"), c("syntaxComment")
	piToolPendingBg, piToolSuccessBg, piToolErrorBg = c("toolPendingBg"), c("toolSuccessBg"), c("toolErrorBg")
	piActiveTheme = name
	return true
}

// initPiTheme detects the terminal scheme and applies the configured theme.
// It runs before the UI owns the terminal.
func initPiTheme(setting string) string {
	colors := terminalColors{}
	if os.Getenv("GI_TUI_NO_THEME_QUERY") == "" {
		colors = queryTerminalColors(piThemeQueryTimeout)
	}
	name := selectPiTheme(setting, detectTerminalTheme(colors, os.Getenv("COLORFGBG")))
	applyPiTheme(name)
	return name
}
