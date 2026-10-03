package tui

import (
	"os"
	"runtime"
	"strings"

	gotui "github.com/grindlemire/go-tui"
)

// Pi's default keys differ on Windows and WSL (pi-coding-agent
// core/keybindings.js); see docs/internal/keybindings.md.

// piKeys are Pi's platform-dependent defaults and key names.
type piKeys struct {
	windows, wsl bool // Windows itself; Linux under WSL
	darwin       bool // macOS: Alt is shown as Option
}

// defaultPiKeys is decided once, as Pi decides when its modules load.
var defaultPiKeys = piKeys{
	windows: runtime.GOOS == "windows",
	darwin:  runtime.GOOS == "darwin",
	wsl:     runtime.GOOS == "linux" && (os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != ""),
}

// windowsKeys is Pi's useWindowsKeybindings.
func (k piKeys) windowsKeys() bool { return k.windows || k.wsl }

// undo is tui.editor.undo: Ctrl+Z on Windows, Alt+Z on WSL, else Ctrl+-.
func (k piKeys) undo() (gotui.KeyMatcher, string) {
	switch {
	case k.windows:
		return gotui.Rune('z').Ctrl(), "Ctrl+Z"
	case k.wsl:
		return gotui.Rune('z').Alt(), "Alt+Z"
	}
	return gotui.Rune('-').Ctrl(), "Ctrl+-"
}

// followUp is app.message.followUp: Ctrl+Q on Windows keys, else Alt+Enter.
func (k piKeys) followUp() (gotui.KeyMatcher, string) {
	if k.windowsKeys() {
		return gotui.Rune('q').Ctrl(), "Ctrl+Q"
	}
	return gotui.KeyEnter.Alt(), "Alt+Enter"
}

// dequeue is app.message.dequeue: Alt+Q on Windows keys, else Alt+Up.
func (k piKeys) dequeue() (gotui.KeyMatcher, string) {
	if k.windowsKeys() {
		return gotui.Rune('q').Alt(), "Alt+Q"
	}
	return gotui.KeyUp.Alt(), "Alt+Up"
}

// cycleModelBackward is app.model.cycleBackward: Alt+P on Windows keys,
// else Shift+Ctrl+P.
func (k piKeys) cycleModelBackward() (gotui.KeyMatcher, string) {
	if k.windowsKeys() {
		return gotui.Rune('p').Alt(), "Alt+P"
	}
	return gotui.Rune('p').Ctrl().Shift(), "Shift+Ctrl+P"
}

// pasteImage is app.clipboard.pasteImage: Alt+V on Windows keys, else Ctrl+V.
func (k piKeys) pasteImage() (gotui.KeyMatcher, string) {
	if k.windowsKeys() {
		return gotui.Rune('v').Alt(), "Alt+V"
	}
	return gotui.KeyCtrlV, "Ctrl+V"
}

// search is tui.altScreen.search: Ctrl+F on Windows keys, else Ctrl+Shift+F.
func (k piKeys) search() gotui.KeyMatcher {
	if k.windowsKeys() {
		return gotui.Rune('f').Ctrl()
	}
	return gotui.Rune('f').Ctrl().Shift()
}

// display is Pi's formatKeyText with capitalize: Alt is Option on macOS.
func (k piKeys) display(key string) string {
	if !k.darwin {
		return key
	}
	parts := strings.Split(key, "+")
	for i, part := range parts {
		if strings.EqualFold(part, "alt") {
			parts[i] = "Option"
		}
	}
	return strings.Join(parts, "+")
}

// keyText is Pi's keyText: the key in lower case (Option on macOS).
func (k piKeys) keyText(key string) string { return strings.ToLower(k.display(key)) }

// hotkeysMarkdown is Pi's /hotkeys text with these keys.
func (k piKeys) hotkeysMarkdown() string {
	_, undo := k.undo()
	_, followUp := k.followUp()
	_, dequeue := k.dequeue()
	_, cycleBack := k.cycleModelBackward()
	_, paste := k.pasteImage()
	suspend := "Ctrl+Z"
	newLine := "New line"
	if k.windows {
		suspend, newLine = "", "New line (Ctrl+Enter on Windows Terminal)"
	}
	return k.optionKeys(strings.NewReplacer(
		"{newLine}", newLine,
		"{undo}", undo,
		"{suspend}", suspend,
		"{cycleBack}", cycleBack,
		"{followUp}", followUp,
		"{dequeue}", dequeue,
		"{paste}", paste,
	).Replace(piHotkeysTemplate))
}

// optionKeys renames Alt in a /hotkeys table's key cells on macOS.
func (k piKeys) optionKeys(markdown string) string {
	if !k.darwin {
		return markdown
	}
	return strings.ReplaceAll(markdown, "Alt+", "Option+")
}

// giHotkeysMarkdown is gi's /hotkeys table with these keys.
func (k piKeys) giHotkeysMarkdown() string {
	search := "Ctrl+Shift+F"
	if k.windowsKeys() {
		search = "Ctrl+F"
	}
	return k.optionKeys(strings.ReplaceAll(giHotkeysTemplate, "{search}", search))
}
