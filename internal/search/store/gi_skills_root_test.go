package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// .gi/skills is a skills root (before .pi/skills) only where it exists, so
// workspaces without it keep their configuration fingerprint.
func TestGiSkillsRootOnlyWhenPresent(t *testing.T) {
	ws := t.TempDir()
	without, err := DefaultScopeConfig(ws, "skills", nil, nil, "v1")
	if err != nil || !reflect.DeepEqual(without.Roots(), []string{".pi/skills"}) {
		t.Fatal(without.Roots(), err)
	}
	if err := os.MkdirAll(filepath.Join(ws, ".gi", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	with, err := DefaultScopeConfig(ws, "skills", nil, nil, "v1")
	if err != nil || !reflect.DeepEqual(with.Roots(), []string{".gi/skills", ".pi/skills"}) {
		t.Fatal(with.Roots(), err)
	}
	all, err := DefaultScopeConfig(ws, "all", nil, nil, "v1")
	if err != nil || len(all.Roots()) != 3 {
		t.Fatal(all.Roots(), err)
	}
}
