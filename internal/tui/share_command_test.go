package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gotui "github.com/grindlemire/go-tui"
)

// fakeGH makes ghCommand run a shell script standing in for gh; it logs its
// arguments and copies the gist's file.
func fakeGH(t *testing.T, script string) (dir string) {
	t.Helper()
	dir = t.TempDir()
	path := filepath.Join(dir, "gh")
	body := "#!/bin/sh\necho \"$@\" >> " + filepath.Join(dir, "log") + "\n" + script
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	previous := ghCommand
	ghCommand = func(ctx context.Context, args ...string) *exec.Cmd { return exec.CommandContext(ctx, path, args...) }
	t.Cleanup(func() { ghCommand = previous })
	return dir
}

func shareTestChat(t *testing.T) *chatTUI {
	c := sessionTestChat(t)
	c.cfg.WorkspaceRoot = t.TempDir()
	c.uiQueue = make(chan func(), 16)
	if err := c.store.AddMessage(t.Context(), "m1", "A", "user", "hello share", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	return c
}

// waitUI runs background UI updates until done holds.
func waitUI(t *testing.T, c *chatTUI, done func() bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !done() {
		select {
		case fn := <-c.uiQueue:
			fn()
		case <-deadline:
			t.Fatalf("timed out; transcript %q", c.transcript)
		}
	}
}

func lastLine(c *chatTUI) string {
	if len(c.transcript) == 0 {
		return ""
	}
	return c.transcript[len(c.transcript)-1]
}

// startShare runs /share and answers the warning.
func startShare(t *testing.T, c *chatTUI, yes bool) {
	t.Helper()
	c.appendTranscript(c.shareCommand()...)
	if c.modelMenuKind != "select" || !strings.HasPrefix(c.selectDialog.title, "Share session\nUpload this session as a secret GitHub gist?") {
		t.Fatalf("no warning: %q %q", c.modelMenuKind, c.selectDialog.title)
	}
	if !yes {
		pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyDown})
	}
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEnter})
}

// /share warns, then uploads Pi's HTML page as a secret gist and prints
// Pi's viewer link and the gist.
func TestShareCreatesSecretGist(t *testing.T) {
	dir := fakeGH(t, `case "$1" in gist) cp "$4" "$(dirname "$0")/uploaded.html"; echo https://gist.github.com/rui/abc123;; esac`)
	t.Setenv("GI_SHARE_VIEWER_URL", "")
	t.Setenv("PI_SHARE_VIEWER_URL", "")
	c := shareTestChat(t)
	startShare(t, c, false)
	if lastLine(c) != "sys: Share cancelled" {
		t.Fatal(lastLine(c))
	}
	if _, err := os.Stat(filepath.Join(dir, "log")); err == nil {
		t.Fatal("gh ran without confirmation")
	}
	startShare(t, c, true)
	waitUI(t, c, func() bool { return strings.HasPrefix(lastLine(c), "sys: Share URL") })
	if want := "sys: Share URL: https://pi.dev/session/#abc123\nGist: https://gist.github.com/rui/abc123"; lastLine(c) != want {
		t.Fatalf("got %q", lastLine(c))
	}
	if c.modelMenuOpen || c.loader != nil {
		t.Fatal("loader still open")
	}
	log, _ := os.ReadFile(filepath.Join(dir, "log"))
	calls := strings.Split(strings.TrimSpace(string(log)), "\n")
	if len(calls) != 2 || calls[0] != "auth status" || !strings.HasPrefix(calls[1], "gist create --public=false ") || !strings.HasSuffix(calls[1], "/session.html") {
		t.Fatalf("gh calls %q", calls)
	}
	page, _ := os.ReadFile(filepath.Join(dir, "uploaded.html"))
	if !strings.Contains(string(page), `id="session-data"`) {
		t.Fatalf("uploaded %.200s", page)
	}
	if _, err := os.Stat(filepath.Dir(strings.TrimPrefix(calls[1], "gist create --public=false "))); !os.IsNotExist(err) {
		t.Fatalf("temporary directory left: %v", err)
	}
	t.Setenv("GI_SHARE_VIEWER_URL", "https://viewer.example/s/")
	if got := shareViewerLink("abc"); got != "https://viewer.example/s/#abc" {
		t.Fatal(got)
	}
}

// Pi's errors: gh missing, not logged in, the gist failing.
func TestShareErrors(t *testing.T) {
	c := shareTestChat(t)
	previous := ghCommand
	ghCommand = func(ctx context.Context, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "gi-test-no-such-gh", args...)
	}
	startShare(t, c, true)
	waitUI(t, c, func() bool { return strings.HasPrefix(lastLine(c), "error:") })
	if lastLine(c) != "error: GitHub CLI (gh) is not installed. Install it from https://cli.github.com/" {
		t.Fatal(lastLine(c))
	}
	ghCommand = previous
	fakeGH(t, `[ "$1" = auth ] && exit 1`)
	startShare(t, c, true)
	waitUI(t, c, func() bool {
		return lastLine(c) != "error: GitHub CLI (gh) is not installed. Install it from https://cli.github.com/"
	})
	if lastLine(c) != "error: GitHub CLI is not logged in. Run 'gh auth login' first." {
		t.Fatal(lastLine(c))
	}
	fakeGH(t, `[ "$1" = gist ] && { echo "HTTP 422: Validation Failed" >&2; exit 1; }; exit 0`)
	startShare(t, c, true)
	waitUI(t, c, func() bool { return strings.Contains(lastLine(c), "gist") && !strings.Contains(lastLine(c), "login") })
	if lastLine(c) != "error: Failed to create gist: HTTP 422: Validation Failed" {
		t.Fatal(lastLine(c))
	}
}

// Pi's "Creating gist..." loader replaces the editor; Escape kills gh and
// cancels.
func TestShareLoaderCancels(t *testing.T) {
	dir := fakeGH(t, `[ "$1" = gist ] && { touch "$(dirname "$0")/started"; exec sleep 10; }; exit 0`)
	c := shareTestChat(t)
	startShare(t, c, true)
	waitUI(t, c, func() bool { return c.loader != nil })
	rows := c.piLoaderRows(40, time.Time{})
	var lines []string
	for _, row := range rows {
		var b strings.Builder
		for _, span := range row {
			b.WriteString(span.Text)
		}
		lines = append(lines, b.String())
	}
	want := []string{strings.Repeat("─", 40), "", " " + brailleSpinnerFrame(time.Time{}) + " Creating gist...", "", " escape/ctrl+c cancel", "", strings.Repeat("─", 40)}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("loader\n%s", strings.Join(lines, "\n"))
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(filepath.Join(dir, "started")); err == nil {
			break
		} else if time.Now().After(deadline) {
			t.Fatal("gh gist not started")
		}
	}
	started := time.Now()
	pressMenuKey(t, c.KeyMap(), gotui.KeyEvent{Key: gotui.KeyEscape})
	if lastLine(c) != "sys: Share cancelled" || c.loader != nil || c.modelMenuOpen {
		t.Fatalf("%q loader %v", lastLine(c), c.loader)
	}
	time.Sleep(200 * time.Millisecond)
	for len(c.uiQueue) > 0 {
		(<-c.uiQueue)()
	}
	if lastLine(c) != "sys: Share cancelled" || time.Since(started) > 5*time.Second {
		t.Fatalf("after cancel: %q", lastLine(c))
	}
}
