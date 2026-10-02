package mcp_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gimcp "github.com/rcarmo/gi/internal/mcp"
	"github.com/rcarmo/gi/internal/mcp/mcptest"
)

type cliRun struct {
	code     int
	out, err string
}

func runCLI(t *testing.T, opts gimcp.CLIOptions, args ...string) cliRun {
	t.Helper()
	var out, errs []string
	opts.Log = func(s string) { out = append(out, s) }
	opts.Error = func(s string) { errs = append(errs, s) }
	code := gimcp.RunCommand(args, opts)
	return cliRun{code, strings.Join(out, "\n"), strings.Join(errs, "\n")}
}

func cliOptions(t *testing.T) gimcp.CLIOptions {
	dir := t.TempDir()
	return gimcp.CLIOptions{Cwd: dir, UserPath: filepath.Join(dir, "home", "mcp.json"),
		ProjectPath: filepath.Join(dir, ".pi", "mcp.json"), LogPath: filepath.Join(dir, "mcp.log"),
		CredentialsPath: filepath.Join(dir, "home", "mcp-auth.json")}
}

// add writes Pi's entry shape, keeps other content and the file's indent.
func TestCLIAddRemove(t *testing.T) {
	opts := cliOptions(t)
	_ = os.MkdirAll(filepath.Dir(opts.UserPath), 0o755)
	if err := os.WriteFile(opts.UserPath, []byte("{\n\t\"autoEnableCodemode\": false,\n\t\"mcpServers\": {\"old\": {\"command\": \"x\"}}\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := runCLI(t, opts, "add", "fs", "--env", "A=1", "--exposure", "direct", "--", "npx", "-y", "server", "--flag")
	if r.code != 0 || r.out != `Added global MCP server "fs" in `+opts.UserPath+".\nCheck it with: gi mcp list" {
		t.Fatalf("%+v", r)
	}
	want := "{\n\t\"autoEnableCodemode\": false,\n\t\"mcpServers\": {\n\t\t\"old\": {\n\t\t\t\"command\": \"x\"\n\t\t},\n\t\t\"fs\": {\n\t\t\t\"command\": \"npx\",\n\t\t\t\"args\": [\n\t\t\t\t\"-y\",\n\t\t\t\t\"server\",\n\t\t\t\t\"--flag\"\n\t\t\t],\n\t\t\t\"env\": {\n\t\t\t\t\"A\": \"1\"\n\t\t\t},\n\t\t\t\"exposure\": \"direct\"\n\t\t}\n\t}\n}\n"
	if got, _ := os.ReadFile(opts.UserPath); string(got) != want {
		t.Fatalf("file:\n%s", got)
	}
	r = runCLI(t, opts, "add", "docs", "--url", "https://example.com/mcp", "--bearer-token-env-var", "TOKEN")
	if r.code != 0 || !strings.HasSuffix(r.out, "Check it with: gi mcp list") {
		t.Fatalf("%+v", r)
	}
	r = runCLI(t, opts, "add", "sentry", "--url", "https://example.com/mcp")
	if !strings.HasSuffix(r.out, "If it requires sign-in: gi mcp login sentry") {
		t.Fatalf("%+v", r)
	}
	if r = runCLI(t, opts, "add", "sentry", "--url", "https://example.com/v2"); !strings.HasPrefix(r.out, `Replaced global MCP server "sentry"`) {
		t.Fatalf("%+v", r)
	}
	if r = runCLI(t, opts, "remove", "sentry"); r.code != 0 || !strings.HasPrefix(r.out, `Removed global MCP server "sentry"`) {
		t.Fatalf("%+v", r)
	}
	if r = runCLI(t, opts, "remove", "fs", "-l"); r.code != 1 || !strings.Contains(r.err, `No project MCP server named "fs" in `+opts.ProjectPath+". It is defined in "+opts.UserPath+"; omit --local.") {
		t.Fatalf("%+v", r)
	}
	for args, want := range map[string]string{
		"add x --cwd /tmp --url https://a": "--cwd only applies to stdio servers.",
		"add x --header A=1 -- cmd":        "--header only applies to HTTP servers (--url).",
		"add x --bogus 1 -- cmd":           "Unknown option --bogus.",
		"add x":                            "Usage: gi mcp add <server>",
		"add x --env NOPE -- cmd":          `--env expects KEY=VALUE, got "NOPE".`,
		"add x --url ftp://a":              "url must be an http or https URL",
		"frob":                             `Unknown mcp command "frob".`,
	} {
		if r := runCLI(t, opts, strings.Fields(args)...); r.code != 1 || !strings.Contains(r.err, want) {
			t.Fatalf("%s: %+v (want %q)", args, r, want)
		}
	}
}

// list connects to every enabled server and fails when one does not connect.
func TestCLIList(t *testing.T) {
	fake := mcptest.New("")
	t.Cleanup(fake.Close)
	opts := cliOptions(t)
	if r := runCLI(t, opts, "add", "fake", "--url", fake.URL, "--exposure", "direct"); r.code != 0 {
		t.Fatalf("%+v", r)
	}
	r := runCLI(t, opts, "list")
	if r.code != 0 || !strings.Contains(r.out, "fake: connected, 4 tools (direct, global)\n  "+fake.URL+"\n  tools: ") || !strings.Contains(r.out, "resources: 1, URI templates: 0") {
		t.Fatalf("%+v", r)
	}
	if r := runCLI(t, opts, "add", "broken", "--", "/nonexistent/server"); r.code != 0 {
		t.Fatalf("%+v", r)
	}
	r = runCLI(t, opts, "list", "--json")
	if r.code != 1 || !strings.Contains(r.out, `"state": "failed"`) {
		t.Fatalf("%+v", r)
	}
	if r := runCLI(t, opts, "login", "fake"); r.code != 0 || r.out != `Already signed in to MCP server "fake" (4 tools).` {
		t.Fatalf("%+v", r)
	}
}
