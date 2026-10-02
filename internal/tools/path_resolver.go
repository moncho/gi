package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	storevfs "github.com/rcarmo/gi/internal/store/vfs"
)

// ToolPath represents a resolved tool input path after validation.
// It can point at either the workspace filesystem or a managed VFS namespace.
type ToolPath struct {
	WorkspacePath string
	VFSNamespace  string
	VFSPath       string
	isVFS         bool
}

// IsVFS reports whether this path resolves into a managed VFS namespace.
func (p ToolPath) IsVFS() bool { return p.isVFS }

func resolveToolPath(root, raw string, writable bool) (resolvedPath, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return resolvedPath{}, fmt.Errorf("empty path")
	}
	if strings.HasPrefix(trimmed, "fts://") {
		if writable {
			return resolvedPath{}, fmt.Errorf("fts namespace is read-only")
		}
		locator := strings.TrimLeft(strings.TrimPrefix(trimmed, "fts://"), "/")
		if locator == "" {
			locator = "help"
		}
		return resolvedPath{workspacePath: "", vfsNamespace: "fts", vfsPath: locator, isVFS: true}, nil
	}
	if strings.HasPrefix(trimmed, "vfs://") {
		ns, vpath, err := storevfs.ParsePath(trimmed)
		if err != nil {
			return resolvedPath{}, err
		}
		if writable && (ns == "reference" || ns == "chat") {
			return resolvedPath{}, fmt.Errorf("vfs namespace is read-only: %s", ns)
		}
		return resolvedPath{workspacePath: "", vfsNamespace: ns, vfsPath: vpath, isVFS: true}, nil
	}
	rootClean, err := filepath.Abs(root)
	if err != nil {
		return resolvedPath{}, fmt.Errorf("resolve workspace root: %w", err)
	}
	full := trimmed
	if !filepath.IsAbs(full) {
		full = filepath.Join(rootClean, full)
	}
	clean := filepath.Clean(full)
	rootResolved, err := resolveExistingPath(rootClean)
	if err != nil {
		return resolvedPath{}, err
	}
	targetResolved, err := resolveExistingPath(clean)
	if err != nil {
		return resolvedPath{}, err
	}
	if !pathWithinRoot(targetResolved, rootResolved) {
		return resolvedPath{}, fmt.Errorf("path escapes workspace")
	}
	return resolvedPath{workspacePath: clean, isVFS: false}, nil
}

// Resolve through the nearest existing ancestor when a write creates multiple
// missing directories. Never fall back to lexical checks below an unresolved
// (including dangling) symlink.
func resolveExistingPath(path string) (string, error) {
	candidate := path
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		if info, lerr := os.Lstat(candidate); lerr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("unresolved workspace symlink: %s", candidate)
		} else if lerr != nil && !os.IsNotExist(lerr) {
			return "", lerr
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", err
		}
		missing = append(missing, filepath.Base(candidate))
		candidate = parent
	}
}

func pathWithinRoot(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

type resolvedPath struct {
	workspacePath string
	vfsNamespace  string
	vfsPath       string
	isVFS         bool
}

// ResolveToolPath exposes the shared path resolution strategy to other packages.
func ResolveToolPath(root, raw string, writable bool) (ToolPath, error) {
	resolved, err := resolveToolPath(root, raw, writable)
	if err != nil {
		return ToolPath{}, err
	}
	return ToolPath{
		WorkspacePath: resolved.workspacePath,
		VFSNamespace:  resolved.vfsNamespace,
		VFSPath:       resolved.vfsPath,
		isVFS:         resolved.isVFS,
	}, nil
}
