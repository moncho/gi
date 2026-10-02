package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each config file resolves to .gi first, then .pi; the first existing wins.
func TestConfigFilesPreferGiThenPi(t *testing.T) {
	home, ws := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("GI_CODING_AGENT_DIR", "")
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

// PI_CODING_AGENT_DIR (Pi's getAgentDir) and GI_CODING_AGENT_DIR move the
// user-level directories; .gi still wins.
func TestUserConfigDirsHonourAgentDirEnv(t *testing.T) {
	giDir, piDir := t.TempDir(), t.TempDir()
	t.Setenv("GI_CODING_AGENT_DIR", giDir)
	t.Setenv("PI_CODING_AGENT_DIR", piDir)
	if got := UserConfigFile("auth.json"); got != filepath.Join(piDir, "auth.json") {
		t.Fatalf("none exist: %s", got)
	}
	if err := os.WriteFile(filepath.Join(giDir, "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := UserConfigFile("auth.json"); got != filepath.Join(giDir, "auth.json") {
		t.Fatalf("gi file: %s", got)
	}
}

// Project settings are read and written in .gi when .gi/settings.json
// exists, else in .pi; the two are never merged.
func TestProjectSettingsPreferGi(t *testing.T) {
	ws := t.TempDir()
	if ProjectConfigDirName(ws, "settings.json") != ".pi" {
		t.Fatal("default must be Pi's")
	}
	for dir, model := range map[string]string{".pi": "pi-model", ".gi": "gi-model"} {
		if err := os.MkdirAll(filepath.Join(ws, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, dir, "settings.json"), []byte(`{"defaultProvider":"p","defaultModel":"`+model+`"}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if cfg := Load(ws); cfg.DefaultModel != "gi-model" {
		t.Fatalf("read %q", cfg.DefaultModel)
	}
	if err := PersistModelSelection(ws, "p", "new-model", "", nil); err != nil {
		t.Fatal(err)
	}
	gi, _ := os.ReadFile(filepath.Join(ws, ".gi", "settings.json"))
	pi, _ := os.ReadFile(filepath.Join(ws, ".pi", "settings.json"))
	if !strings.Contains(string(gi), "new-model") || strings.Contains(string(pi), "new-model") {
		t.Fatalf("wrote the wrong file:\n.gi %s\n.pi %s", gi, pi)
	}
}
