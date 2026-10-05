//go:build !windows

package tui

import (
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Open nonblocking: os.File deadlines on a blocking /dev/tty are not portable.
// Read until the deadline so silent terminals cannot hang startup. poll(2) is
// not used: macOS reports POLLNVAL for ttys, which left every reply unread for
// the UI to take as keys (the BEL ending each OSC reply is Ctrl+G).
func queryTerminalColors(timeout time.Duration) terminalColors {
	fd, err := unix.Open("/dev/tty", unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		return terminalColors{}
	}
	defer unix.Close(fd)
	if !term.IsTerminal(fd) {
		return terminalColors{}
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return terminalColors{}
	}
	defer term.Restore(fd, state)
	if _, err := unix.Write(fd, []byte(terminalThemeQuery)); err != nil {
		return terminalColors{}
	}
	return readTerminalThemeReplies(fd, timeout)
}

func readTerminalThemeReplies(fd int, timeout time.Duration) terminalColors {
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 0, 1024)
	chunk := make([]byte, 256)
	for len(buf) < 8192 && time.Now().Before(deadline) {
		n, err := unix.Read(fd, chunk)
		if err == unix.EINTR {
			continue
		}
		if err == unix.EAGAIN {
			time.Sleep(2 * time.Millisecond)
			continue
		}
		if err != nil || n == 0 {
			break
		}
		buf = append(buf, chunk[:n]...)
		if deviceAttributes.Match(buf) {
			break
		}
	}
	return parseTerminalReplies(string(buf))
}
