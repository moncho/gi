package skills

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"gopkg.in/yaml.v2"
)

// Pi's skill directory rules (core/skills.js loadSkillsFromDirInternal):
//   - a directory containing SKILL.md is one skill; its subdirectories are
//     not searched;
//   - otherwise .md files directly in the skills root are skills (they need a
//     description), and subdirectories are searched for SKILL.md;
//   - dot entries and node_modules are skipped, symlinks are followed, and
//     .gitignore/.ignore/.fdignore files exclude paths below them.

const (
	maxSkillNameLength        = 64
	maxSkillDescriptionLength = 1024
)

var ignoreFileNames = []string{".gitignore", ".ignore", ".fdignore"}

func loadSkillsFromDir(dir, source string) []Skill {
	return scanSkillsDir(dir, dir, source, true, nil)
}

// ignorePatterns reads a directory's ignore files as patterns scoped to it.
func ignorePatterns(dir, root string) []gitignore.Pattern {
	var domain []string
	if rel, err := filepath.Rel(root, dir); err == nil && rel != "." {
		domain = strings.Split(filepath.ToSlash(rel), "/")
	}
	var out []gitignore.Pattern
	for _, name := range ignoreFileNames {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, `\#`) {
				continue
			}
			out = append(out, gitignore.ParsePattern(line, domain))
		}
	}
	return out
}

func relParts(root, path string) []string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil
	}
	return strings.Split(filepath.ToSlash(rel), "/")
}

func scanSkillsDir(dir, root, source string, includeRootFiles bool, patterns []gitignore.Pattern) []Skill {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	patterns = append(append([]gitignore.Pattern(nil), patterns...), ignorePatterns(dir, root)...)
	ignored := func(path string, isDir bool) bool {
		return len(patterns) > 0 && gitignore.NewMatcher(patterns).Match(relParts(root, path), isDir)
	}
	kind := func(path string, entry os.DirEntry) (isDir, isFile, ok bool) {
		if entry.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(path)
			if err != nil {
				return false, false, false
			}
			return info.IsDir(), info.Mode().IsRegular(), true
		}
		return entry.IsDir(), entry.Type().IsRegular(), true
	}
	for _, entry := range entries {
		if entry.Name() != "SKILL.md" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if _, isFile, ok := kind(path, entry); !ok || !isFile || ignored(path, false) {
			continue
		}
		if skill, ok := loadSkillFile(path, source); ok {
			return []Skill{skill}
		}
		return nil
	}
	var out []Skill
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		path := filepath.Join(dir, name)
		isDir, isFile, ok := kind(path, entry)
		if !ok || ignored(path, isDir) {
			continue
		}
		if isDir {
			out = append(out, scanSkillsDir(path, root, source, false, patterns)...)
			continue
		}
		if !isFile || !includeRootFiles || !strings.HasSuffix(name, ".md") {
			continue
		}
		if skill, ok := loadSkillFile(path, source); ok {
			out = append(out, skill)
		}
	}
	return out
}

// parseFrontmatter is Pi's: YAML between a leading "---" line and the next
// "\n---"; no front matter is an empty map.
func parseFrontmatter(content string) (map[string]any, error) {
	content = strings.TrimPrefix(content, "\uFEFF")
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	if !strings.HasPrefix(content, "---") {
		return map[string]any{}, nil
	}
	end := strings.Index(content[3:], "\n---")
	if end == -1 {
		return map[string]any{}, nil
	}
	end += 3
	yamlText := "" // JavaScript's slice(4, end)
	if end > 4 {
		yamlText = content[4:end]
	}
	if yamlText == "" {
		return map[string]any{}, nil
	}
	var raw map[any]any
	if err := yaml.Unmarshal([]byte(yamlText), &raw); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for k, v := range raw {
		if ks, ok := k.(string); ok {
			out[ks] = v
		}
	}
	return out, nil
}

var skillNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// validateSkillName is Pi's Agent Skills name check (warnings only).
func validateSkillName(name string) []string {
	var errs []string
	if n := len([]rune(name)); n > maxSkillNameLength {
		errs = append(errs, "name exceeds 64 characters ("+itoa(n)+")")
	}
	if !skillNamePattern.MatchString(name) {
		errs = append(errs, "name contains invalid characters (must be lowercase a-z, 0-9, hyphens only)")
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		errs = append(errs, "name must not start or end with a hyphen")
	}
	if strings.Contains(name, "--") {
		errs = append(errs, "name must not contain consecutive hyphens")
	}
	return errs
}

func itoa(n int) string { return strconv.Itoa(n) }

// loadSkillFile is Pi's loadSkillFromFile: the description is required; the
// name is the front matter's or the directory's.
func loadSkillFile(path, source string) (Skill, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, false
	}
	front, err := parseFrontmatter(string(data))
	if err != nil {
		return Skill{}, false
	}
	description, _ := front["description"].(string)
	if strings.TrimSpace(description) == "" {
		return Skill{}, false
	}
	dir := filepath.Dir(path)
	name, _ := front["name"].(string)
	if name == "" {
		name = filepath.Base(dir)
	}
	skill := Skill{Name: name, Description: description, Path: path, BaseDir: dir, Source: source}
	if n := len([]rune(description)); n > maxSkillDescriptionLength {
		skill.Warnings = append(skill.Warnings, "description exceeds 1024 characters ("+itoa(n)+")")
	}
	skill.Warnings = append(skill.Warnings, validateSkillName(name)...)
	if v, ok := front["disable-model-invocation"].(bool); ok && v {
		skill.DisableModelInvocation = true
	}
	return skill, true
}
