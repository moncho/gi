package tui

import "testing"

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
		{"", "light", "light"}, {"", "dark", "dark"},
		{"dark", "light", "dark"}, {"light", "dark", "light"},
		{"light/dark", "light", "light"}, {"light/dark", "dark", "dark"},
		{"my-custom", "light", "light"}, {"a/b/c", "dark", "dark"},
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
