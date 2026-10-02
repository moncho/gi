package skills

import (
	"os"
	"testing"
)

// Isolate tests from the developer's user-level agent directories
// (~/.gi/agent, ~/.pi/agent): skills and settings there must not leak in.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gi-test-agent-")
	if err != nil {
		panic(err)
	}
	os.Setenv("GI_CODING_AGENT_DIR", dir+"/gi")
	os.Setenv("PI_CODING_AGENT_DIR", dir+"/pi")
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
