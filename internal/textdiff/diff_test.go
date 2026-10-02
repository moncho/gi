package textdiff

import (
	"encoding/json"
	"os"
	"testing"
)

type goldenCase struct {
	Old     string   `json:"old"`
	New     string   `json:"new"`
	Changes []Change `json:"changes"`
}

func (c *Change) UnmarshalJSON(b []byte) error {
	var v struct {
		Value          string `json:"value"`
		Count          int    `json:"count"`
		Added, Removed bool
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*c = Change{Value: v.Value, Count: v.Count, Added: v.Added, Removed: v.Removed}
	return nil
}

func loadGolden(t *testing.T) (lines, words []goldenCase) {
	t.Helper()
	raw, err := os.ReadFile("testdata/jsdiff.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct{ Lines, Words []goldenCase }
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g.Lines, g.Words
}

func check(t *testing.T, kind string, c goldenCase, got []Change) {
	t.Helper()
	if len(got) != len(c.Changes) {
		t.Fatalf("%s %q→%q: got %d changes %+v, want %+v", kind, c.Old, c.New, len(got), got, c.Changes)
	}
	for i := range got {
		if got[i] != c.Changes[i] {
			t.Fatalf("%s %q→%q change %d: got %+v, want %+v", kind, c.Old, c.New, i, got[i], c.Changes[i])
		}
	}
}

// TestJSDiffGolden compares with jsdiff 8.0.4 (scripts/golden-edit-diff.mjs).
func TestJSDiffGolden(t *testing.T) {
	lines, words := loadGolden(t)
	for _, c := range lines {
		check(t, "lines", c, Lines(c.Old, c.New))
	}
	for _, c := range words {
		check(t, "words", c, Words(c.Old, c.New))
	}
}
