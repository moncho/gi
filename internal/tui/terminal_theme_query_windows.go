package tui

import "time"

// Windows retains environment-based detection until console probing is supported.
func queryTerminalColors(time.Duration) terminalColors { return terminalColors{} }
