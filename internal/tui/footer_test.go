package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/gi/internal/config"
)

func TestEffectiveThinkingFollowsPiDefaults(t *testing.T) {
	c := &chatTUI{cfg: config.RuntimeConfig{}}
	if got := c.effectiveThinking("test", "unknown-model", ""); got != "medium" {
		t.Fatalf("Pi default thinking level = %q", got)
	}
	c.cfg.DefaultThinkingLevel = "high"
	if got := c.effectiveThinking("test", "unknown-model", ""); got != "high" {
		t.Fatalf("configured default = %q", got)
	}
	if got := c.effectiveThinking("test", "unknown-model", "low"); got != "low" {
		t.Fatalf("session level = %q", got)
	}
}

func TestGitBranchFromSubdirectoryAndWorktreeFile(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/.git/HEAD", "ref: refs/heads/feature/x\n")
	mustWrite(t, root+"/sub/dir/.keep", "")
	c := &chatTUI{}
	if got := c.gitBranchName(root + "/sub/dir"); got != "feature/x" {
		t.Fatalf("parent repo branch = %q", got)
	}
	wt := t.TempDir()
	mustWrite(t, root+"/.git/worktrees/wt/HEAD", "0123456789abcdef\n")
	mustWrite(t, wt+"/.git", "gitdir: "+root+"/.git/worktrees/wt\n")
	if got := c.gitBranchName(wt); got != "detached" {
		t.Fatalf("worktree detached HEAD = %q", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
