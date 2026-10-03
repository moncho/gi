package tui

import (
	"fmt"
	"os"
	"runtime"

	gotui "github.com/grindlemire/go-tui"
)

// authURL is Pi's AuthUrlComponent (components/auth-url.js): a sign-in URL
// with a click hint and a copy hint. Hosts call copy on app.message.copy
// (Ctrl+X), since a long URL wraps and often cannot be selected or clicked
// as a whole (SSH, tmux). Used by /login and the /mcp sign-in screen.
type authURL struct {
	url    string
	suffix []gotui.TextSpan // after the click hint: the copy hint, or the copy result
}

func newAuthURL(url string) *authURL {
	return &authURL{url: url, suffix: keyHintSpans("ctrl+x", "to copy")}
}

// spans are the component's two Text lines (padding 1 is the host's).
func (a *authURL) spans() [2][]gotui.TextSpan {
	hint := []gotui.TextSpan{{Text: clickHint(), Style: piFg(piDim), Link: a.url}, {Text: " "}, {Text: "•", Style: piFg(piDim)}, {Text: " "}}
	return [2][]gotui.TextSpan{{{Text: a.url, Style: piFg(piAccent), Link: a.url}}, append(hint, a.suffix...)}
}

// copy is AuthUrlComponent.copy: the result replaces the copy hint.
func (c *chatTUI) copyAuthURL(a *authURL) {
	if err := c.copyToClipboard(a.url); err != nil {
		a.suffix = []gotui.TextSpan{{Text: err.Error(), Style: piFg(piError)}}
	} else {
		a.suffix = []gotui.TextSpan{{Text: "Copied URL to clipboard", Style: piFg(piSuccess)}}
	}
	c.markDirty()
}

// copyToClipboard is Pi's copyToClipboard (utils/clipboard.js): the native
// clipboard, then OSC 52 in remote sessions or without a display, since
// OSC 52 cannot be verified.
func (c *chatTUI) copyToClipboard(text string) error {
	if c.copyNative(text) == nil {
		return nil
	}
	remote := os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" || os.Getenv("MOSH_CONNECTION") != ""
	headless := runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("TERMUX_VERSION") == ""
	if remote || headless {
		if err := c.writeOSC52(text); err == nil {
			return nil
		} else if len(text) > osc52PayloadLimit {
			return fmt.Errorf("Clipboard unavailable: text exceeds the OSC 52 size limit")
		}
	}
	if runtime.GOOS == "linux" {
		switch {
		case os.Getenv("TERMUX_VERSION") != "":
			return fmt.Errorf("Clipboard unavailable: install the Termux:API app and `termux-api` package")
		case os.Getenv("WAYLAND_DISPLAY") != "":
			return fmt.Errorf("Clipboard unavailable: install `wl-clipboard` (`wl-copy`) or check Wayland access")
		case os.Getenv("DISPLAY") != "":
			return fmt.Errorf("Clipboard unavailable: install `xclip` or `xsel`, or check X11 access")
		}
	}
	return fmt.Errorf("Clipboard unavailable")
}
