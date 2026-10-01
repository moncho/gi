package mcp

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// OutputMaxBytes is Pi's MCP_OUTPUT_MAX_BYTES: model-facing text beyond it is
// cut in the middle and the full text is saved for reading.
const OutputMaxBytes = 20 * 1024

// ReadResourceTool reads the resources named by resource links.
const ReadResourceTool = "read_mcp_resource"

// SaveFunc stores data (full text or a binary resource) and returns a path the
// model can read. gi saves into the managed VFS rather than a temp file.
type SaveFunc func(data []byte, extension string) (string, error)

// Truncation ports Pi's truncateMiddle result.
type Truncation struct {
	Content      string
	Truncated    bool
	RemovedChars int
	TotalBytes   int
	TotalLines   int
}

// TruncateMiddle keeps the start and end of content within maxBytes, cutting
// on UTF-8 boundaries, with Codex's "…N chars truncated…" marker (Pi).
func TruncateMiddle(content string, maxBytes int) Truncation {
	t := Truncation{Content: content, TotalBytes: len(content), TotalLines: countLines(content)}
	if len(content) <= maxBytes {
		return t
	}
	isBoundary := func(i int) bool { return i >= len(content) || content[i]&0xc0 != 0x80 }
	headEnd := maxBytes / 2
	for headEnd > 0 && !isBoundary(headEnd) {
		headEnd--
	}
	tailStart := len(content) - (maxBytes - maxBytes/2)
	for tailStart < len(content) && !isBoundary(tailStart) {
		tailStart++
	}
	t.RemovedChars = utf8.RuneCountInString(content[headEnd:tailStart])
	t.Content = fmt.Sprintf("%s…%d chars truncated…%s", content[:headEnd], t.RemovedChars, content[tailStart:])
	t.Truncated = true
	return t
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	n := strings.Count(content, "\n") + 1
	if strings.HasSuffix(content, "\n") {
		n--
	}
	return n
}

// FormatSize ports Pi's formatSize.
func FormatSize(bytes int) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%dB", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	}
}

// ConvertOptions control result conversion.
type ConvertOptions struct {
	Save              SaveFunc
	ReadableResources bool // the server's resources can be read with read_mcp_resource
}

// Converted is a model-facing tool result. gi tool results are text, so image
// blocks are described in the text (a placeholder) rather than attached.
type Converted struct {
	Text           string
	IsError        bool
	FullOutputPath string
}

// ConvertResult ports Pi's convertMcpResult: text passes through, embedded
// text resources become text, resource links name read_mcp_resource, binary
// resources are saved for reading, a result without content uses its
// structuredContent as JSON, an isError result without text gets a generic
// message, and text over OutputMaxBytes is cut in the middle with the full
// text saved.
func ConvertResult(server, tool string, result *mcp.CallToolResult, opts ConvertOptions) Converted {
	var parts []string
	if result != nil {
		for _, block := range result.Content {
			parts = append(parts, blockText(server, block, opts))
		}
		if len(result.Content) == 0 && result.StructuredContent != nil {
			if raw, err := json.MarshalIndent(result.StructuredContent, "", "  "); err == nil {
				parts = append(parts, string(raw))
			}
		}
	}
	text := strings.Join(nonEmpty(parts), "\n")
	out := Converted{IsError: result != nil && result.IsError}
	if out.IsError && strings.TrimSpace(text) == "" {
		text = fmt.Sprintf("MCP tool %s/%s returned an error", server, tool)
	}
	trunc := TruncateMiddle(text, OutputMaxBytes)
	if !trunc.Truncated {
		out.Text = text
		return out
	}
	where := "[Could not save the full output: no storage available]"
	if opts.Save != nil {
		if path, err := opts.Save([]byte(text), ".txt"); err == nil {
			out.FullOutputPath = path
			where = fmt.Sprintf("[Full output: %s (read it with offset/limit)]", path)
		} else {
			where = fmt.Sprintf("[Could not save the full output: %v]", err)
		}
	}
	tokens := int(math.Ceil(float64(trunc.TotalBytes) / 4))
	out.Text = fmt.Sprintf("Warning: truncated output (original token count: %d)\nTotal output lines: %d\n\n%s\n\n%s", tokens, trunc.TotalLines, trunc.Content, where)
	return out
}

func nonEmpty(parts []string) []string {
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

var uriExtension = regexp.MustCompile(`\.[A-Za-z0-9]{1,8}$`)

func extensionOf(uri string) string {
	p := uri
	if u, err := url.Parse(uri); err == nil && u.Path != "" {
		p = u.Path
	}
	if ext := uriExtension.FindString(p); ext != "" {
		return ext
	}
	return ".bin"
}

func isTextMimeType(mimeType string) bool {
	t := strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	return t != "" && (strings.HasPrefix(t, "text/") || t == "application/json" || strings.HasSuffix(t, "+json") || strings.HasSuffix(t, "+xml"))
}

func blockText(server string, block mcp.Content, opts ConvertOptions) string {
	switch b := block.(type) {
	case *mcp.TextContent:
		return b.Text
	case *mcp.ImageContent:
		return fmt.Sprintf("[image %s, %s]", b.MIMEType, FormatSize(len(b.Data)))
	case *mcp.AudioContent:
		return fmt.Sprintf("[audio %s omitted]", b.MIMEType)
	case *mcp.ResourceLink:
		var details []string
		if b.MIMEType != "" {
			details = append(details, b.MIMEType)
		}
		if b.Size != nil {
			details = append(details, FormatSize(int(*b.Size)))
		}
		title := b.Name
		if b.Title != "" {
			title = b.Title
		}
		text := fmt.Sprintf("[Resource %s %q", b.URI, title)
		if len(details) > 0 {
			text += " (" + strings.Join(details, ", ") + ")"
		}
		if b.Description != "" {
			text += ": " + b.Description
		}
		if opts.ReadableResources {
			text += fmt.Sprintf(". Read it with %s (server %q)", ReadResourceTool, server)
		}
		return text + "]"
	case *mcp.EmbeddedResource:
		r := b.Resource
		if r == nil {
			return ""
		}
		if r.Blob == nil {
			return r.Text
		}
		if strings.HasPrefix(r.MIMEType, "image/") {
			return fmt.Sprintf("[image %s, %s]", r.MIMEType, FormatSize(len(r.Blob)))
		}
		if isTextMimeType(r.MIMEType) {
			return string(r.Blob)
		}
		kind := r.MIMEType
		if kind == "" {
			kind = "unknown type"
		}
		kind += ", " + FormatSize(len(r.Blob))
		if opts.Save == nil {
			return fmt.Sprintf("[binary resource %s (%s) omitted]", r.URI, kind)
		}
		path, err := opts.Save(r.Blob, extensionOf(r.URI))
		if err != nil {
			return fmt.Sprintf("[Binary resource %s (%s) could not be saved: %v]", r.URI, kind, err)
		}
		return fmt.Sprintf("[Binary resource %s (%s) saved to %s]", r.URI, kind, path)
	default:
		return "[unsupported MCP content]"
	}
}

