// Package prompt builds gi's system prompt as Pi does (core/system-prompt.js):
// named, ordered sections (preamble, tools, rules, docs, addendum,
// project_context, skills, cwd, then extension sections such as
// mcp_servers), each replaceable on its own so a change mid-conversation is
// appended as a section patch instead of rewriting the cached prompt.
//
// The structure, tool and rule assembly, project context, skills and cwd
// rendering are Pi's; the preamble and docs are gi's wording, and gi's shell
// tool (`shell`) takes the place of Pi's `bash` in the file-operation rule.
package prompt

import (
	"fmt"
	"regexp"
	"strings"
)

// Section is one named prompt section. Text is what the model sees: the
// preamble as is, every other section wrapped in <name>…</name>.
type Section struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// ContextFile is a project instructions file (AGENTS.md).
type ContextFile struct {
	Path    string
	Content string
}

// Skill is a skill offered to the model.
type Skill struct {
	Name        string
	Description string
	Location    string
}

// Options are Pi's BuildSystemPromptOptions with gi's preamble and docs.
type Options struct {
	// CustomPrompt replaces the preamble and drops the tools, rules and docs
	// sections (Pi's customPrompt).
	CustomPrompt string
	// Preamble and Docs are gi's default preamble and docs section body.
	Preamble, Docs   string
	SelectedTools    []string
	ToolSnippets     map[string]string
	ToolGuidelines   map[string][]string
	PromptGuidelines []string
	AppendPrompt     string
	ContextFiles     []ContextFile
	Skills           []Skill
	Cwd              string
	// Extra are extension sections (e.g. mcp_servers): name and body, in
	// order; empty bodies are omitted.
	Extra []Section
}

var sectionName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// buildRules is Pi's buildRules: the shell rule when no file-search tools
// are selected, the selected tools' guidelines, the prompt guidelines, then
// Pi's two closing rules; trimmed and deduplicated.
func buildRules(selected []string, toolGuidelines map[string][]string, promptGuidelines []string) string {
	var rules []string
	seen := map[string]bool{}
	add := func(rule string) {
		rule = strings.TrimSpace(rule)
		if rule == "" || seen[rule] {
			return
		}
		seen[rule] = true
		rules = append(rules, rule)
	}
	hasBash := contains(selected, "bash")
	hasShell := contains(selected, "shell")
	hasPowerShell := contains(selected, "powershell")
	if (hasBash || hasShell || hasPowerShell) && !contains(selected, "grep") && !contains(selected, "find") && !contains(selected, "ls") {
		switch {
		case (hasBash || hasShell) && hasPowerShell:
			add("Use bash or PowerShell for file operations like listing, searching, and finding files")
		case hasPowerShell:
			add("Use PowerShell for file operations like listing, searching, and finding files")
		case hasBash:
			add("Use bash for file operations like ls, rg, find")
		default:
			add("Use shell for file operations like ls, rg, find")
		}
	}
	for _, name := range selected {
		for _, rule := range toolGuidelines[name] {
			add(rule)
		}
	}
	for _, rule := range promptGuidelines {
		add(rule)
	}
	add("Be concise in your responses")
	add("Show file paths clearly when working with files")
	for i, rule := range rules {
		rules[i] = "- " + rule
	}
	return strings.Join(rules, "\n")
}

func renderProjectContext(files []ContextFile) string {
	parts := []string{"Project-specific instructions and guidelines:"}
	for _, f := range files {
		parts = append(parts, fmt.Sprintf("<project_instructions path=\"%s\">\n%s\n</project_instructions>", f.Path, f.Content))
	}
	return strings.Join(parts, "\n\n")
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

// formatSkills is Pi's formatSkillsForPrompt (trimmed, as the prompt uses it).
func formatSkills(skills []Skill, readTool string) string {
	if len(skills) == 0 {
		return ""
	}
	how := "Use the read tool to load a skill's file when the task matches its description."
	if readTool != "read" {
		how = "Use " + readTool + " to load a skill's file when the task matches its description."
	}
	lines := []string{
		"The following skills provide specialized instructions for specific tasks.",
		how,
		"When a skill file references a relative path, resolve it against the skill directory (parent of SKILL.md / dirname of the path) and use that absolute path in tool commands.",
		"",
		"<available_skills>",
	}
	for _, s := range skills {
		lines = append(lines, "  <skill>",
			"    <name>"+xmlEscaper.Replace(s.Name)+"</name>",
			"    <description>"+xmlEscaper.Replace(s.Description)+"</description>",
			"    <location>"+xmlEscaper.Replace(s.Location)+"</location>",
			"  </skill>")
	}
	lines = append(lines, "</available_skills>")
	return strings.Join(lines, "\n")
}

// BuildSections is Pi's buildSystemPromptSections.
func BuildSections(o Options) ([]Section, error) {
	for _, s := range o.Extra {
		if !sectionName.MatchString(s.Name) || s.Name == "preamble" {
			return nil, fmt.Errorf("invalid system prompt section name: %s", s.Name)
		}
	}
	var out []Section
	add := func(name, body string) {
		for i := range out {
			if out[i].Name == name {
				out[i].Text = body
				return
			}
		}
		out = append(out, Section{name, body})
	}
	if o.CustomPrompt != "" {
		add("preamble", o.CustomPrompt)
	} else {
		add("preamble", o.Preamble)
		var tools []string
		for _, name := range o.SelectedTools {
			if snippet := o.ToolSnippets[name]; snippet != "" {
				tools = append(tools, "- "+name+": "+snippet)
			}
		}
		list := "(none)"
		if len(tools) > 0 {
			list = strings.Join(tools, "\n")
		}
		add("tools", list+"\n\nIn addition to the tools above, you may have access to other custom tools depending on the project.")
		add("rules", buildRules(o.SelectedTools, o.ToolGuidelines, o.PromptGuidelines))
		if o.Docs != "" {
			add("docs", o.Docs)
		}
	}
	if o.AppendPrompt != "" {
		add("addendum", o.AppendPrompt)
	}
	if len(o.ContextFiles) > 0 {
		add("project_context", renderProjectContext(o.ContextFiles))
	}
	readTool := ""
	for _, t := range []string{"read", "bash", "shell"} {
		if contains(o.SelectedTools, t) {
			readTool = t
			break
		}
	}
	if readTool != "" {
		if skills := formatSkills(o.Skills, readTool); skills != "" {
			add("skills", skills)
		}
	}
	add("cwd", strings.ReplaceAll(o.Cwd, `\`, "/"))
	for _, s := range o.Extra {
		if s.Text != "" {
			add(s.Name, s.Text)
		}
	}
	for i := range out {
		if out[i].Name != "preamble" {
			out[i].Text = Wrap(out[i].Name, out[i].Text)
		}
	}
	return out, nil
}

// Wrap is a section body in its tags.
func Wrap(name, body string) string { return "<" + name + ">\n" + body + "\n</" + name + ">" }

// Render is the prompt text of sections (pi-ai getSystemMessageText).
func Render(sections []Section) string {
	var parts []string
	for _, s := range sections {
		if s.Text != "" {
			parts = append(parts, s.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// Patch changes one section: Text nil removes it.
type Patch struct {
	Name string  `json:"name"`
	Text *string `json:"text"`
}

// Diff is Pi's diffSystemPromptSections: changed or added sections in
// current order, then removed ones; nil when nothing changed.
func Diff(previous, current []Section) []Patch {
	prev := map[string]string{}
	for _, s := range previous {
		prev[s.Name] = s.Text
	}
	var patch []Patch
	names := map[string]bool{}
	for _, s := range current {
		names[s.Name] = true
		if old, ok := prev[s.Name]; !ok || old != s.Text {
			text := s.Text
			patch = append(patch, Patch{s.Name, &text})
		}
	}
	for _, s := range previous {
		if !names[s.Name] {
			patch = append(patch, Patch{s.Name, nil})
		}
	}
	return patch
}

// Apply replays a patch onto sections like pi-ai's getCurrentSystemMessage:
// a replaced section keeps its place, a new one goes last, a removed one
// goes.
func Apply(sections []Section, patch []Patch) []Section {
	out := append([]Section(nil), sections...)
	for _, p := range patch {
		i := -1
		for j := range out {
			if out[j].Name == p.Name {
				i = j
				break
			}
		}
		switch {
		case p.Text == nil && i >= 0:
			out = append(out[:i], out[i+1:]...)
		case p.Text != nil && i >= 0:
			out[i].Text = *p.Text
		case p.Text != nil:
			out = append(out, Section{p.Name, *p.Text})
		}
	}
	return out
}
