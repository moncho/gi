//go:build !windows

package tui

import (
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Open nonblocking: os.File deadlines on a blocking /dev/tty are not portable.
// Poll and read explicitly so silent terminals cannot hang startup.
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
	for len(buf) < 8192 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(poll, int((remaining+time.Millisecond-1)/time.Millisecond))
		if err == unix.EINTR {
			continue
		}
		if err != nil || n == 0 || poll[0].Revents&unix.POLLIN == 0 {
			break
		}
		n, err = unix.Read(fd, chunk)
		if err == unix.EINTR || err == unix.EAGAIN {
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
