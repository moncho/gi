//go:build darwin || linux

package tui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// The terminal's replies to the startup theme query must be consumed through
// /dev/tty. macOS poll(2) reports POLLNVAL for /dev/tty, so a reader that
// waited with poll left every reply unread; the UI then took the BEL ending
// each OSC reply as Ctrl+G and opened the external editor on startup.
//
// The test plays the terminal on a pty's master side for a child process
// whose controlling terminal is the pty, so the child's /dev/tty is real.
func TestQueryTerminalColorsThroughDevTTY(t *testing.T) {
	if os.Getenv("GI_TTY_QUERY_CHILD") != "" {
		queryTerminalColorsChild()
		return
	}
	// macOS and Linux always have /dev/ptmx: a failure is a broken helper.
	master, slave, err := openPTY()
	if err != nil {
		t.Fatal("open pty:", err)
	}
	defer unix.Close(master)
	slaveFile := os.NewFile(uintptr(slave), "pty-slave")

	cmd := exec.Command(os.Args[0], "-test.run=^TestQueryTerminalColorsThroughDevTTY$")
	cmd.Env = append(os.Environ(), "GI_TTY_QUERY_CHILD=1")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slaveFile, slaveFile, slaveFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	err = cmd.Start()
	// Only the child holds the slave now, so its exit ends master reads
	// instead of leaving them blocked.
	slaveFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var out bytes.Buffer
	replied := false
	deadline := time.Now().Add(20 * time.Second)
	chunk := make([]byte, 4096)
	for time.Now().Before(deadline) && !bytes.Contains(out.Bytes(), []byte("RESULT ")) {
		n, err := unix.Read(master, chunk)
		if n > 0 {
			out.Write(chunk[:n])
		}
		if (err != nil && err != unix.EINTR) || (n == 0 && err == nil) {
			break
		}
		if !replied && bytes.Contains(out.Bytes(), []byte("\x1b[c")) {
			replied = true
			reply := "\x1b]10;rgb:cdcd/d6d6/f4f4\x07\x1b]11;rgb:1e1e/1e1e/2e2e\x07\x1b[?62;22c"
			if _, err := unix.Write(master, []byte(reply)); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Keep draining the terminal so the child can write its exit output.
	go func() {
		for {
			if n, err := unix.Read(master, chunk); (err != nil && err != unix.EINTR) || (n == 0 && err == nil) {
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
	}
	m := regexp.MustCompile(`RESULT bg=(\S+) left=(\d+)`).FindSubmatch(out.Bytes())
	if !replied || m == nil {
		t.Fatalf("child did not query and report; output %q", out.String())
	}
	if string(m[1]) != "1e1e2e" || string(m[2]) != "0" {
		t.Fatalf("background %s with %s reply bytes left for the UI; want 1e1e2e and none", m[1], m[2])
	}
}

// queryTerminalColorsChild runs the startup query, then reports the colour it
// parsed and how many reply bytes were left unread on the terminal.
func queryTerminalColorsChild() {
	colors := queryTerminalColors(2 * time.Second)
	bg := "none"
	if c := colors.background; c != nil {
		bg = fmt.Sprintf("%02x%02x%02x", c.r, c.g, c.b)
	}
	left := 0
	if fd, err := unix.Open("/dev/tty", unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0); err == nil {
		if state, err := term.MakeRaw(fd); err == nil {
			time.Sleep(200 * time.Millisecond)
			left, _ = unix.Read(fd, make([]byte, 4096))
			left = max(left, 0)
			_ = term.Restore(fd, state)
		}
		unix.Close(fd)
	}
	fmt.Printf("RESULT bg=%s left=%d\n", bg, left)
}
