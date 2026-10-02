package prompt

import (
	"encoding/json"
	"os"
	"testing"
)

type piInput struct {
	CustomPrompt       string              `json:"customPrompt"`
	SelectedTools      []string            `json:"selectedTools"`
	ToolSnippets       map[string]string   `json:"toolSnippets"`
	ToolGuidelines     map[string][]string `json:"toolGuidelines"`
	PromptGuidelines   []string            `json:"promptGuidelines"`
	AppendSystemPrompt string              `json:"appendSystemPrompt"`
	ContextFiles       []ContextFile       `json:"contextFiles"`
	Skills             []struct {
		Name, Description, FilePath string
	} `json:"skills"`
	Cwd      string            `json:"cwd"`
	Sections map[string]string `json:"sections"`
}

type patchJSON struct {
	Name string  `json:"name"`
	Text *string `json:"text"`
}

type golden struct {
	Cases map[string]struct {
		Input    piInput   `json:"input"`
		Sections []Section `json:"sections"`
	} `json:"cases"`
	Diffs []struct {
		Previous, Current []Section
		Patch             []patchJSON
	} `json:"diffs"`
	Replays []struct {
		Initial  []Section
		Patches  [][]patchJSON
		Sections []Section
		Text     string
	} `json:"replays"`
}

func load(t *testing.T) golden {
	t.Helper()
	raw, err := os.ReadFile("testdata/pi-system-prompt.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func patches(p []patchJSON) []Patch {
	out := make([]Patch, len(p))
	for i, x := range p {
		out[i] = Patch{x.Name, x.Text}
	}
	return out
}

// Sections from Pi's buildSystemPromptSections (scripts/golden-system-prompt.mjs);
// preamble and docs are gi's wording and are not compared.
func TestBuildSectionsMatchesPi(t *testing.T) {
	for name, c := range load(t).Cases {
		in := c.Input
		o := Options{CustomPrompt: in.CustomPrompt, Preamble: "gi preamble", Docs: "gi docs", SelectedTools: in.SelectedTools, ToolSnippets: in.ToolSnippets, ToolGuidelines: in.ToolGuidelines, PromptGuidelines: in.PromptGuidelines, AppendPrompt: in.AppendSystemPrompt, ContextFiles: in.ContextFiles, Cwd: in.Cwd}
		for _, s := range in.Skills {
			o.Skills = append(o.Skills, Skill{s.Name, s.Description, s.FilePath})
		}
		if text, ok := in.Sections["mcp_servers"]; ok {
			o.Extra = []Section{{"mcp_servers", text}}
		}
		got, err := BuildSections(o)
		if err != nil {
			t.Fatal(err)
		}
		var want []Section
		for _, s := range c.Sections {
			if (s.Name == "preamble" && in.CustomPrompt == "") || s.Name == "docs" {
				continue
			}
			want = append(want, s)
		}
		var cmp []Section
		for _, s := range got {
			if (s.Name == "preamble" && in.CustomPrompt == "") || s.Name == "docs" {
				continue
			}
			cmp = append(cmp, s)
		}
		if len(cmp) != len(want) {
			t.Fatalf("%s: sections %v, want %v", name, names(cmp), names(want))
		}
		for i := range cmp {
			if cmp[i] != want[i] {
				t.Fatalf("%s.%s:\n got %q\nwant %q", name, want[i].Name, cmp[i].Text, want[i].Text)
			}
		}
	}
}

func names(s []Section) []string {
	var out []string
	for _, x := range s {
		out = append(out, x.Name)
	}
	return out
}

func TestDiffAndReplayMatchPi(t *testing.T) {
	g := load(t)
	for _, d := range g.Diffs {
		got := Diff(d.Previous, d.Current)
		want := patches(d.Patch)
		if len(got) != len(want) {
			t.Fatalf("patch %v, want %v", got, want)
		}
		for i := range got {
			if got[i].Name != want[i].Name || (got[i].Text == nil) != (want[i].Text == nil) || got[i].Text != nil && *got[i].Text != *want[i].Text {
				t.Fatalf("patch %d: %+v, want %+v", i, got[i], want[i])
			}
		}
	}
	for _, r := range g.Replays {
		sections := r.Initial
		for _, p := range r.Patches {
			sections = Apply(sections, patches(p))
		}
		if names(sections) == nil || len(sections) != len(r.Sections) {
			t.Fatalf("replay %v, want %v", names(sections), names(r.Sections))
		}
		for i := range sections {
			if sections[i] != r.Sections[i] {
				t.Fatalf("replay %d: %+v want %+v", i, sections[i], r.Sections[i])
			}
		}
		if Render(sections) != r.Text {
			t.Fatal("rendered text differs from pi-ai's getSystemMessageText")
		}
	}
	if Diff([]Section{{"a", "x"}}, []Section{{"a", "x"}}) != nil {
		t.Fatal("no-change diff not nil")
	}
}

func TestShellRuleAndInvalidSection(t *testing.T) {
	got, _ := BuildSections(Options{SelectedTools: []string{"shell"}, Cwd: "/w"})
	if got[2].Name != "rules" || got[2].Text != "<rules>\n- Use shell for file operations like ls, rg, find\n- Be concise in your responses\n- Show file paths clearly when working with files\n</rules>" {
		t.Fatalf("%q", got[2].Text)
	}
	if _, err := BuildSections(Options{Extra: []Section{{"Bad Name", "x"}}}); err == nil {
		t.Fatal("invalid name accepted")
	}
}
