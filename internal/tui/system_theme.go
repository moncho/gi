package tui

import (
	"fmt"
	"math"
)

// Pi's "system" theme (pi-coding-agent modes/interactive/theme/
// system-theme.js): its colours derived from the terminal's own background,
// foreground and 16-colour palette. Every token belongs to a colour family
// and must reach a contrast level on the surfaces it is drawn on. Hue and
// saturation come from the palette (or the family), lightness from the
// rules; a palette colour never gains OKLCH chroma at another lightness, so
// pastel palettes such as Catppuccin Frappe stay pastel (Pi 1.0, #10255).

const systemThemeName = "system"

type colorFamily struct {
	hue      float64
	min, max float64 // saturation range
	slot     int     // ANSI palette slot
}

var systemFamilies = map[string]colorFamily{
	"neutral":            {231.49, 0.02, 0.08, 8},
	"blue":               {231.49, 0.1, 0.68, 4},
	"green":              {158.68, 0.1, 0.76, 2},
	"red":                {20, 0.1, 0.92, 1},
	"yellow":             {82.36, 0.5, 1, 3},
	"orange":             {52, 0.12, 0.85, 3},
	"violet":             {295, 0.2, 0.6, 5},
	"calamine":           {202.43, 0.1, 0.74, 6},
	"thinkingSlate":      {231.49, 0.08, 0.2, 4},
	"thinkingBlue":       {231.49, 0.2, 0.45, 4},
	"thinkingPeriwinkle": {263.25, 0.3, 0.6, 6},
	"thinkingViolet":     {295, 0.4, 0.75, 5},
	"thinkingMagenta":    {337.5, 0.5, 0.85, 13},
	"thinkingRed":        {20, 0.95, 1, 1},
}

// systemTokens is TOKEN_FAMILIES in Pi's order.
var systemTokens = [][2]string{
	{"selectedBg", "blue"}, {"searchMatchBg", "orange"}, {"userMessageBg", "blue"}, {"customMessageBg", "violet"},
	{"toolPendingBg", "neutral"}, {"toolSuccessBg", "green"}, {"toolErrorBg", "red"},
	{"text", "neutral"}, {"userMessageText", "neutral"}, {"customMessageText", "neutral"}, {"toolTitle", "neutral"},
	{"syntaxOperator", "neutral"}, {"syntaxPunctuation", "neutral"}, {"muted", "neutral"}, {"dim", "neutral"},
	{"thinkingText", "neutral"}, {"toolOutput", "neutral"}, {"mdLinkUrl", "neutral"}, {"mdQuote", "neutral"},
	{"mdQuoteBorder", "neutral"}, {"mdHr", "neutral"}, {"mdCodeBlockBorder", "neutral"}, {"toolDiffContext", "neutral"},
	{"syntaxComment", "neutral"}, {"scrollbarTrack", "neutral"}, {"scrollbarThumb", "neutral"}, {"searchMatchText", "neutral"},
	{"borderMuted", "neutral"}, {"accent", "violet"}, {"borderAccent", "violet"}, {"customMessageLabel", "violet"},
	{"mdCode", "violet"}, {"mdListBullet", "violet"}, {"syntaxType", "violet"}, {"border", "blue"}, {"mdLink", "blue"},
	{"syntaxKeyword", "blue"}, {"syntaxVariable", "calamine"}, {"success", "green"}, {"mdCodeBlock", "green"},
	{"toolDiffAdded", "green"}, {"bashMode", "green"}, {"syntaxNumber", "green"}, {"error", "red"},
	{"toolDiffRemoved", "red"}, {"warning", "yellow"}, {"mdHeading", "yellow"}, {"syntaxFunction", "yellow"},
	{"syntaxString", "orange"}, {"thinkingOff", "neutral"}, {"thinkingMinimal", "thinkingSlate"},
	{"thinkingLow", "thinkingBlue"}, {"thinkingMedium", "thinkingPeriwinkle"}, {"thinkingHigh", "thinkingViolet"},
	{"thinkingXhigh", "thinkingMagenta"}, {"thinkingMax", "thinkingRed"},
}

var systemTokenFamily = func() map[string]string {
	m := map[string]string{}
	for _, t := range systemTokens {
		m[t[0]] = t[1]
	}
	return m
}()

// systemTokenSlots: palette slots for tokens that would share a hue.
var systemTokenSlots = map[string]int{"syntaxString": 2, "syntaxNumber": 5, "searchMatchBg": 3}

type levelCurve struct {
	coefficients [6]float64
	lo, hi       float64 // reachable surface lightness
}

var systemLevels = map[string]map[string]levelCurve{
	"panel": {"dark": {[6]float64{0.29131, -0.39746, 2.33185, -0.85524, -1.2076, 0.86276}, 0, 0.979}, "light": {[6]float64{-3.74073, 27.94549, -78.44258, 112.6798, -79.60015, 22.11277}, 0.348, 1}},
	"track": {"dark": {[6]float64{0.39028, -0.23015, 0.83573, 2.43829, -4.38292, 2.01582}, 0, 0.946}, "light": {[6]float64{-5.24921, 38.37322, -107.28833, 152.10005, -106.17127, 29.18061}, 0.368, 1}},
	"thinking0": {"dark": {[6]float64{0.52988, -0.05809, -0.30924, 4.63567, -6.52933, 2.89108}, 0, 0.873}, "light": {[6]float64{-28.27749, 182.85284, -469.62416, 603.15916, -384.59976, 97.35147}, 0.51, 1}},
	"thinking1": {"dark": {[6]float64{0.55278, -0.03667, -0.45659, 4.95347, -6.90265, 3.0706}, 0, 0.858}, "light": {[6]float64{-37.10484, 235.86282, -596.62344, 754.3633, -474.00763, 118.3551}, 0.535, 1}},
	"thinking2": {"dark": {[6]float64{0.57486, -0.01765, -0.58987, 5.25227, -7.27175, 3.25532}, 0, 0.842}, "light": {[6]float64{-59.89653, 377.05024, -945.07843, 1182.03145, -734.96375, 181.68658}, 0.556, 1}},
	"thinking3": {"dark": {[6]float64{0.59621, -0.00062, -0.71148, 5.53588, -7.6392, 3.44606}, 0, 0.827}, "light": {[6]float64{-72.07122, 445.84082, -1099.57352, 1353.88793, -829.53392, 202.26164}, 0.58, 1}},
	"thinking4": {"dark": {[6]float64{0.61691, 0.01462, -0.82288, 5.80651, -8.00641, 3.64333}, 0, 0.811}, "light": {[6]float64{-110.14338, 674.21488, -1645.75941, 2004.32367, -1215.15899, 293.3183}, 0.6, 1}},
	"thinking5": {"dark": {[6]float64{0.63702, 0.02826, -0.92498, 6.06465, -8.37246, 3.84651}, 0, 0.795}, "light": {[6]float64{-175.47701, 1063.54495, -2570.70594, 3098.80776, -1860.15527, 444.76392}, 0.62, 1}},
	"thinking6": {"dark": {[6]float64{0.65658, 0.04044, -1.01835, 6.30989, -8.73529, 4.05439}, 0, 0.779}, "light": {[6]float64{-183.81712, 1094.70055, -2602.68539, 3088.71276, -1826.91131, 430.75931}, 0.643, 1}},
	"subtle": {"dark": {[6]float64{0.56762, -0.02475, -0.5383, 5.12628, -7.10931, 3.17324}, 0, 0.848}, "light": {[6]float64{-232.85459, 1376.54473, -3249.11801, 3827.91186, -2248.29472, 526.55751}, 0.657, 1}},
	"thumb": {"dark": {[6]float64{0.60323, 0.00278, -0.73328, 5.57157, -7.68067, 3.46933}, 0, 0.823}, "light": {[6]float64{-82.89897, 511.01355, -1255.98095, 1540.76821, -940.68087, 228.58523}, 0.586, 1}},
	"readable": {"dark": {[6]float64{0.66937, 0.04704, -1.06871, 6.43941, -8.9332, 4.17229}, 0, 0.77}, "light": {[6]float64{-1554.52576, 8733.56817, -19604.93507, 21977.72696, -12300.99599, 2749.81288}, 0.751, 1}},
	"emphasis": {"dark": {[6]float64{0.7303, 0.07695, -1.31626, 7.1681, -10.14436, 4.92846}, 0, 0.712}, "light": {[6]float64{-4948.31942, 26870.91986, -58334.48399, 63280.17197, -34298.01053, 7430.30146}, 0.811, 1}},
	"textOnPanel": {"dark": {[6]float64{0.86713, 0.05232, -0.89428, 4.79014, -5.5432, 1.75023}, 0, 0.542}, "light": {[6]float64{-8570.89457, 43954.60805, -90084.00702, 92220.6791, -47152.15802, 9632.27113}, 0.867, 1}},
	"text": {"dark": {[6]float64{0.89242, 0.02311, -0.44862, 2.34417, -0.06084, -2.63844}, 0, 0.5}, "light": {[6]float64{-2004.67048, 6664.47299, -6060.70202, -1792.61209, 5133.82359, -1939.85583}, 0.894, 1}},
}

var (
	systemToolPanels    = []string{"toolPendingBg", "toolSuccessBg", "toolErrorBg"}
	systemMessagePanels = []string{"userMessageBg", "customMessageBg"}
	systemPanels        = []string{"userMessageBg", "toolPendingBg", "toolSuccessBg", "toolErrorBg", "selectedBg", "searchMatchBg", "customMessageBg"}
)

type systemRule struct {
	token string
	on    []string
	level string
}

func cat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

var systemRules = func() []systemRule {
	var rules []systemRule
	each := func(tokens, on []string, level string) {
		for _, t := range tokens {
			rules = append(rules, systemRule{t, on, level})
		}
	}
	bg := []string{"background"}
	each(systemPanels, bg, "panel")
	rules = append(rules,
		systemRule{"text", bg, "text"},
		systemRule{"text", []string{"selectedBg"}, "textOnPanel"},
		systemRule{"userMessageText", []string{"userMessageBg"}, "textOnPanel"},
		systemRule{"toolTitle", systemToolPanels, "textOnPanel"})
	each([]string{"accent", "success", "error", "warning"}, cat(bg, []string{"selectedBg"}, systemToolPanels), "readable")
	rules = append(rules,
		systemRule{"muted", cat(bg, []string{"selectedBg", "customMessageBg"}, systemToolPanels), "readable"},
		systemRule{"dim", cat(bg, []string{"selectedBg", "customMessageBg"}, systemToolPanels), "subtle"},
		systemRule{"thinkingText", bg, "readable"},
		systemRule{"customMessageText", cat([]string{"customMessageBg"}, systemToolPanels), "readable"},
		systemRule{"customMessageLabel", cat(bg, []string{"customMessageBg", "selectedBg"}, systemToolPanels), "readable"},
		systemRule{"toolOutput", cat(bg, systemToolPanels), "readable"})
	each([]string{"mdHeading", "mdLink", "mdLinkUrl", "mdCode", "mdQuote", "mdCodeBlockBorder", "mdListBullet"}, cat(bg, systemMessagePanels), "readable")
	rules = append(rules, systemRule{"mdCodeBlock", cat(bg, systemMessagePanels, systemToolPanels), "readable"})
	each([]string{"toolDiffAdded", "toolDiffRemoved", "toolDiffContext"}, cat(bg, systemToolPanels), "readable")
	each([]string{"syntaxComment", "syntaxKeyword", "syntaxFunction", "syntaxVariable", "syntaxString", "syntaxNumber", "syntaxType", "syntaxOperator", "syntaxPunctuation"}, cat(bg, systemMessagePanels, systemToolPanels), "readable")
	rules = append(rules, systemRule{"searchMatchText", []string{"searchMatchBg"}, "readable"})
	each([]string{"bashMode", "border", "borderAccent"}, bg, "readable")
	rules = append(rules, systemRule{"borderMuted", bg, "subtle"})
	each([]string{"mdQuoteBorder", "mdHr"}, cat(bg, systemMessagePanels, systemToolPanels), "readable")
	rules = append(rules, systemRule{"scrollbarTrack", bg, "track"}, systemRule{"scrollbarThumb", []string{"scrollbarTrack"}, "thumb"})
	for i, t := range []string{"thinkingOff", "thinkingMinimal", "thinkingLow", "thinkingMedium", "thinkingHigh", "thinkingXhigh", "thinkingMax"} {
		rules = append(rules, systemRule{t, bg, fmt.Sprintf("thinking%d", i)})
	}
	return rules
}()

var systemReadableFloor = map[string]string{"dark": "readable", "light": "subtle"}

const (
	systemForegroundLevel  = "emphasis"
	systemTextMinimumWCAG  = 4.5
	systemRelaxationRounds = 20
)

var systemForegroundTokens = []string{"text", "userMessageText", "toolTitle"}

// systemSolveOrder lists tokens with every surface before the tokens on it.
var systemSolveOrder = func() []string {
	var order []string
	seen := map[string]bool{}
	var visit func(string)
	visit = func(token string) {
		if seen[token] {
			return
		}
		for _, r := range systemRules {
			if r.token != token {
				continue
			}
			for _, s := range r.on {
				if s != "background" {
					visit(s)
				}
			}
		}
		seen[token] = true
		order = append(order, token)
	}
	for _, r := range systemRules {
		visit(r.token)
	}
	return order
}()


func bellWeight(l float64) float64 {
	g := func(x float64) float64 { return math.Exp(-((x - 0.5) * (x - 0.5)) / (2 * 0.25 * 0.25)) }
	return (g(l) - g(0)) / (1 - g(0))
}

func saturationCurve(f colorFamily, l float64) float64 {
	floor := 1.0
	if f.max > 0 {
		floor = f.min / f.max
	}
	return floor + (1-floor)*bellWeight(l)
}

func levelTarget(level, appearance string, surfaceL float64) (float64, bool) {
	c := systemLevels[level][appearance]
	if surfaceL < c.lo || surfaceL > c.hi {
		return 0, false
	}
	sum := 0.0
	for p, k := range c.coefficients {
		sum += k * math.Pow(surfaceL, float64(p))
	}
	return sum, true
}

func hexOf(c rgb) string { return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b) }

type colorSource struct {
	okhsl
	chroma float64
}

func sourceOf(c rgb) colorSource { return colorSource{rgbToOkhsl(c), rgbToOklch(c).c} }

// anchored is a source colour's hue at another OKHSL lightness: its
// saturation falls off along the family's curve, and its OKLCH chroma never
// exceeds the source's (Pi: pastel palettes stay pastel).
func anchored(src colorSource, f colorFamily, lightness, saturation float64) rgb {
	anchor := saturationCurve(f, src.l)
	falloff := 1.0
	if anchor > 0 {
		falloff = math.Min(1, saturationCurve(f, lightness)/anchor)
	}
	color := okhslToRgb(src.h, src.s*falloff*saturation, lightness)
	limit := src.chroma * falloff * saturation
	lc := rgbToOklch(color)
	if lc.c <= limit {
		return color
	}
	return oklchToRgb(oklch{lc.l, limit, src.h})
}

func withTextContrast(color rgb, surfaces []rgb, lighter bool) rgb {
	meets := func(c rgb) bool {
		for _, s := range surfaces {
			if wcagContrast(c, s) < systemTextMinimumWCAG {
				return false
			}
		}
		return true
	}
	if meets(color) {
		return color
	}
	o := rgbToOkhsl(color)
	at := func(l float64) rgb { return okhslToRgb(o.h, o.s, l) }
	extreme := 0.0
	if lighter {
		extreme = 1
	}
	if !meets(at(extreme)) {
		return at(extreme)
	}
	low, high := o.l, extreme
	for i := 0; i < 20; i++ {
		mid := (low + high) / 2
		if meets(at(mid)) {
			high = mid
		} else {
			low = mid
		}
	}
	return at(high)
}

// systemThemeInput is what the terminal reported.
type systemThemeInput struct {
	background, foreground *rgb
	palette                []rgb // 16 colours, or nil
	saturation             float64
}

// generateSystemThemeColors ports Pi's generator for a reported
// background; tokens map to "#rrggbb", or "" for the terminal's own
// foreground. ok is false without a background (Pi then uses palette
// indices; gi keeps its built-in theme).
func generateSystemThemeColors(in systemThemeInput) (map[string]string, string, bool) {
	if in.background == nil {
		return nil, "", false
	}
	sat := math.Min(1, math.Max(0, in.saturation))
	background := *in.background
	var palette []colorSource
	if len(in.palette) == 16 {
		for _, c := range in.palette {
			palette = append(palette, sourceOf(c))
		}
	}
	appearance := terminalAppearance(background, in.foreground)
	lighter := appearance == "dark"
	extreme := 0.0
	if lighter {
		extreme = 1
	}
	backgroundL := oklabLightness(background)

	paint := func(token string, oklabL float64) rgb {
		lightness := oklabToOkhslLightness(oklabL)
		f := systemFamilies[systemTokenFamily[token]]
		if palette == nil {
			return okhslToRgb(f.hue, (f.min+(f.max-f.min)*bellWeight(lightness))*sat, lightness)
		}
		slot := f.slot
		if s, ok := systemTokenSlots[token]; ok {
			slot = s
		}
		return anchored(palette[slot], f, lightness, sat)
	}
	target := func(level string, surfaceL, t float64) (float64, bool) {
		reached, ok := levelTarget(level, appearance, surfaceL)
		if !ok && t == 0 {
			return 0, false
		}
		if !ok {
			reached = extreme
		}
		distance := reached - surfaceL
		floorTarget, okF := levelTarget(systemReadableFloor[appearance], appearance, surfaceL)
		if !okF {
			floorTarget = extreme
		}
		floor := floorTarget - surfaceL
		compressed := distance
		if math.Abs(distance) > math.Abs(floor) {
			compressed = distance - (distance-floor)*math.Min(t, 1)
		}
		return surfaceL + compressed*(1-math.Max(0, t-1)), true
	}
	extremeText := rgb{0, 0, 0}
	if lighter {
		extremeText = rgb{255, 255, 255}
	}
	readable := func(c rgb) bool { return wcagContrast(extremeText, c) >= systemTextMinimumWCAG }
	limitPanel := func(token string, l float64) rgb {
		color := paint(token, l)
		if readable(color) {
			return color
		}
		low, high := backgroundL, l
		for i := 0; i < 20; i++ {
			mid := (low + high) / 2
			if readable(paint(token, mid)) {
				low = mid
			} else {
				high = mid
			}
		}
		return paint(token, low)
	}
	solve := func(t float64) map[string]rgb {
		colors := map[string]rgb{"background": background}
		for _, token := range systemSolveOrder {
			var targets []float64
			for _, r := range systemRules {
				if r.token != token {
					continue
				}
				for _, s := range r.on {
					surface, ok := colors[s]
					if !ok {
						surface = background
					}
					v, ok := target(r.level, oklabLightness(surface), t)
					if !ok || v < 0 || v > 1 {
						return nil
					}
					targets = append(targets, v)
				}
			}
			l := targets[0]
			for _, v := range targets[1:] {
				if lighter {
					l = math.Max(l, v)
				} else {
					l = math.Min(l, v)
				}
			}
			if containsString(systemPanels, token) {
				colors[token] = limitPanel(token, l)
			} else {
				colors[token] = paint(token, l)
			}
		}
		return colors
	}
	relaxation := 0.0
	colors := solve(0)
	if colors == nil {
		low, high := 0.0, 2.0
		colors = solve(high)
		for i := 0; i < systemRelaxationRounds; i++ {
			mid := (low + high) / 2
			if attempt := solve(mid); attempt != nil {
				high, colors = mid, attempt
			} else {
				low = mid
			}
		}
		relaxation = high
	}
	if colors == nil {
		colors = map[string]rgb{}
	}
	surfacesOf := func(token string) []rgb {
		var out []rgb
		for _, r := range systemRules {
			if r.token != token {
				continue
			}
			for _, s := range r.on {
				c, ok := colors[s]
				if !ok {
					c = background
				}
				out = append(out, c)
			}
		}
		return out
	}
	result := map[string]string{}
	for _, t := range systemTokens {
		if c, ok := colors[t[0]]; ok {
			result[t[0]] = hexOf(c)
		} else {
			result[t[0]] = ""
		}
	}
	for _, token := range systemForegroundTokens {
		surfaces := surfacesOf(token)
		text, hasText := colors[token]
		if in.foreground != nil {
			all := true
			var needed float64
			for i, s := range surfaces {
				v, ok := target(systemForegroundLevel, oklabLightness(s), relaxation)
				if !ok || v < 0 || v > 1 {
					all = false
					break
				}
				if i == 0 || (lighter && v > needed) || (!lighter && v < needed) {
					needed = v
				}
			}
			if all {
				fl := oklabLightness(*in.foreground)
				if (lighter && fl >= needed) || (!lighter && fl <= needed) {
					result[token] = ""
					continue
				}
				text, hasText = anchored(sourceOf(*in.foreground), systemFamilies["neutral"], oklabToOkhslLightness(needed), sat), true
			}
		}
		if hasText {
			result[token] = hexOf(withTextContrast(text, surfaces, lighter))
		}
	}
	return result, appearance, true
}
