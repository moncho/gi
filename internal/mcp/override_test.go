package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Project overrides of global servers load and save as Pi's own
// loadMcpConfig and updateMcpServerConfig do
// (scripts/golden-mcp-overrides.mjs).
func TestProjectOverridesMatchPi(t *testing.T) {
	raw, err := os.ReadFile("testdata/pi-mcp-overrides.json")
	if err != nil {
		t.Fatal(err)
	}
	type server struct {
		Name, Scope, Source, Override, Exposure string
		Enabled                                 bool
		ToolExposure                            map[string]string
	}
	var g struct {
		Global json.RawMessage
		Cases  []struct {
			Name          string
			Project       json.RawMessage
			Servers       []server
			Errors        []string
			ProjectConfig string
		}
		Updates []struct {
			Label string
			Name  string
			Patch struct {
				Enabled  *bool
				Exposure string
			}
			Override bool
			Error    *string
			Text     *string
		}
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	for _, c := range g.Cases {
		t.Run(c.Name, func(t *testing.T) {
			dir := t.TempDir()
			user := writeJSON(t, filepath.Join(dir, "agent", "mcp.json"), string(g.Global))
			project := writeJSON(t, filepath.Join(dir, "work", ".pi", "mcp.json"), string(c.Project))
			cfg := LoadConfig(user, project, true)
			rel := func(p string) string { return strings.ReplaceAll(p, dir, "<dir>") }
			var got []server
			for _, name := range cfg.Names() {
				s := cfg.Servers[name]
				te := s.ToolExposure
				if len(te) == 0 {
					te = nil
				}
				got = append(got, server{Name: name, Scope: s.Scope, Source: rel(s.Source), Override: rel(s.Override), Exposure: s.Exposure, Enabled: s.Enabled, ToolExposure: te})
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(c.Servers)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("servers:\n got %s\nwant %s", gotJSON, wantJSON)
			}
			if rel(cfg.ProjectConfig) != c.ProjectConfig {
				t.Errorf("project config %q, want %q", rel(cfg.ProjectConfig), c.ProjectConfig)
			}
			var errs []string
			for _, err := range cfg.Errors {
				errs = append(errs, rel(err.Error()))
			}
			sort.Strings(errs)
			if strings.Join(errs, "\n") != strings.Join(c.Errors, "\n") {
				t.Errorf("errors:\n got %s\nwant %s", strings.Join(errs, "\n"), strings.Join(c.Errors, "\n"))
			}
		})
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "mcp.json")
	for i, u := range g.Updates {
		if i == 4 { // the golden rewrites the file before saving onto a definition
			writeJSON(t, file, "{\n\t\"mcpServers\": {\n\t\t\"fs\": {\"enabled\": true},\n\t\t\"def\": {\"command\": \"srv\", \"enabled\": false, \"exposure\": \"direct\"}\n\t}\n}\n")
		}
		err := UpdateServerConfig(file, u.Name, ServerPatch{Enabled: u.Patch.Enabled, Exposure: u.Patch.Exposure}, u.Override)
		switch {
		case u.Error == nil && err != nil:
			t.Fatalf("%s: %v", u.Label, err)
		case u.Error != nil && (err == nil || strings.ReplaceAll(err.Error(), dir, "<dir>") != *u.Error):
			t.Fatalf("%s: error %v, want %q", u.Label, err, *u.Error)
		}
		text, _ := os.ReadFile(file)
		if u.Text != nil && string(text) != *u.Text {
			t.Fatalf("%s:\n%s\nPi:\n%s", u.Label, text, *u.Text)
		}
	}
}
