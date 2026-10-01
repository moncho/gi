package config

import (
	"os"
	"path/filepath"
)

// Config files are looked up in gi's own directories first, then Pi's: for
// each file the first existing location wins (no merging between the two).
//
//	user level:    ~/.gi/agent/<file>, then ~/.pi/agent/<file>
//	project level: <workspace>/.gi/<file>, then <workspace>/.pi/<file>
//
// This lets gi-specific configuration override Pi's while still reading an
// existing Pi setup unchanged.

// ConfigDirNames lists the per-project and per-user config directory names in
// lookup order.
var ConfigDirNames = []string{".gi", ".pi"}

// UserConfigCandidates returns the user-level locations for a config file in
// lookup order.
func UserConfigCandidates(rel ...string) []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	out := make([]string, 0, len(ConfigDirNames))
	for _, dir := range ConfigDirNames {
		out = append(out, filepath.Join(append([]string{home, dir, "agent"}, rel...)...))
	}
	return out
}

// ProjectConfigCandidates returns the project-level locations for a config
// file in lookup order.
func ProjectConfigCandidates(workspace string, rel ...string) []string {
	out := make([]string, 0, len(ConfigDirNames))
	for _, dir := range ConfigDirNames {
		out = append(out, filepath.Join(append([]string{workspace, dir}, rel...)...))
	}
	return out
}

// FirstExisting returns the first candidate that exists, or the last one (the
// Pi location) when none do, so callers report a sensible missing path.
func FirstExisting(candidates []string) string {
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	return candidates[len(candidates)-1]
}

// UserConfigFile resolves a user-level config file (.gi first, then .pi).
func UserConfigFile(rel ...string) string { return FirstExisting(UserConfigCandidates(rel...)) }

// ProjectConfigFile resolves a project-level config file (.gi first, then .pi).
func ProjectConfigFile(workspace string, rel ...string) string {
	return FirstExisting(ProjectConfigCandidates(workspace, rel...))
}
