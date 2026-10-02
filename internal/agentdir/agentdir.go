// Package agentdir locates the user-level agent directories: gi's
// (GI_CODING_AGENT_DIR, else ~/.gi/agent), then Pi's (PI_CODING_AGENT_DIR,
// else ~/.pi/agent, Pi's getAgentDir). It has no dependencies so any package
// can use it.
package agentdir

import (
	"os"
	"path/filepath"
	"strings"
)

// Dirs are the user-level agent directories in lookup order.
func Dirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	dir := func(env, name string) string {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
		return filepath.Join(home, name, "agent")
	}
	return []string{dir("GI_CODING_AGENT_DIR", ".gi"), dir("PI_CODING_AGENT_DIR", ".pi")}
}
