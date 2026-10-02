package tui

import (
	"os"
	"runtime"
	"strings"

	gotui "github.com/grindlemire/go-tui"
)

// Pi decides between truecolor and 256-color output itself (pi-tui
// terminal-image.js detectCapabilities) and quantizes theme colors with its own
// nearest-color search (colors.js rgbToAnsi256). go-tui detects differently and
// truncates RGB into the cube, so the same theme looks different. Gi resolves
// the palette with Pi's rules and lets go-tui emit exactly those colors.

// piTrueColor is resolved from the startup environment, before
// enableGoTUIColorOutput adjusts COLORTERM for go-tui.
var piTrueColor = detectPiTrueColor(os.Getenv)

func detectPiTrueColor(getenv func(string) string) bool {
	switch getenv("PI_TRUE_COLOR") {
	case "1":
		return true
	case "0":
		return false
	}
	termProgram := strings.ToLower(getenv("TERM_PROGRAM"))
	terminalEmulator := strings.ToLower(getenv("TERMINAL_EMULATOR"))
	term := strings.ToLower(getenv("TERM"))
	colorTerm := strings.ToLower(getenv("COLORTERM"))
	hint := colorTerm == "truecolor" || colorTerm == "24bit" || strings.HasSuffix(term, "-direct")
	switch {
	case getenv("TMUX") != "" || strings.HasPrefix(term, "tmux"), strings.HasPrefix(term, "screen"):
		return hint
	case getenv("KITTY_WINDOW_ID") != "" || termProgram == "kitty",
		termProgram == "ghostty" || strings.Contains(term, "ghostty") || getenv("GHOSTTY_RESOURCES_DIR") != "",
		getenv("WEZTERM_PANE") != "" || termProgram == "wezterm",
		termProgram == "warpterminal" || getenv("WARP_SESSION_ID") != "" || getenv("WARP_TERMINAL_SESSION_UUID") != "",
		getenv("ITERM_SESSION_ID") != "" || termProgram == "iterm.app",
		getenv("WT_SESSION") != "",
		termProgram == "alacritty" || termProgram == "vscode" || termProgram == "zed",
		terminalEmulator == "jetbrains-jediterm",
		runtime.GOOS == "windows":
		return true
	}
	return hint
}

// enableGoTUIColorOutput makes go-tui pass colors through unchanged: RGB as
// 24-bit and palette indices as 38;5;N. It must run before the app/terminal
// is created, since go-tui samples the environment then.
func enableGoTUIColorOutput() {
	if ct := strings.ToLower(os.Getenv("COLORTERM")); ct != "truecolor" && ct != "24bit" {
		_ = os.Setenv("COLORTERM", "truecolor")
	}
}

var (
	piCubeValues = [6]int{0, 95, 135, 175, 215, 255}
	piGrayValues = func() (v [24]int) {
		for i := range v {
			v[i] = 8 + i*10
		}
		return v
	}()
)

func piFindClosest(values []int, target int) int {
	best, bestDist := 0, 1<<30
	for i, v := range values {
		d := v - target
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

func piColorDistance(r1, g1, b1, r2, g2, b2 int) float64 {
	dr, dg, db := float64(r1-r2), float64(g1-g2), float64(b1-b2)
	return dr*dr*0.299 + dg*dg*0.587 + db*db*0.114
}

// piRGBToANSI256 ports pi-tui colors.js rgbToAnsi256.
func piRGBToANSI256(r, g, b uint8) uint8 {
	R, G, B := int(r), int(g), int(b)
	ri, gi, bi := piFindClosest(piCubeValues[:], R), piFindClosest(piCubeValues[:], G), piFindClosest(piCubeValues[:], B)
	cr, cg, cb := piCubeValues[ri], piCubeValues[gi], piCubeValues[bi]
	cube := 16 + 36*ri + 6*gi + bi
	gray := int(0.299*float64(R) + 0.587*float64(G) + 0.114*float64(B) + 0.5)
	grayIndex := piFindClosest(piGrayValues[:], gray)
	gv := piGrayValues[grayIndex]
	spread := max(R, max(G, B)) - min(R, min(G, B))
	if spread < 10 && piColorDistance(R, G, B, gv, gv, gv) < piColorDistance(R, G, B, cr, cg, cb) {
		return uint8(232 + grayIndex)
	}
	return uint8(cube)
}

// piRGB is a theme color as Pi would emit it in this terminal.
func piRGB(r, g, b uint8) gotui.Color {
	if piTrueColor {
		return gotui.RGBColor(r, g, b)
	}
	return gotui.ANSIColor(piRGBToANSI256(r, g, b))
}
