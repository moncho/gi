package tools

import (
	"os"
	"path/filepath"
	"testing"
)

// Directories registered as read-only (user skill directories) are readable
// outside the workspace; writes there are still refused.
func TestReadOnlyRootsAllowReadsOnly(t *testing.T) {
	ws, skills := t.TempDir(), t.TempDir()
	file := filepath.Join(skills, "demo", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveToolPath(ws, file, false); err == nil {
		t.Fatal("outside path readable before registration")
	}
	SetReadOnlyRoots([]string{skills})
	t.Cleanup(func() { SetReadOnlyRoots(nil) })
	if _, err := ResolveToolPath(ws, file, false); err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := ResolveToolPath(ws, file, true); err == nil {
		t.Fatal("write allowed outside the workspace")
	}
	if _, err := ResolveToolPath(ws, filepath.Join(filepath.Dir(skills), "other"), false); err == nil {
		t.Fatal("sibling of a read-only root readable")
	}
}
