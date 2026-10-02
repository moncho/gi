package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSkillsAndToolManifests(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".gi", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: Demo skill\n---\nUse it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	toolDir := filepath.Join(root, ".gi", "tools")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "hello.json"), []byte(`{"name":"hello","description":"Hello tool","engine":"js","script":"'ok'"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Discover(root)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(d.Skills) != 1 || d.Skills[0].Name != "demo" || d.Skills[0].Description != "Demo skill" {
		t.Fatalf("skills: %#v", d.Skills)
	}
	if len(d.Tools) != 1 || d.Tools[0].Name != "hello" {
		t.Fatalf("tools: %#v", d.Tools)
	}
}

// Pi's rules: SKILL.md needs a description in its front matter; the name
// falls back to the directory and is checked against the Agent Skills spec
// (warnings only).
func TestDiscoverSkillsFollowsPiRules(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gi/skills/Bad_Name/SKILL.md", "---\ndescription: Named by its directory\n---\n")
	write(".gi/skills/legacy/SKILL.md", "# Legacy skill\n\nUse it.")
	write(".gi/skills/nested/group/deep/SKILL.md", "---\nname: deep\ndescription: Found by recursion\n---\n")
	write(".gi/skills/nested/group/deep/inner/SKILL.md", "---\nname: inner\ndescription: Below a skill root\n---\n")
	write(".gi/skills/loose.md", "---\nname: loose\ndescription: A root .md file\n---\n")
	write(".gi/skills/notes.md", "no front matter: not a skill")
	write(".gi/skills/.hidden/SKILL.md", "---\nname: hidden\ndescription: x\n---\n")
	write(".gi/skills/ignored/SKILL.md", "---\nname: ignored\ndescription: x\n---\n")
	write(".gi/skills/.gitignore", "ignored/\n")
	write(".gi/skills/quiet/SKILL.md", "---\nname: quiet\ndescription: Explicit only\ndisable-model-invocation: true\n---\n")
	write(".pi/skills/deep/SKILL.md", "---\nname: deep\ndescription: Shadowed by .gi\n---\n")
	skills, err := DiscoverSkills(root)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Skill{}
	for _, s := range skills {
		byName[s.Name] = s
	}
	for _, name := range []string{"Bad_Name", "deep", "loose", "quiet"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("missing %s: %+v", name, skills)
		}
	}
	for _, name := range []string{"legacy", "inner", "notes", "hidden", "ignored"} {
		if _, ok := byName[name]; ok {
			t.Fatalf("%s must not load", name)
		}
	}
	if byName["deep"].Description != "Found by recursion" || byName["deep"].Source != "project" {
		t.Fatalf("deep: %+v", byName["deep"])
	}
	if !byName["quiet"].DisableModelInvocation || len(byName["Bad_Name"].Warnings) == 0 {
		t.Fatalf("flags: %+v %+v", byName["quiet"], byName["Bad_Name"])
	}
}

// User skills (gi's agent directory, then Pi's) come before project skills;
// the first of a name wins.
func TestDiscoverSkillsUserDirsFirst(t *testing.T) {
	root, user := t.TempDir(), t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", user)
	for path, body := range map[string]string{
		filepath.Join(user, "skills", "shared", "SKILL.md"): "---\nname: shared\ndescription: user copy\n---\n",
		filepath.Join(root, ".pi", "skills", "shared", "SKILL.md"): "---\nname: shared\ndescription: project copy\n---\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	skills, _ := DiscoverSkills(root)
	if len(skills) != 1 || skills[0].Description != "user copy" || skills[0].Source != "user" {
		t.Fatalf("%+v", skills)
	}
}
