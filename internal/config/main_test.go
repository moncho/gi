package config

import (
	"os"
	"testing"
)

// Isolate tests from the developer's global Pi settings (~/.pi/agent).
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gi-config-agent-")
	if err != nil {
		panic(err)
	}
	os.Setenv("PI_CODING_AGENT_DIR", dir)
	os.Setenv("GI_CODING_AGENT_DIR", dir+"-gi") // absent: no gi user config
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
