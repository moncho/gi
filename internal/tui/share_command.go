package tui

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gotui "github.com/grindlemire/go-tui"
	"github.com/rcarmo/gi/internal/sessionexport"
)

// shareViewerURL is Pi's default session viewer; GI_SHARE_VIEWER_URL (or
// Pi's PI_SHARE_VIEWER_URL) overrides it.
const shareViewerURL = "https://pi.dev/session/"

// ghCommand runs the GitHub CLI (replaced in tests).
var ghCommand = func(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "gh", args...)
}

// shareViewerLink is Pi's getShareViewerUrl.
func shareViewerLink(gistID string) string {
	base := shareViewerURL
	for _, name := range []string{"GI_SHARE_VIEWER_URL", "PI_SHARE_VIEWER_URL"} {
		if v := os.Getenv(name); v != "" {
			base = v
			break
		}
	}
	return base + "#" + gistID
}

// shareCommand is Pi's /share: the session as Pi's HTML page in a secret
// GitHub gist, with Pi's viewer link. gi asks first, as Pi's docs advise
// reviewing what a shared session contains.
func (c *chatTUI) shareCommand() []string {
	if c.store == nil || c.sessionID == "" {
		return []string{"error: Failed to export session: no active session"}
	}
	c.openSelect("Share session\nUpload this session as a secret GitHub gist? Anyone with the link can read it, and sessions can contain prompts, model responses, tool arguments, command output and file contents.",
		[]string{"Yes", "No"}, func(choice string) {
			if choice != "Yes" {
				c.appendTranscript("sys: Share cancelled")
				return
			}
			c.shareSession()
		}, func() { c.appendTranscript("sys: Share cancelled") })
	return nil
}

// shareSession checks gh, exports the page and creates the gist.
func (c *chatTUI) shareSession() {
	sessionID, cwd, theme := c.sessionID, c.cfg.WorkspaceRoot, exportTheme()
	go func() {
		fail := func(message string) { c.runOnUI(func() { c.appendTranscript("error: " + message) }) }
		var notFound *exec.Error
		if err := ghCommand(context.Background(), "auth", "status").Run(); errors.As(err, &notFound) {
			fail("GitHub CLI (gh) is not installed. Install it from https://cli.github.com/")
			return
		} else if err != nil {
			fail("GitHub CLI is not logged in. Run 'gh auth login' first.")
			return
		}
		dir, err := os.MkdirTemp("", "gi-share-")
		if err != nil {
			fail("Failed to export session: " + err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		file, err := sessionexport.Export(ctx, c.store, sessionID, cwd, filepath.Join(dir, "session.html"), theme)
		cancel()
		if err != nil {
			os.RemoveAll(dir)
			fail("Failed to export session: " + err.Error())
			return
		}
		c.runOnUI(func() { c.createGist(dir, file) })
	}()
}

// createGist runs gh gist create behind Pi's "Creating gist..." loader;
// Escape cancels it.
func (c *chatTUI) createGist(dir, file string) {
	ctx, cancel := context.WithCancel(context.Background())
	c.openLoader("Creating gist...", func() {
		cancel()
		c.appendTranscript("sys: Share cancelled")
	})
	go func() {
		defer os.RemoveAll(dir)
		var stdout, stderr bytes.Buffer
		cmd := ghCommand(ctx, "gist", "create", "--public=false", file)
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		cmd.WaitDelay = time.Second // a killed gh's children may hold its output open
		err := cmd.Run()
		if ctx.Err() != nil {
			return
		}
		cancel()
		c.runOnUI(func() {
			c.closeLoader()
			if err != nil {
				message := strings.TrimSpace(stderr.String())
				if message == "" {
					message = "Unknown error"
				}
				c.appendTranscript("error: Failed to create gist: " + message)
				return
			}
			gistURL := strings.TrimSpace(stdout.String())
			gistID := gistURL[strings.LastIndex(gistURL, "/")+1:]
			if gistID == "" {
				c.appendTranscript("error: Failed to parse gist ID from gh output")
				return
			}
			c.appendTranscript("sys: Share URL: " + shareViewerLink(gistID) + "\nGist: " + gistURL)
		})
	}()
}

// borderedLoader is Pi's BorderedLoader: a cancellable loader that replaces
// the editor.
type borderedLoader struct {
	message string
	onAbort func()
}

func (c *chatTUI) openLoader(message string, onAbort func()) {
	c.closeModelMenu()
	c.loader = &borderedLoader{message: message, onAbort: onAbort}
	c.modelMenuOpen = true
	c.modelMenuKind = "loader"
	c.inputActive = false
	if c.app != nil {
		c.app.BlurFocused()
		c.app.MarkDirty()
	}
}

// closeLoader restores the editor without aborting.
func (c *chatTUI) closeLoader() {
	if c.modelMenuKind != "loader" {
		return
	}
	c.loader = nil
	c.closeModelMenu()
}

func (c *chatTUI) abortLoader() {
	loader := c.loader
	c.closeLoader()
	if loader != nil && loader.onAbort != nil {
		loader.onAbort()
	}
}

func (c *chatTUI) loaderKeys() gotui.KeyMap {
	abort := func(gotui.KeyEvent) { c.abortLoader() }
	return gotui.KeyMap{
		gotui.OnPreemptStop(gotui.KeyEscape, abort),
		gotui.OnPreemptStop(gotui.KeyCtrlC, abort),
	}
}

// piLoaderRows renders BorderedLoader: border, the loader (a blank line, then
// spinner and message with one column of padding), its cancel hint, border.
func (c *chatTUI) piLoaderRows(width int, now time.Time) spanRows {
	rows := spanRows{piRule(width, piBorder), nil}
	line := []gotui.TextSpan{{Text: " "}, {Text: brailleSpinnerFrame(now), Style: piFg(piAccent)}, {Text: " "}, {Text: c.loader.message, Style: piFg(piMuted)}}
	rows = append(rows, line, nil)
	hint := append([]gotui.TextSpan{{Text: " "}}, keyHintSpans("escape/ctrl+c", "cancel")...)
	return append(rows, hint, nil, piRule(width, piBorder))
}
