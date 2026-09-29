package tui

import "testing"

func TestPiRGBToANSI256MatchesPi(t *testing.T) {
	// Expected values from pi-tui colors.js rgbToAnsi256 for Pi's dark theme.
	for _, tc := range []struct {
		r, g, b uint8
		want    uint8
	}{
		{33, 59, 73, 23}, {52, 56, 58, 237}, {37, 65, 49, 23}, {91, 40, 42, 52}, {222, 224, 225, 254},
		{167, 152, 215, 140}, {126, 136, 142, 102}, {118, 129, 134, 102}, {104, 183, 141, 72}, {234, 127, 129, 174},
	} {
		if got := piRGBToANSI256(tc.r, tc.g, tc.b); got != tc.want {
			t.Fatalf("rgb(%d,%d,%d)=%d want %d", tc.r, tc.g, tc.b, got, tc.want)
		}
	}
}

func TestDetectPiTrueColor(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	for _, tc := range []struct {
		env  map[string]string
		want bool
	}{
		{map[string]string{"TERM": "xterm-256color"}, false},
		{map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor"}, true},
		{map[string]string{"TERM": "xterm-ghostty"}, true},
		{map[string]string{"TERM": "tmux-256color", "TERM_PROGRAM": "ghostty"}, false},
		{map[string]string{"TERM": "xterm-direct"}, true},
		{map[string]string{"TERM": "xterm-256color", "VTE_VERSION": "7000"}, false},
		{map[string]string{"TERM": "xterm-256color", "PI_TRUE_COLOR": "1"}, true},
	} {
		if got := detectPiTrueColor(env(tc.env)); got != tc.want {
			t.Fatalf("%v: got %v want %v", tc.env, got, tc.want)
		}
	}
}
