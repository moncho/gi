package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Each config file resolves to .gi first, then .pi; the first existing wins.
func TestConfigFilesPreferGiThenPi(t *testing.T) {
	home, ws := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	write := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	piUser, giUser := filepath.Join(home, ".pi", "agent", "mcp.json"), filepath.Join(home, ".gi", "agent", "mcp.json")
	piProj, giProj := filepath.Join(ws, ".pi", "mcp.json"), filepath.Join(ws, ".gi", "mcp.json")
	if got := UserConfigFile("mcp.json"); got != piUser {
		t.Fatalf("none exist: %s, want the Pi path", got)
	}
	write(piUser)
	write(piProj)
	if UserConfigFile("mcp.json") != piUser || ProjectConfigFile(ws, "mcp.json") != piProj {
		t.Fatal("Pi-only files not found")
	}
	write(giUser)
	write(giProj)
	if UserConfigFile("mcp.json") != giUser || ProjectConfigFile(ws, "mcp.json") != giProj {
		t.Fatal(".gi files must win over .pi")
	}
}
