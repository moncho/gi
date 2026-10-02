package tools

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// ResultError is a tool failure whose Text is the complete tool result for
// the model (Pi's isError results), e.g. a shell command's output followed
// by its exit status. The engine sends Text as is, without an "Error:"
// prefix, so the output is not lost.
type ResultError struct {
	Text string
	Err  error
}

func (e *ResultError) Error() string { return e.Text }
func (e *ResultError) Unwrap() error { return e.Err }

// appendStatus is Pi's bash appendStatus: the output, a blank line, then
// the status.
func appendStatus(text, status string) string {
	if text == "" {
		return status
	}
	return text + "\n\n" + status
}

// shellExitError reports a failed shell command like Pi's bash tool:
// "<output or (no output)>\n\nCommand exited with code N" (128+signal for
// commands killed by a signal). Cancellation and start failures keep their
// original error.
func shellExitError(ctx context.Context, output string, err error) error {
	if ctx.Err() != nil {
		return err
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}
	code := exitErr.ExitCode()
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		code = 128 + int(ws.Signal())
	}
	text := strings.TrimRight(output, "\n")
	if strings.TrimSpace(text) == "" {
		text = "(no output)"
	}
	return &ResultError{Text: appendStatus(text, fmt.Sprintf("Command exited with code %d", code)), Err: err}
}
