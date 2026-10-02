package config

import (
	"os"
	"path/filepath"
	"testing"
)

// buildContextFixture lays out: an agent dir with AGENTS.md; root/AGENTS.md;
// root/a with AGENTS.override.md and AGENTS.md (override wins);
// root/a/b with CLAUDE.md; and a git repo root/a/b/repo with a nested
// linked worktree root/a/b/repo/wt, both with AGENTS.md.
func buildContextFixture(t *testing.T, base string) (agent, cwd, worktree string) {
	t.Helper()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("agent/AGENTS.md", "global")
	write("root/AGENTS.md", "\uFEFFroot")
	write("root/a/AGENTS.override.md", "override")
	write("root/a/AGENTS.md", "shadowed by override")
	write("root/a/b/CLAUDE.md", "claude")
	write("root/a/b/repo/.git/HEAD", "ref: refs/heads/main\n")
	write("root/a/b/repo/AGENTS.md", "main repo")
	write("root/a/b/repo/.git/worktrees/wt/HEAD", "ref: refs/heads/wt\n")
	write("root/a/b/repo/.git/worktrees/wt/commondir", "../..\n")
	write("root/a/b/repo/wt/.git", "gitdir: "+filepath.Join(base, "root/a/b/repo/.git/worktrees/wt")+"\n")
	write("root/a/b/repo/wt/AGENTS.md", "worktree")
	return filepath.Join(base, "agent"), filepath.Join(base, "root/a/b"), filepath.Join(base, "root/a/b/repo/wt")
}

// Order and shadowing follow Pi's loadProjectContextFiles (checked against
// Pi on the same layout).
func TestLoadContextFilesLikePi(t *testing.T) {
	base := t.TempDir()
	agent, cwd, worktree := buildContextFixture(t, base)
	t.Setenv("GI_CODING_AGENT_DIR", filepath.Join(base, "no-gi-agent"))
	t.Setenv("PI_CODING_AGENT_DIR", agent)
	paths := func(files []ContextFile) []string {
		var out []string
		for _, f := range files {
			rel, _ := filepath.Rel(base, f.Path)
			out = append(out, filepath.ToSlash(rel)+"="+f.Content)
		}
		return out
	}
	check := func(got []ContextFile, want ...string) {
		t.Helper()
		g := paths(got)
		// Ancestors above base (e.g. /tmp) may hold context files of their own.
		for len(g) > len(want) && len(g) > 1 {
			g = append(g[:1], g[2:]...)
		}
		if len(g) != len(want) {
			t.Fatalf("got %v, want %v", g, want)
		}
		for i := range g {
			if g[i] != want[i] {
				t.Fatalf("got %v, want %v", g, want)
			}
		}
	}
	check(LoadContextFiles(cwd), "agent/AGENTS.md=global", "root/AGENTS.md=root", "root/a/AGENTS.override.md=override", "root/a/b/CLAUDE.md=claude")
	// In the nested linked worktree, its own AGENTS.md shadows the main repo's.
	check(LoadContextFiles(worktree), "agent/AGENTS.md=global", "root/AGENTS.md=root", "root/a/AGENTS.override.md=override", "root/a/b/CLAUDE.md=claude", "root/a/b/repo/wt/AGENTS.md=worktree")
	// gi's agent directory wins over Pi's.
	giAgent := filepath.Join(base, "gi-agent")
	if err := os.MkdirAll(giAgent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(giAgent, "AGENTS.md"), []byte("gi global"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GI_CODING_AGENT_DIR", giAgent)
	check(LoadContextFiles(cwd), "gi-agent/AGENTS.md=gi global", "root/AGENTS.md=root", "root/a/AGENTS.override.md=override", "root/a/b/CLAUDE.md=claude")
}
