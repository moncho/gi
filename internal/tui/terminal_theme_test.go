package tui

import (
	"fmt"
	"strings"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

func TestParseTerminalThemeReplies(t *testing.T) {
	c := parseTerminalReplies("\x1b]10;rgb:dede/e0e0/e1e1\x07\x1b]11;#1e1e2e\x1b\\\x1b[?997;2n\x1b[?62;22c")
	if c.foreground == nil || *c.foreground != (rgb{222, 224, 225}) || c.background == nil || *c.background != (rgb{30, 30, 46}) || c.scheme != "light" {
		t.Fatalf("replies %+v fg=%v bg=%v", c, c.foreground, c.background)
	}
	for raw, want := range map[string]rgb{"#ffffff": {255, 255, 255}, "#ffff00000000": {255, 0, 0}, "rgb:f/8/0": {255, 136, 0}, "rgba:0000/0000/ffff/ffff": {0, 0, 255}} {
		if got, ok := parseOSCColor(raw); !ok || got != want {
			t.Fatalf("%q = %v %v, want %v", raw, got, ok, want)
		}
	}
	if _, ok := parseOSCColor("#12345"); ok {
		t.Fatal("bad hex accepted")
	}
}

// Detection order is Pi's: reported background, scheme report, COLORFGBG, dark.
// The query is Pi's (OSC 10, 11, OSC 4 for 0-15) plus the scheme report;
// a complete palette is kept in index order.
func TestTerminalPaletteReplies(t *testing.T) {
	for i := 0; i < 16; i++ {
		if !strings.Contains(terminalThemeQuery, fmt.Sprintf("\x1b]4;%d;?\x07", i)) {
			t.Fatalf("query lacks OSC 4;%d", i)
		}
	}
	var replies strings.Builder
	for i := 15; i >= 0; i-- { // any order
		fmt.Fprintf(&replies, "\x1b]4;%d;rgb:%02x%02x/0000/0000\x1b\\", i, i, i)
	}
	c := parseTerminalReplies(replies.String() + "\x1b]11;#000000\x07\x1b[?62c")
	if len(c.palette) != 16 || c.palette[3] != (rgb{3, 0, 0}) || c.palette[15] != (rgb{15, 0, 0}) {
		t.Fatalf("palette %v", c.palette)
	}
	if c := parseTerminalReplies("\x1b]4;1;#ff0000\x07\x1b]4;2;#00ff00\x07"); c.palette != nil {
		t.Fatalf("partial palette kept: %v", c.palette)
	}
}

// The system theme replaces the palette with Pi's generated colours; the
// terminal's own foreground stays the terminal default.
func TestApplySystemTheme(t *testing.T) {
	t.Cleanup(func() { applyPiTheme("dark") })
	bg, fg := rgb{0x30, 0x34, 0x46}, rgb{0xc6, 0xd0, 0xf5}
	if !applySystemTheme(terminalColors{background: &bg, foreground: &fg}) || piActiveTheme != "system" {
		t.Fatal("system theme not applied")
	}
	if !piText.IsDefault() || piAccent.IsDefault() || piUserBg.IsDefault() {
		t.Fatalf("text %v accent %v userBg %v", piText, piAccent, piUserBg)
	}
	if applySystemTheme(terminalColors{}) {
		t.Fatal("applied without a background")
	}
}

func TestDetectTerminalThemeLikePi(t *testing.T) {
	light, dark := rgb{250, 250, 250}, rgb{20, 20, 30}
	fgDark := rgb{30, 30, 30}
	cases := []struct {
		colors terminalColors
		fgbg   string
		want   string
	}{
		{terminalColors{background: &light}, "", "light"},
		{terminalColors{background: &dark}, "15;7", "dark"},
		{terminalColors{background: &light, foreground: &fgDark}, "", "light"},
		{terminalColors{scheme: "light"}, "15;0", "light"},
		{terminalColors{}, "0;15", "light"},
		{terminalColors{}, "15;0", "dark"},
		{terminalColors{}, "15;8", "dark"},
		{terminalColors{}, "default;default", "dark"},
		{terminalColors{}, "", "dark"},
	}
	for _, tc := range cases {
		if got := detectTerminalTheme(tc.colors, tc.fgbg); got != tc.want {
			t.Fatalf("%+v %q = %s, want %s", tc.colors, tc.fgbg, got, tc.want)
		}
	}
}

func TestSelectPiThemeHonoursSetting(t *testing.T) {
	for _, tc := range []struct{ setting, terminal, want string }{
		{"", "light", "system"}, {"", "dark", "system"}, {"system", "light", "system"},
		{"dark", "light", "dark"}, {"light", "dark", "light"},
		{"light/dark", "light", "light"}, {"light/dark", "dark", "dark"},
		{"system/dark", "light", "system"},
		{"my-custom", "light", "system"}, {"a/b/c", "dark", "system"},
	} {
		if got := selectPiTheme(tc.setting, tc.terminal); got != tc.want {
			t.Fatalf("setting %q on %s = %s, want %s", tc.setting, tc.terminal, got, tc.want)
		}
	}
}

// Applying a theme swaps every palette colour for Pi's resolved values.
func TestApplyPiThemeSwapsPalette(t *testing.T) {
	t.Cleanup(func() { applyPiTheme("dark") })
	if !applyPiTheme("light") || piActiveTheme != "light" {
		t.Fatal("light theme not applied")
	}
	lt := piBuiltinThemes["light"]
	if piText != piRGB(lt["text"][0], lt["text"][1], lt["text"][2]) || piUserBg != piRGB(lt["userMessageBg"][0], lt["userMessageBg"][1], lt["userMessageBg"][2]) || piSyntaxKeyword != piRGB(lt["syntaxKeyword"][0], lt["syntaxKeyword"][1], lt["syntaxKeyword"][2]) {
		t.Fatal("light palette not applied")
	}
	applyPiTheme("dark")
	if piText != piRGB(222, 224, 225) || piAccent != piRGB(167, 152, 215) {
		t.Fatal("dark palette values drifted from Pi's")
	}
	if applyPiTheme("no-such-theme") {
		t.Fatal("unknown theme applied")
	}
}

func TestTerminalThemeRejectsMalformedColorsAndIndices(t *testing.T) {
	for _, value := range []string{"f/f/f", "rgb:f/f/f/f", "rgba:f/f/f", "rgba:f/f/f/no", "rgb:fffff/f/f", "rgb:/f/f", "#fff", "rgb:gg/f/f"} {
		if _, ok := parseOSCColor(value); ok {
			t.Errorf("accepted malformed colour %q", value)
		}
	}
	for _, value := range []string{"15;-1", "0;16", "0;256", "0;", "nonsense"} {
		if got := colorFgBgTheme(value); got != "" {
			t.Errorf("%q classified as %q", value, got)
		}
	}
	for i := 0; i < 16; i++ {
		want := "light"
		if i <= 6 || i == 8 {
			want = "dark"
		}
		if got := colorFgBgTheme(fmt.Sprintf("15;%d", i)); got != want {
			t.Errorf("index %d: %s", i, got)
		}
	}
}

func TestThemeRolesAppliedIndependently(t *testing.T) {
	t.Cleanup(func() { applyPiTheme("dark") })
	// An artificial full palette proves roles are not accidentally aliases.
	palette := make(map[string][3]uint8)
	for key, value := range piBuiltinThemes["light"] {
		palette[key] = value
	}
	keys := []string{"mdHeading", "mdCodeBlock", "mdCodeBlockBorder", "mdQuote", "mdQuoteBorder", "mdHr", "mdLinkUrl", "mdListBullet", "toolTitle", "toolOutput", "toolDiffAdded", "toolDiffRemoved", "toolDiffContext", "userMessageText", "customMessageLabel", "customMessageText"}
	for i, key := range keys {
		palette[key] = [3]uint8{uint8(i + 1), 2, 3}
	}
	piBuiltinThemes["role-fixture"] = palette
	defer delete(piBuiltinThemes, "role-fixture")
	applyPiTheme("role-fixture")
	colors := []gotui.Color{piMdHeading, piMdCodeBlock, piMdCodeBorder, piMdQuote, piMdQuoteBorder, piMdHr, piMdLinkUrl, piMdListBullet, piToolTitle, piToolOutput, piDiffAdded, piDiffRemoved, piDiffContext, piUserText, piCustomLabel, piCustomText}
	for i, color := range colors {
		if color != piRGB(uint8(i+1), 2, 3) {
			t.Errorf("role %s not applied", keys[i])
		}
	}
	base := gotui.NewStyle()
	if piMarkdownStyle(base, "heading") != base.Foreground(piMdHeading).Bold() {
		t.Fatal("heading still aliases warning")
	}
	if toolOutputStyle("ordinary source") != base.Foreground(piToolOutput) {
		t.Fatal("tool output still aliases muted")
	}
	if s, _ := diffLineStyle("+source"); s != base.Foreground(piDiffAdded) {
		t.Fatal("diff still aliases success")
	}
	if s, _ := piSyntaxStyle(base, "syn-removed"); s != base.Foreground(piDiffRemoved) {
		t.Fatal("syntax diff still aliases error")
	}
}
