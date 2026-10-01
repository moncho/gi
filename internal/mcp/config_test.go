package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeJSON(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Pi's mcp.json rules: transports, defaults, validation, skip-not-fail.
func TestLoadConfigFollowsPiRules(t *testing.T) {
	dir := t.TempDir()
	user := writeJSON(t, filepath.Join(dir, "user.json"), `{
	  "autoEnableCodemode": false,
	  "mcpServers": {
	    "fs": {"command": "npx", "args": ["-y", "server-fs", "."], "env": {"TOKEN": "${GI_TEST_TOKEN}"}, "cwd": "sub"},
	    "docs": {"url": "https://example.com/mcp", "headers": {"Authorization": "Bearer ${X}"}, "timeout": 5, "exposure": "codemode-deferred",
	             "toolExposure": {"search": "direct", "get_*": "hidden"}, "description": "Product docs"},
	    "off": {"command": "srv", "enabled": false},
	    "legacy": {"type": "sse", "url": "https://example.com/sse"},
	    "both": {"command": "x", "url": "https://x"},
	    "bad name!": {"command": "x"},
	    "badexp": {"command": "x", "exposure": "public"},
	    "my-tool": {"command": "a"},
	    "my_tool": {"command": "b"}
	  }}`)
	cfg := LoadConfig(user, "", false)
	if cfg.AutoEnableCodemode {
		t.Fatal("autoEnableCodemode false not honoured")
	}
	fs := cfg.Servers["fs"]
	if fs.Transport != "stdio" || fs.Timeout != DefaultTimeout || !fs.Enabled || fs.Exposure != ExposureCodemode || fs.Cwd != "sub" {
		t.Fatalf("stdio defaults: %+v", fs)
	}
	docs := cfg.Servers["docs"]
	if docs.Transport != "http" || docs.Timeout != 5*time.Second || docs.Exposure != ExposureCodemode || docs.Description != "Product docs" {
		t.Fatalf("http entry: %+v", docs)
	}
	if docs.ToolExposureFor("search") != ExposureDirect || docs.ToolExposureFor("get_page") != ExposureHidden || docs.ToolExposureFor("other") != ExposureCodemode {
		t.Fatal("toolExposure resolution")
	}
	if cfg.Servers["off"].Enabled {
		t.Fatal("enabled:false ignored")
	}
	for _, rejected := range []string{"legacy", "both", "bad name!", "badexp"} {
		if _, ok := cfg.Servers[rejected]; ok {
			t.Fatalf("%q accepted", rejected)
		}
	}
	_, a := cfg.Servers["my-tool"]
	_, b := cfg.Servers["my_tool"]
	if a == b {
		t.Fatal("names differing only in - and _ must keep exactly one")
	}
	joined := ""
	for _, err := range cfg.Errors {
		joined += err.Error() + "\n"
	}
	for _, want := range []string{"SSE transport is not supported", "either command or url", "letters, digits", "exposure must be", "duplicates"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing error %q in:\n%s", want, joined)
		}
	}
}

// A trusted project entry replaces a user entry with the same name; an
// untrusted project file is ignored entirely.
func TestLoadConfigProjectOverrideNeedsTrust(t *testing.T) {
	dir := t.TempDir()
	user := writeJSON(t, filepath.Join(dir, "user.json"), `{"mcpServers": {"fs": {"command": "user-fs"}, "keep": {"command": "k"}}}`)
	project := writeJSON(t, filepath.Join(dir, "project.json"), `{"autoEnableCodemode": false, "mcpServers": {"fs": {"command": "project-fs"}}}`)
	untrusted := LoadConfig(user, project, false)
	if untrusted.Servers["fs"].Command != "user-fs" || !untrusted.AutoEnableCodemode {
		t.Fatal("untrusted project config was read")
	}
	trusted := LoadConfig(user, project, true)
	if trusted.Servers["fs"].Command != "project-fs" || trusted.Servers["keep"].Command != "k" || trusted.AutoEnableCodemode {
		t.Fatalf("project override: %+v", trusted.Servers)
	}
}

func TestExpandValue(t *testing.T) {
	lookup := func(k string) (string, bool) { v, ok := map[string]string{"A": "x", "B": "y"}[k]; return v, ok }
	if v, _ := expandValue(context.Background(), "Bearer ${A}-${B}-${MISSING}", lookup); v != "Bearer x-y-" {
		t.Fatalf("env expansion %q", v)
	}
	if v, err := expandValue(context.Background(), "!printf '  tok  '", lookup); err != nil || v != "tok" {
		t.Fatalf("command expansion %q %v", v, err)
	}
	if _, err := expandValue(context.Background(), "!exit 3", lookup); err == nil {
		t.Fatal("failing command accepted")
	}
}
