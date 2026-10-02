package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goai "github.com/rcarmo/go-ai"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
)

// Golden data from Pi's edit-diff.js and jsdiff (scripts/golden-edit-diff.mjs).
func TestEditDiffMatchesPi(t *testing.T) {
	raw, err := os.ReadFile("../textdiff/testdata/jsdiff.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		DiffStrings []struct {
			Old, New         string
			Diff             string
			FirstChangedLine int
		}
		Edits []struct {
			Name, Content, NewContent, Diff, Error string
			Edits                                  []Edit
			FirstChangedLine                       int
		}
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	for _, c := range g.DiffStrings {
		diff, first := DiffString(c.Old, c.New)
		if diff != c.Diff || first != c.FirstChangedLine {
			t.Fatalf("DiffString(%q,%q) = %q,%d; want %q,%d", c.Old, c.New, diff, first, c.Diff, c.FirstChangedLine)
		}
	}
	for _, c := range g.Edits {
		base, updated, err := ApplyEdits(normalizeToLF(c.Content), c.Edits, "f.txt")
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatalf("%s: error %v, want %q", c.Name, err, c.Error)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		diff, first := DiffString(base, updated)
		if updated != c.NewContent || diff != c.Diff || first != c.FirstChangedLine {
			t.Fatalf("%s: got %q / %q,%d; want %q / %q,%d", c.Name, updated, diff, first, c.NewContent, c.Diff, c.FirstChangedLine)
		}
	}
}

func TestEditArgumentsNormalizesModelShapes(t *testing.T) {
	for _, args := range []map[string]any{
		{"path": "a", "edits": []any{map[string]any{"oldText": "x", "newText": "y"}}},
		{"path": "a", "edits": `[{"oldText":"x","newText":"y"}]`},
		{"path": "a", "edits": `{"oldText":"x","newText":"y"}`},
		{"path": "a", "edits": map[string]any{"oldText": "x", "newText": "y"}},
		{"path": "a", "oldText": "x", "newText": "y"},
	} {
		path, edits, ok := EditArguments(args)
		if !ok || path != "a" || len(edits) != 1 || edits[0] != (Edit{"x", "y"}) {
			t.Fatalf("%v: %q %v %v", args, path, edits, ok)
		}
	}
}

func TestExecuteEditToolKeepsBomAndCRLF(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "f.txt")
	if err := os.WriteFile(file, []byte("\uFEFFone\r\ntwo\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newTestStore(t)
	var details map[string]any
	rt := ToolRuntime{Store: s, WorkspaceRoot: root, SetDetails: func(d map[string]any) { details = d }}
	out, err := ExecuteEditTool(context.Background(), config.RuntimeConfig{WorkspaceRoot: root}, rt, goai.ToolCall{Arguments: map[string]any{"path": "f.txt", "edits": []any{map[string]any{"oldText": "two", "newText": "2"}}}})
	if err != nil || out != "Successfully replaced 1 block(s) in f.txt." {
		t.Fatalf("%q %v", out, err)
	}
	got, _ := os.ReadFile(file)
	if string(got) != "\uFEFFone\r\n2\r\n" {
		t.Fatalf("file %q", got)
	}
	if details["diff"] != " 1 one\n-2 two\n+2 2" || details["firstChangedLine"] != 2 {
		t.Fatalf("details %#v", details)
	}
	if _, err := ExecuteEditTool(context.Background(), config.RuntimeConfig{WorkspaceRoot: root}, rt, goai.ToolCall{Arguments: map[string]any{"path": "missing.txt", "edits": []any{map[string]any{"oldText": "a", "newText": "b"}}}}); err == nil || !strings.HasPrefix(err.Error(), "Could not edit file: missing.txt. Error code: ENOENT.") {
		t.Fatalf("missing: %v", err)
	}
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "edit.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
