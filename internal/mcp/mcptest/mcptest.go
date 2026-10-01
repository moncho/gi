// Package mcptest provides an in-process MCP server for tests in other
// packages (served over streamable HTTP with httptest).
package mcptest

import (
	"context"
	"net/http/httptest"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Args is the argument shape of the fake tools.
type Args struct {
	Text string `json:"text,omitempty"`
}

// Server is a running fake MCP server.
type Server struct {
	MCP  *mcp.Server
	HTTP *httptest.Server
	URL  string
}

// New starts a fake server with tools echo, upper, search_code and
// delete_all, one text resource, and the given instructions.
func New(instructions string) *Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "fake", Version: "1"}, &mcp.ServerOptions{Instructions: instructions})
	text := func(v string) *mcp.CallToolResult {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: v}}}
	}
	mcp.AddTool(s, &mcp.Tool{Name: "echo", Description: "Echo text back"}, func(_ context.Context, _ *mcp.CallToolRequest, in Args) (*mcp.CallToolResult, any, error) {
		return text("echo:" + in.Text), nil, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "upper", Description: "Uppercase text", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(_ context.Context, _ *mcp.CallToolRequest, in Args) (*mcp.CallToolResult, any, error) {
		return text(strings.ToUpper(in.Text)), nil, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "search_code", Description: "Search code"}, func(_ context.Context, _ *mcp.CallToolRequest, in Args) (*mcp.CallToolResult, any, error) {
		return text("found:" + in.Text), nil, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "delete_all"}, func(context.Context, *mcp.CallToolRequest, Args) (*mcp.CallToolResult, any, error) {
		r := text("refused")
		r.IsError = true
		return r, nil, nil
	})
	s.AddResource(&mcp.Resource{URI: "mem://notes/readme.txt", Name: "readme", MIMEType: "text/plain"}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: "mem://notes/readme.txt", MIMEType: "text/plain", Text: "hello resource"}}}, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*httpRequest) *mcp.Server { return s }, nil)
	srv := httptest.NewServer(handler)
	return &Server{MCP: s, HTTP: srv, URL: srv.URL}
}

// Close stops the HTTP server.
func (s *Server) Close() { s.HTTP.Close() }

// AddTool adds a tool returning fixed text; the server announces the change.
func AddTool(s *Server, name, reply string) {
	mcp.AddTool(s.MCP, &mcp.Tool{Name: name, Description: "added at runtime"}, func(context.Context, *mcp.CallToolRequest, Args) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: reply}}}, nil, nil
	})
}
