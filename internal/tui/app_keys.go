package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rcarmo/gi/internal/config"
)

// Pi's app.* actions (pi-coding-agent interactive-mode.js) bound to Pi's
// default keys in KeyMap; see docs/internal/keybindings.md.

const ctrlCExitWindow = 500 * time.Millisecond // Pi's handleCtrlC

// handleCtrlC is Pi's app.clear: the first press clears the editor, a
// second within half a second exits. With a transcript selection it copies
// the selection (gi).
func (c *chatTUI) handleCtrlC() {
	if c.textSelection.active {
		c.copyTranscriptSelection()
		return
	}
	now := time.Now()
	if now.Sub(c.lastCtrlC) < ctrlCExitWindow {
		if c.app != nil {
			c.app.Stop()
		}
		return
	}
	c.lastCtrlC = now
	c.input.Clear()
	c.markDirty()
}

// copySelectionOrLastMessage is Pi's app.message.copy: the selection, else
// the last assistant message, with a brief confirmation.
func (c *chatTUI) copySelectionOrLastMessage() {
	if c.textSelection.active {
		c.copyTranscriptSelection()
		return
	}
	if lines := c.copyLastAssistantLines(); len(lines) > 0 {
		c.selectionNotice(lines[0])
	}
}

// toggleThinkingBlocks is Pi's app.thinking.toggle: thinking blocks show as
// a "Thinking..." label, saved as hideThinkingBlock.
func (c *chatTUI) toggleThinkingBlocks() {
	c.cfg.HideThinkingBlock = !c.cfg.HideThinkingBlock
	state := "visible"
	if c.cfg.HideThinkingBlock {
		state = "hidden"
	}
	if err := config.PersistHideThinkingBlock(c.cfg.WorkspaceRoot, c.cfg.HideThinkingBlock); err != nil {
		c.selectionNotice(fmt.Sprintf("Thinking blocks: %s (not saved: %v)", state, err))
		return
	}
	c.selectionNotice("Thinking blocks: " + state)
}

// externalEditorCommand is Pi's getExternalEditorCommand.
func (c *chatTUI) externalEditorCommand() string {
	if editor := strings.TrimSpace(c.cfg.ExternalEditor); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "nano"
}

// openExternalEditor is Pi's app.editor.external: the draft is edited as
// prompt.md in the external editor, which gets the terminal; when it exits
// successfully the editor takes the file's text (one trailing newline
// dropped).
func (c *chatTUI) openExternalEditor() {
	if c.app == nil {
		return
	}
	command := c.externalEditorCommand()
	draft := c.input.Text()
	c.app.RunWithTerminal(func() {
		text, ok := editInExternalEditor(command, draft)
		if ok {
			c.input.SetText(text)
		}
	})
}

// editInExternalEditor is Pi's editInExternalEditor.
func editInExternalEditor(command, content string) (string, bool) {
	dir, err := os.MkdirTemp("", "pi-editor-")
	if err != nil {
		return "", false
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", false
	}
	fields := strings.Split(command, " ")
	fmt.Printf("Launching external editor: %s\ngi will resume when the editor exits.\n", command)
	cmd := exec.Command(fields[0], append(fields[1:], path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	text := strings.TrimPrefix(string(raw), "\ufeff")
	return strings.TrimSuffix(text, "\n"), true
}

// suspend is Pi's app.suspend: the process stops in the background
// (resumed with fg).
func (c *chatTUI) suspend() {
	if runtime.GOOS == "windows" {
		c.selectionNotice("Suspend to background is not supported on Windows")
		return
	}
	if c.app != nil {
		c.app.Suspend()
	}
}

// pasteClipboard is Pi's app.clipboard.pasteImage for images: a clipboard
// image is attached to the draft.
func (c *chatTUI) pasteClipboard() {
	if lines := c.pasteImageCommand("/paste-image", []string{"/paste-image"}); len(lines) > 0 {
		c.selectionNotice(lines[0])
	}
}
