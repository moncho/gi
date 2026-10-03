package mcp

import (
	"fmt"
	"strings"
)

// DescribeState ports Pi's describeState (without resource counts).
func DescribeState(st Status) string {
	switch st.State {
	case StateDisabled:
		return "disabled"
	case StateFailed:
		msg := st.Error
		if msg == "" {
			msg = "unknown error"
		}
		return "failed: " + strings.SplitN(msg, "\n", 2)[0]
	case StateConnected:
		plural := "s"
		if st.Tools == 1 {
			plural = ""
		}
		return fmt.Sprintf("connected · %d tool%s", st.Tools, plural)
	case StateNeedsAuth:
		return "needs sign-in"
	case StateConnecting:
		return "connecting…"
	}
	return st.State
}

// ProblemReport ports Pi's reportProblems: one message for config errors and
// servers that failed or need a sign-in after startup; empty when none.
func ProblemReport(statuses []Status, configErrors []error) string {
	var lines []string
	for _, err := range configErrors {
		lines = append(lines, "  config: "+err.Error())
	}
	for _, st := range statuses {
		if st.State == StateNeedsAuth || st.State == StateFailed {
			lines = append(lines, "  "+st.Name+": "+DescribeState(st))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "MCP servers need attention:\n" + strings.Join(lines, "\n") + "\nRun /mcp to fix."
}

// StillConnectingNotice is Pi's notice when the first prompt stops waiting
// for servers whose tools are declared to the model.
const StillConnectingNotice = "MCP servers are still connecting; their tools become available once connected."
