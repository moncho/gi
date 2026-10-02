package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Config files are looked up in gi's own directories first, then Pi's: for
// each file the first existing location wins (no merging between the two).
//
//	user level:    $GI_CODING_AGENT_DIR or ~/.gi/agent, then
//	               $PI_CODING_AGENT_DIR or ~/.pi/agent (Pi's getAgentDir)
//	project level: <workspace>/.gi/<file>, then <workspace>/.pi/<file>
//
// This lets gi-specific configuration override Pi's while still reading an
// existing Pi setup unchanged. A file is written where it was read; when
// neither location has it, it is created in Pi's location, so it stays shared
// with Pi. Directories of items (skills, tools, extensions) are scanned in
// the same order, the first item of a name winning.

// ConfigDirNames lists the per-project and per-user config directory names in
// lookup order.
var ConfigDirNames = []string{".gi", ".pi"}

// UserConfigCandidates returns the user-level locations for a config file in
// lookup order.
func UserConfigCandidates(rel ...string) []string {
	dirs := UserConfigDirs()
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		out = append(out, filepath.Join(append([]string{dir}, rel...)...))
	}
	return out
}

// UserConfigDirs are the user-level config directories in lookup order:
// gi's (GI_CODING_AGENT_DIR, else ~/.gi/agent), then Pi's
// (PI_CODING_AGENT_DIR, else ~/.pi/agent).
func UserConfigDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	out := make([]string, 0, len(ConfigDirNames))
	for _, dir := range ConfigDirNames {
		env := "GI_CODING_AGENT_DIR"
		if dir == ".pi" {
			env = "PI_CODING_AGENT_DIR"
		}
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			out = append(out, v)
		} else {
			out = append(out, filepath.Join(home, dir, "agent"))
		}
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

// ProjectConfigDirName is the project config directory holding a file:
// ".gi" when <workspace>/.gi/<file> exists, else ".pi" (read and written
// there).
func ProjectConfigDirName(workspace string, rel ...string) string {
	if _, err := os.Stat(filepath.Join(append([]string{workspace, ".gi"}, rel...)...)); err == nil {
		return ".gi"
	}
	return ".pi"
}

// ProjectConfigFile resolves a project-level config file (.gi first, then .pi).
func ProjectConfigFile(workspace string, rel ...string) string {
	return FirstExisting(ProjectConfigCandidates(workspace, rel...))
}
