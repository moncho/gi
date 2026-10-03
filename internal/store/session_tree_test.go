package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// A /tree branch records its source; a plain copy of it (/fork, /clone)
// starts its own tree. Labels are set and removed in place.
func TestTreeBranchesAndLabels(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "gi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if _, err := s.CreateSession(ctx, "root", "@agent", map[string]any{"status": "idle"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMessage(ctx, "m1", "root", "user", "hi", nil); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 3, 21, 12, 0, 0, time.UTC)
	if err := s.SetTreeLabel(ctx, "root", "m1", "start", at); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTreeLabel(ctx, "root", "m2", "other", at); err != nil {
		t.Fatal(err)
	}
	branch, err := s.BranchSessionBefore(ctx, "root", "b1", "@agent1", "agent1", "")
	if err != nil || TreeParent(*branch) != "root" {
		t.Fatalf("branch %+v %v", branch, err)
	}
	copied, err := s.CloneSessionBefore(ctx, "b1", "c1", "@agent2", "agent2", "")
	if err != nil || TreeParent(*copied) != "" || len(TreeLabels(*copied)) != 0 {
		t.Fatalf("copy %+v %v", copied, err)
	}
	if err := s.SetTreeLabel(ctx, "root", "m2", "", at); err != nil {
		t.Fatal(err)
	}
	root, err := s.GetSession(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}
	labels := TreeLabels(*root)
	if len(labels) != 1 || labels["m1"] != (TreeLabel{Label: "start", Time: "2026-10-03T21:12:00.000Z"}) || root.State["status"] != "idle" {
		t.Fatalf("labels %+v state %+v", labels, root.State)
	}
}
