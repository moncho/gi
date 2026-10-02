// tui-ghostty-probe measures the terminal's actual cursor advance for the
// grapheme sequences used in Gi's Unicode Markdown tables. Run it inside the
// affected terminal, not through tmux or a pipe.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func cursorPosition(fd int) (int, error) {
	if _, err := os.Stdout.WriteString("\x1b[6n"); err != nil {
		return 0, err
	}
	var reply strings.Builder
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ms := int(time.Until(deadline).Milliseconds())
		if ms < 1 {
			break
		}
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, ms)
		if err != nil {
			return 0, err
		}
		if n == 0 {
			break
		}
		var b [1]byte
		if _, err := os.Stdin.Read(b[:]); err != nil {
			return 0, err
		}
		reply.WriteByte(b[0])
		if b[0] == 'R' {
			break
		}
		if reply.Len() > 80 {
			break
		}
	}
	raw := reply.String()
	start := strings.LastIndex(raw, "\x1b[")
	if start < 0 || !strings.HasSuffix(raw, "R") {
		return 0, fmt.Errorf("no cursor report (%q)", raw)
	}
	parts := strings.Split(strings.TrimSuffix(raw[start+2:], "R"), ";")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid cursor report %q", raw)
	}
	return strconv.Atoi(parts[1])
}

func run() ([]string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return nil, fmt.Errorf("stdin and stdout must be the affected terminal")
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	os.Stdout.WriteString("\x1b[?1049h\x1b[?25l")
	defer func() {
		os.Stdout.WriteString("\x1b[?25h\x1b[?1049l")
		term.Restore(fd, old)
	}()
	samples := []string{"ASCII", "日本語", "é", "🇵🇹", "⚠️", "👩🏽‍💻", "✈️", "a\u200d👩", "Flags 🇵🇹  │ 日本語 🧪  │"}
	var results []string
	for _, s := range samples {
		// Begin from column 1, away from the right edge and from auto-wrap.
		if _, err := os.Stdout.WriteString("\x1b[2J\x1b[1;1H" + s); err != nil {
			return nil, err
		}
		col, err := cursorPosition(fd)
		if err != nil {
			return nil, err
		}
		expected := gotui.StringWidth(s)
		status := "OK"
		if col-1 != expected {
			status = "MISMATCH"
		}
		results = append(results, fmt.Sprintf("%-10s terminal=%2d go-tui=%2d %q", status, col-1, expected, s))
	}
	return results, nil
}
func main() {
	results, err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, result := range results {
		fmt.Println(result)
	}
}
