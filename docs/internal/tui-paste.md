# TUI paste

Pasting several lines into the editor used to submit the first line: gi did
not enable bracketed paste, so a pasted newline arrived as Enter.

- go-tui enables bracketed paste (mode 2004) and delivers a paste as one
  `PasteEvent` (`third_party/go-tui/README.gi.md`).
- The editor inserts it as Pi's `handlePaste` does
  (`internal/tui/input_paste.go`): line endings normalised, tabs as four
  spaces, control characters dropped (CSI-u Ctrl+letter sequences from tmux
  popups decoded first), a space before a pasted path that follows a word.
- A paste of more than 10 lines or 1000 characters becomes a marker,
  `[paste #1 +123 lines]` or `[paste #2 1234 chars]`, as in Pi. The cursor
  and Backspace/Delete treat a marker as one unit; deleting one renumbers the
  higher markers; undo restores it. Submit and follow-up send the expanded
  text. Durable drafts, session switches and editor prompts keep the
  expanded text, so pasted content is never lost.
- When the editor is not focused (a menu is open), the paste replays as
  keystrokes.

Tests: `internal/tui/input_paste_test.go`, go-tui `paste_test.go`, and the
paste step of `make test-tui-smoke` (tmux `paste-buffer -p`: three lines
appear in the editor, nothing is submitted until Enter, then one message).
