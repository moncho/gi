package tui

import (
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/skills"
	"github.com/rcarmo/gi/internal/version"
)

func headerText(t *testing.T, c *chatTUI, width int) string {
	t.Helper()
	blocks := c.buildTranscriptRenderableBlocks(c.visibleTranscript())
	if len(blocks) == 0 || blocks[0].Kind != startupHeaderKind {
		t.Fatalf("first block %+v", blocks)
	}
	rows := c.transcriptRowsAtWidth(width)
	var out []string
	for _, r := range rows {
		out = append(out, strings.TrimRight(r.text, " "))
	}
	return strings.Join(out, "\n")
}

// gi's startup header: name, version and gi's keys collapsed; the full key
// list and loaded resources on Ctrl+O; quietStartup like Pi.
func TestStartupHeader(t *testing.T) {
	c := &chatTUI{startupHeader: true, transcriptExpanded: map[string]bool{},
		cfg: config.RuntimeConfig{AssistantName: "Gi", EnabledModels: []string{"a/one", "b/two"},
			Discovery: skills.Discovery{Skills: []skills.Skill{{Name: "zeta", Path: "/s/zeta/SKILL.md"}, {Name: "alpha", Path: "/s/alpha/SKILL.md"}}}}}
	text := headerText(t, c, 100)
	for _, want := range []string{
		"gi " + version.String(),
		"Esc interrupt · Ctrl+C quit · / commands · ! bash · Ctrl+O more",
		"Press Ctrl+O to show full startup help and loaded resources.",
		"Model scope: a/one, b/two (Ctrl+L to cycle)",
		"[Skills]\n  alpha, zeta",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("collapsed header lacks %q:\n%s", want, text)
		}
	}
	c.toggleToolOutput()
	text = headerText(t, c, 100)
	for _, want := range []string{"Esc to interrupt", "Ctrl+O to expand tools", "!! to run bash (no context)", "[Skills]\n  /s/alpha/SKILL.md\n  /s/zeta/SKILL.md"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expanded header lacks %q:\n%s", want, text)
		}
	}
	c.toggleToolOutput()
	c.cfg.QuietStartup = "header"
	text = headerText(t, c, 100)
	if strings.Contains(text, "[Skills]") || strings.Contains(text, "Model scope") || !strings.Contains(text, "Press Ctrl+O to show full startup help.") {
		t.Fatalf("quietStartup header:\n%s", text)
	}
	c.cfg.QuietStartup = "true"
	if blocks := c.buildTranscriptRenderableBlocks(c.visibleTranscript()); len(blocks) > 0 && blocks[0].Kind == startupHeaderKind {
		t.Fatal("quietStartup true still shows the header")
	}
}
