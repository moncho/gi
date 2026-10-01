package mcp

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Golden values produced by Pi's own createMcpToolName and truncateMiddle.
func TestToolNamesAndTruncationMatchPi(t *testing.T) {
	taken := func(string) bool { return true }
	for _, c := range []struct{ server, tool, plain, hashed string }{
		{"github", "search_code", "mcp__github__search_code", "mcp__github__search_code_80553286"},
		{"dev-radius", "get.page", "mcp__dev_radius__get_page", "mcp__dev_radius__get_page_48233c38"},
		{"s", strings.Repeat("a", 80), "mcp__s__aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_2b133101", "mcp__s__aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_2b133101"},
		{"my-server", "tool name/with:chars", "mcp__my_server__tool_name_with_chars", "mcp__my_server__tool_name_with_chars_c402cab2"},
	} {
		if got := ToolName(c.server, c.tool, nil); got != c.plain {
			t.Fatalf("%s/%s plain = %s, want %s", c.server, c.tool, got, c.plain)
		}
		if got := ToolName(c.server, c.tool, taken); got != c.hashed {
			t.Fatalf("%s/%s hashed = %s, want %s", c.server, c.tool, got, c.hashed)
		}
	}
	tr := TruncateMiddle(strings.Repeat("héllo wörld ", 10), 37)
	if tr.Content != "héllo wörld hél…89 chars truncated…rld héllo wörld " || tr.RemovedChars != 89 || tr.TotalBytes != 140 || tr.TotalLines != 1 {
		t.Fatalf("truncateMiddle %+v", tr)
	}
	if Namespace("dev-radius") != "mcp__dev_radius" {
		t.Fatal("namespace")
	}
}

// Tools whose names sanitize to one name all get the hash suffix, regardless
// of order; names owned by other servers are avoided.
func TestServerToolNamesCollisions(t *testing.T) {
	names := ServerToolNames("srv", []string{"a-b", "a_b", "plain"}, nil)
	if names["plain"] != "mcp__srv__plain" || names["a-b"] == names["a_b"] || !strings.HasPrefix(names["a-b"], "mcp__srv__a_b_") || !strings.HasPrefix(names["a_b"], "mcp__srv__a_b_") {
		t.Fatalf("collision names %v", names)
	}
	other := ServerToolNames("srv", []string{"x"}, func(n string) bool { return n == "mcp__srv__x" })
	if other["x"] == "mcp__srv__x" {
		t.Fatal("taken name reused")
	}
}

func TestConvertResultLikePi(t *testing.T) {
	size := int64(2048)
	r := &mcp.CallToolResult{Content: []mcp.Content{
		&mcp.TextContent{Text: "hello"},
		&mcp.ResourceLink{URI: "file:///a.txt", Name: "a.txt", MIMEType: "text/plain", Size: &size, Description: "notes"},
		&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "mem://x.json", MIMEType: "application/json", Blob: []byte(`{"k":1}`)}},
		&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "mem://bin.dat", MIMEType: "application/octet-stream", Blob: []byte{1, 2, 3}}},
		&mcp.ImageContent{MIMEType: "image/png", Data: make([]byte, 10)},
	}}
	var saved []string
	save := func(data []byte, ext string) (string, error) {
		saved = append(saved, ext)
		return "vfs://mcp-output/s/1" + ext, nil
	}
	got := ConvertResult("docs", "search", r, ConvertOptions{Save: save, ReadableResources: true})
	want := "hello\n[Resource file:///a.txt \"a.txt\" (text/plain, 2.0KB): notes. Read it with read_mcp_resource (server \"docs\")]\n{\"k\":1}\n[Binary resource mem://bin.dat (application/octet-stream, 3B) saved to vfs://mcp-output/s/1.dat]\n[image image/png, 10B]"
	if got.Text != want || got.IsError {
		t.Fatalf("converted:\n%q\nwant\n%q", got.Text, want)
	}
	empty := ConvertResult("docs", "boom", &mcp.CallToolResult{IsError: true}, ConvertOptions{})
	if !empty.IsError || empty.Text != "MCP tool docs/boom returned an error" {
		t.Fatalf("empty error %+v", empty)
	}
	structured := ConvertResult("docs", "s", &mcp.CallToolResult{StructuredContent: map[string]any{"n": 1}}, ConvertOptions{})
	if structured.Text != "{\n  \"n\": 1\n}" {
		t.Fatalf("structured %q", structured.Text)
	}
	big := ConvertResult("docs", "big", &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.Repeat("line\n", 10000)}}}, ConvertOptions{Save: save})
	if big.FullOutputPath == "" || !strings.HasPrefix(big.Text, "Warning: truncated output (original token count: 12500)\nTotal output lines: 10000\n\n") ||
		!strings.Contains(big.Text, "chars truncated…") || !strings.HasSuffix(big.Text, "[Full output: vfs://mcp-output/s/1.txt (read it with offset/limit)]") {
		t.Fatalf("truncated text head %q tail %q", big.Text[:120], big.Text[len(big.Text)-80:])
	}
}

func TestServersSection(t *testing.T) {
	cfg := func(name, exposure, desc string, te map[string]string) ServerConfig {
		return ServerConfig{Name: name, Enabled: true, Exposure: exposure, Description: desc, ToolExposure: te}
	}
	section := RenderServersSection([]SectionServer{
		{Config: cfg("zeta", ExposureDeferred, "", nil), Instructions: "Zeta server.\nMore."},
		{Config: cfg("dev-radius", ExposureCodemode, "Radius tools", nil)},
		{Config: cfg("direct-only", ExposureDirect, "x", nil)},
		{Config: cfg("hidden-but-one", ExposureHidden, "", map[string]string{"get": "codemode"})},
	})
	want := serversSectionIntro + "\n- mcp__dev_radius (codemode): Radius tools\n- mcp__hidden_but_one (codemode)\n- mcp__zeta (tool_search): Zeta server."
	if section != want {
		t.Fatalf("section:\n%s\nwant\n%s", section, want)
	}
	if RenderServersSection([]SectionServer{{Config: cfg("d", ExposureDirect, "", nil)}}) != "" {
		t.Fatal("direct-only servers are not listed")
	}
}

// toolExposure patterns resolve in file order (first match wins).
func TestToolExposurePatternOrder(t *testing.T) {
	path := writeJSON(t, t.TempDir()+"/mcp.json", `{"mcpServers": {"s": {"command": "x", "toolExposure": {"get_*": "hidden", "*": "direct", "get_one": "deferred"}}}}`)
	s := LoadConfig(path, "", false).Servers["s"]
	if s.ToolExposureFor("get_two") != ExposureHidden || s.ToolExposureFor("other") != ExposureDirect || s.ToolExposureFor("get_one") != ExposureDeferred {
		t.Fatalf("pattern order: %s %s %s", s.ToolExposureFor("get_two"), s.ToolExposureFor("other"), s.ToolExposureFor("get_one"))
	}
}
