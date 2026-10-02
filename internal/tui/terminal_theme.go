package tui

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	gotui "github.com/grindlemire/go-tui"
)

// Terminal theme detection and Pi theme selection (issues #12, #31), ported
// from Pi's theme.js / system-theme.js and pi-tui's tui.js / terminal-colors.js:
//
//  1. Ask the terminal for its foreground/background (OSC 10/11), its 16
//     palette colours (OSC 4) and colour scheme (CSI ?996n), terminated by a
//     DA1 query every terminal answers.
//  2. A reported background decides light/dark (Pi's terminalAppearance);
//     otherwise the scheme report, then COLORFGBG, then dark.
//  3. Pi's theme setting picks the theme: a name, or a "light/dark" pair
//     resolved by the detected scheme. Unset or unknown: Pi's default
//     "system" theme, generated from the reported colours (system_theme.go).
//     Without a reported background Pi renders ANSI palette indices with
//     faint neutrals; gi's palette has no per-token attributes, so it uses
//     the detected scheme's built-in theme there.

type rgb struct{ r, g, b uint8 }

// terminalColors is what the terminal reported; nil fields were not reported.
type terminalColors struct {
	foreground, background *rgb
	palette                []rgb  // ANSI 0-15 when all 16 were reported (OSC 4)
	scheme                 string // "dark", "light" or "" (CSI ?997;{1,2}n)
}

// piThemeQueryTimeout matches Pi's TERMINAL_QUERY_TIMEOUT_MS order of
// magnitude; replies normally arrive in a few milliseconds.
const piThemeQueryTimeout = 150 * time.Millisecond

// terminalThemeQuery is Pi's TERMINAL_COLOR_QUERY (OSC 10, 11 and OSC 4 for
// colours 0-15) plus the colour scheme report, then DA1.
var terminalThemeQuery = func() string {
	q := "\x1b]10;?\x07\x1b]11;?\x07"
	for i := 0; i < 16; i++ {
		q += "\x1b]4;" + strconv.Itoa(i) + ";?\x07"
	}
	return q + "\x1b[?996n\x1b[c"
}()

var (
	oscColorReply    = regexp.MustCompile(`(?i)\x1b\](?:(1[01])|4;(\d{1,3}));([^\x07\x1b]*)(?:\x07|\x1b\\)`)
	schemeReport     = regexp.MustCompile(`\x1b\[\?997;([12])n`)
	deviceAttributes = regexp.MustCompile(`\x1b\[\?[\d;]*c`)
)

// parseTerminalReplies extracts OSC 10/11 colours and the scheme report.
func parseTerminalReplies(data string) terminalColors {
	var out terminalColors
	palette := map[int]rgb{}
	for _, m := range oscColorReply.FindAllStringSubmatch(data, -1) {
		c, ok := parseOSCColor(m[3])
		if !ok {
			continue
		}
		switch m[1] {
		case "10":
			out.foreground = &c
		case "11":
			out.background = &c
		default:
			if index, err := strconv.Atoi(m[2]); err == nil && index < 16 {
				palette[index] = c
			}
		}
	}
	if len(palette) == 16 {
		for i := 0; i < 16; i++ {
			out.palette = append(out.palette, palette[i])
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
		if !strings.HasPrefix(lower, "rgb:") && !strings.HasPrefix(lower, "rgba:") {
			return rgb{}, false
		}
		want := 3
		if strings.HasPrefix(lower, "rgba:") {
			want = 4
		}
		lower = strings.TrimPrefix(strings.TrimPrefix(lower, "rgba:"), "rgb:")
		parts = strings.Split(lower, "/")
		if len(parts) != want {
			return rgb{}, false
		}
		if want == 4 {
			if _, ok := channel(parts[3]); !ok {
				return rgb{}, false
			}
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

// oklabLightness is OKLab L of an sRGB colour (pi-tui's constants).
func oklabLightness(c rgb) float64 { return rgbToOklab(c)[0] }

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
	if index < 0 {
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

// selectPiTheme picks the theme for a setting and detected scheme: a
// built-in name, or Pi's default "system" when unset or unknown (custom Pi
// themes are not loaded yet; Pi also falls back to system for invalid ones).
func selectPiTheme(setting, terminalTheme string) string {
	if name := resolveThemeSetting(setting, terminalTheme); name != "" {
		if _, ok := piBuiltinThemes[name]; ok {
			return name
		}
	}
	return systemThemeName
}

// piActiveTheme is the applied theme name.
var piActiveTheme = "dark"

// applyPiTheme sets gi's colour palette from a built-in Pi theme.
func applyPiTheme(name string) bool {
	t, ok := piBuiltinThemes[name]
	if !ok {
		return false
	}
	applyPiThemeColors(name, func(key string) gotui.Color {
		v := t[key]
		return piRGB(v[0], v[1], v[2])
	})
	return true
}

// applySystemTheme applies Pi's system theme generated from the terminal's
// colours: "" tokens use the terminal's own colour. ok is false without a
// reported background.
func applySystemTheme(colors terminalColors) bool {
	generated, _, ok := generateSystemThemeColors(systemThemeInput{background: colors.background, foreground: colors.foreground, palette: colors.palette, saturation: 1})
	if !ok {
		return false
	}
	applyPiThemeColors(systemThemeName, func(key string) gotui.Color {
		c, ok := parseOSCColor(generated[key])
		if !ok {
			return gotui.DefaultColor()
		}
		return piRGB(c.r, c.g, c.b)
	})
	return true
}

// applyPiThemeColors sets every palette role from a theme's colour keys.
func applyPiThemeColors(name string, c func(key string) gotui.Color) {
	piText, piMuted, piDim, piAccent = c("text"), c("muted"), c("dim"), c("accent")
	piError, piWarning, piSuccess = c("error"), c("warning"), c("success")
	piThinkingText, piUserBg, piSearchMatchBg, piSelectedBg = c("thinkingText"), c("userMessageBg"), c("searchMatchBg"), c("selectedBg")
	piMdCode, piMdLink, piBashMode = c("mdCode"), c("mdLink"), c("bashMode")
	piBorder, piBorderMuted = c("border"), c("borderMuted")
	piThinkingOff, piThinkingMinimal, piThinkingLow = c("thinkingOff"), c("thinkingMinimal"), c("thinkingLow")
	piThinkingMedium, piThinkingHigh, piThinkingXhigh, piThinkingMax = c("thinkingMedium"), c("thinkingHigh"), c("thinkingXhigh"), c("thinkingMax")
	piSyntaxKeyword, piSyntaxFunction, piSyntaxVariable = c("syntaxKeyword"), c("syntaxFunction"), c("syntaxVariable")
	piSyntaxString, piSyntaxNumber, piSyntaxType, piSyntaxComment = c("syntaxString"), c("syntaxNumber"), c("syntaxType"), c("syntaxComment")
	piToolPendingBg, piToolSuccessBg, piToolErrorBg = c("toolPendingBg"), c("toolSuccessBg"), c("toolErrorBg")
	piMdHeading, piMdCodeBlock = c("mdHeading"), c("mdCodeBlock")
	piMdCodeBorder, piMdQuote, piMdQuoteBorder = c("mdCodeBlockBorder"), c("mdQuote"), c("mdQuoteBorder")
	piMdHr, piMdLinkUrl, piMdListBullet = c("mdHr"), c("mdLinkUrl"), c("mdListBullet")
	piToolTitle, piToolOutput = c("toolTitle"), c("toolOutput")
	piDiffAdded, piDiffRemoved, piDiffContext = c("toolDiffAdded"), c("toolDiffRemoved"), c("toolDiffContext")
	piUserText, piCustomLabel, piCustomText = c("userMessageText"), c("customMessageLabel"), c("customMessageText")
	piActiveTheme = name
}

// initPiTheme detects the terminal scheme and applies the configured theme.
// It runs before the UI owns the terminal.
func initPiTheme(setting string) string {
	colors := terminalColors{}
	if os.Getenv("GI_TUI_NO_THEME_QUERY") == "" {
		colors = queryTerminalColors(piThemeQueryTimeout)
	}
	scheme := detectTerminalTheme(colors, os.Getenv("COLORFGBG"))
	name := selectPiTheme(setting, scheme)
	if name == systemThemeName {
		if applySystemTheme(colors) {
			return name
		}
		name = "dark" // no reported background: the scheme's built-in theme
		if scheme == "light" {
			name = "light"
		}
	}
	applyPiTheme(name)
	return name
}
