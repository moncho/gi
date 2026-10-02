package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveToolPathWorkspace(t *testing.T) {
	root := t.TempDir()
	resolved, err := ResolveToolPath(root, "docs/test.txt", false)
	if err != nil {
		t.Fatalf("resolve workspace path: %v", err)
	}
	if resolved.IsVFS() {
		t.Fatalf("expected workspace path, got vfs namespace=%q", resolved.VFSNamespace)
	}
	if resolved.WorkspacePath == "" {
		t.Fatalf("empty workspace path")
	}
}

func TestResolveToolPathTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveToolPath(root, "../evil.txt", false); err == nil {
		t.Fatalf("expected traversal error")
	} else if !strings.Contains(err.Error(), "path escapes workspace") {
		t.Fatalf("expected traversal error, got %q", err)
	}
}

func TestResolveToolPathEmptyPath(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveToolPath(root, "", false); err == nil {
		t.Fatalf("expected empty path error")
	} else if err.Error() != "empty path" {
		t.Fatalf("expected empty path, got %q", err)
	}
}

func TestResolveToolPathMalformedVFS(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveToolPath(root, "vfs://", false); err == nil {
		t.Fatalf("expected malformed vfs error")
	} else if !strings.Contains(err.Error(), "invalid vfs path") {
		t.Fatalf("expected invalid vfs path error, got %q", err)
	}

	if _, err := ResolveToolPath(root, "vfs://scripts/../evil", false); err == nil {
		t.Fatalf("expected traversal vfs error")
	} else if !strings.Contains(err.Error(), "traversal outside namespace") {
		t.Fatalf("expected traversal outside namespace error, got %q", err)
	}
}

func TestResolveToolPathVFSReadOnlyReference(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveToolPath(root, "vfs://reference/system/readme.md", true); err == nil {
		t.Fatalf("expected reference namespace write error")
	} else if !strings.Contains(err.Error(), "vfs namespace is read-only") {
		t.Fatalf("expected read-only error, got %q", err)
	}
}

func TestResolveToolPathVFSWritableNamespace(t *testing.T) {
	root := t.TempDir()
	resolved, err := ResolveToolPath(root, "vfs://scripts/doc.md", false)
	if err != nil {
		t.Fatalf("resolve vfs path: %v", err)
	}
	if !resolved.IsVFS() {
		t.Fatal("expected vfs resolution")
	}
	if resolved.VFSNamespace != "scripts" || resolved.VFSPath != "doc.md" {
		t.Fatalf("unexpected vfs resolution: %#v", resolved)
	}
}

func TestResolveToolPathSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatalf("write outside secret: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink not supported in test env: %v", err)
	}
	if _, err := ResolveToolPath(root, "link/secret.txt", false); err == nil {
		t.Fatalf("expected symlink escape error")
	} else if !strings.Contains(err.Error(), "path escapes workspace") {
		t.Fatalf("unexpected symlink escape error: %v", err)
	}
}

func TestResolveToolPathFTSReadOnlyNamespace(t *testing.T) {
	root := t.TempDir()
	resolved, err := ResolveToolPath(root, "fts://messages?q=hello", false)
	if err != nil {
		t.Fatalf("resolve fts path: %v", err)
	}
	if !resolved.IsVFS() {
		t.Fatal("expected fts to resolve as virtual namespace")
	}
	if resolved.VFSNamespace != "fts" || resolved.VFSPath != "messages?q=hello" {
		t.Fatalf("unexpected fts resolution: %#v", resolved)
	}
	if _, err := ResolveToolPath(root, "fts://messages?q=hello", true); err == nil {
		t.Fatal("expected write rejection for fts namespace")
	} else if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("unexpected fts write error: %v", err)
	}
}

func TestResolveToolPathAbsoluteWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	for _, raw := range []string{filepath.Join(root, "missing", "nested", "file.txt"), filepath.Join(outside, "file.txt"), root + "-other/file.txt"} {
		for _, write := range []bool{false, true} {
			got, err := ResolveToolPath(root, raw, write)
			if strings.HasPrefix(raw, root+string(os.PathSeparator)) {
				if err != nil || got.WorkspacePath != raw {
					t.Fatalf("absolute in-workspace path: %v %v", got, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "path escapes workspace") {
				t.Fatalf("outside absolute path %q: %v", raw, err)
			}
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ResolveToolPath(rel, "file.txt", false)
	if err != nil || got.WorkspacePath != filepath.Join(root, "file.txt") {
		t.Fatalf("relative root: %v %v", got, err)
	}
	if !pathWithinRoot(filepath.Join(string(os.PathSeparator), "file"), string(os.PathSeparator)) {
		t.Fatal("filesystem root boundary broken")
	}
}
func TestResolveToolPathAbsoluteSymlinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink(filepath.Join(outside, "absent"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "inside"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "inside"), filepath.Join(root, "safe")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"escape/secret", "escape/new/deep/file", "dangling/file"} {
		for _, write := range []bool{false, true} {
			for _, raw := range []string{name, filepath.Join(root, name)} {
				if _, err := ResolveToolPath(root, raw, write); err == nil {
					t.Fatalf("symlink escape accepted: %q", raw)
				}
			}
		}
	}
	for _, name := range []string{"inside/new/deep/file", "safe/new/deep/file"} {
		if _, err := ResolveToolPath(root, filepath.Join(root, name), false); err != nil {
			t.Fatalf("in-workspace symlink/absent parents: %v", err)
		}
	}
	alias := filepath.Join(t.TempDir(), "workspace")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveToolPath(alias, filepath.Join(alias, "inside", "file"), false); err != nil {
		t.Fatalf("symlinked workspace root: %v", err)
	}
}
