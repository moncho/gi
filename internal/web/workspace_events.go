package web

import (
	"os"
	"path/filepath"
	"sort"
)

// Piclaw's explorer needs bounded subtree snapshots as well as changed paths.
// Coalesce bursts by parent directory and collapse wide bursts to the root.
func (s *Server) workspaceUpdatePayload(paths []string) map[string]any {
	parents := map[string]struct{}{}
	for _, path := range paths {
		rel, ok := s.workspaceRel(path)
		if !ok {
			continue
		}
		parents[filepath.Dir(rel)] = struct{}{}
	}
	if len(parents) > 8 {
		parents = map[string]struct{}{".": {}}
	}
	if _, all := parents["."]; all {
		parents = map[string]struct{}{".": {}}
	}
	dirs := make([]string, 0, len(parents))
	for p := range parents {
		dirs = append(dirs, p)
	}
	sort.Strings(dirs)
	root, err := os.OpenRoot(s.workspaceRootPath())
	if err != nil {
		return map[string]any{"updates": []any{}}
	}
	defer root.Close()
	remaining := workspaceTreeNodeLimit
	updates := make([]any, 0, len(dirs))
	for _, dir := range dirs {
		depth := 3
		if dir == "." {
			depth = 4
		}
		node, err := readWorkspaceTree(root, dir, depth, true, false, &remaining)
		truncated := err != nil
		if err != nil {
			info, statErr := root.Stat(dir)
			if statErr != nil || !info.IsDir() {
				continue
			}
			node = workspaceNode{Name: filepath.Base(dir), Path: filepath.ToSlash(dir), Type: "dir"}
		}
		if dir == "." {
			node.Name = filepath.Base(s.workspaceRootPath())
		}
		updates = append(updates, map[string]any{"path": filepath.ToSlash(dir), "root": node, "truncated": truncated, "changed_paths": paths})
	}
	return map[string]any{"updates": updates}
}
