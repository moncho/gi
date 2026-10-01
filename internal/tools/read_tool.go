package tools

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

// Pi's read tool limits (core/tools/truncate.js).
const (
	ReadMaxLines = 2000
	ReadMaxBytes = 50 * 1024
)

// ReadToolDescription is Pi's read tool description.
var ReadToolDescription = fmt.Sprintf("Read the contents of a file. Supports text files and images (jpg, png, gif, webp, bmp). Images are sent as attachments. For text files, output is truncated to %d lines or %dKB (whichever is hit first). Use offset/limit for large files. When you need the full file, continue with offset until complete. Accepts workspace-relative paths and vfs://namespace/path.", ReadMaxLines, ReadMaxBytes/1024)

// ReadToolParameters is Pi's read schema (path, offset, limit).
const ReadToolParameters = `{"type":"object","properties":{"path":{"type":"string","description":"Path to the file to read (workspace-relative or vfs://namespace/path)"},"offset":{"type":"number","description":"Line number to start reading from (1-indexed)"},"limit":{"type":"number","description":"Maximum number of lines to read"}},"required":["path"]}`

// supportedImageTypes are the image formats read attaches (Pi).
var supportedImageTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true, "image/bmp": true}

func detectImageMIME(data []byte) string {
	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	if t := http.DetectContentType(sniff); supportedImageTypes[t] {
		return t
	}
	return ""
}

// numberArg reads an optional positive integer argument (JSON numbers arrive
// as float64; numeric strings are accepted too).
func numberArg(args map[string]any, key string) (int, bool, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, false, nil
	}
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case int:
		n = float64(x)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0, false, fmt.Errorf("read: %s must be a number", key)
		}
		n = f
	default:
		return 0, false, fmt.Errorf("read: %s must be a number", key)
	}
	return int(n), true, nil
}

// ExecuteReadTool is the read tool: Pi's paging (offset/limit, 2000 lines or
// 50 KB with continuation notices) for workspace files, vfs:// files and fts://
// results, and image files attached as images when the runtime allows.
func ExecuteReadTool(ctx context.Context, rt ToolRuntime, call goai.ToolCall) (string, error) {
	path, _ := call.Arguments["path"].(string)
	if path == "" {
		return "", fmt.Errorf("read: path is required")
	}
	offset, hasOffset, err := numberArg(call.Arguments, "offset")
	if err != nil {
		return "", err
	}
	limit, hasLimit, err := numberArg(call.Arguments, "limit")
	if err != nil {
		return "", err
	}
	resolved, err := ResolveToolPath(rt.WorkspaceRoot, path, false)
	if err != nil {
		return "", err
	}
	var raw []byte
	isVFS := resolved.IsVFS()
	switch {
	case isVFS && resolved.VFSNamespace == "fts":
		text, err := ReadFTSQuery(ctx, rt.WorkspaceRoot, rt.Store, resolved.VFSPath)
		if err != nil {
			return "", err
		}
		raw = []byte(text)
	case isVFS:
		_, raw, err = rt.Store.GetVFSFileContent(ctx, resolved.VFSNamespace, resolved.VFSPath)
		if err != nil {
			return "", err
		}
	default:
		raw, err = os.ReadFile(resolved.WorkspacePath)
		if err != nil {
			return "", err
		}
	}
	if mime := detectImageMIME(raw); mime != "" {
		if rt.AttachImage == nil {
			return fmt.Sprintf("Read image file [%s] (%s; images cannot be attached here)", mime, formatReadSize(len(raw))), nil
		}
		rt.AttachImage(mime, raw)
		return fmt.Sprintf("Read image file [%s]", mime), nil
	}
	return pageText(string(raw), path, isVFS, offset, hasOffset, limit, hasLimit)
}

// ExecuteRead is the read tool without an image-capable runtime (kept for
// callers such as the HTTP tool API).
func ExecuteRead(ctx context.Context, workspaceRoot string, s *store.Store, call goai.ToolCall) (string, error) {
	return ExecuteReadTool(ctx, ToolRuntime{Store: s, WorkspaceRoot: workspaceRoot}, call)
}

// pageText ports Pi's read text handling.
func pageText(text, path string, isVFS bool, offset int, hasOffset bool, limit int, hasLimit bool) (string, error) {
	allLines := strings.Split(text, "\n")
	startLine := 0
	if hasOffset && offset > 0 {
		startLine = offset - 1
	}
	if startLine >= len(allLines) {
		return "", fmt.Errorf("Offset %d is beyond end of file (%d lines total)", offset, len(allLines))
	}
	startDisplay := startLine + 1
	var selected string
	userLimited := -1
	if hasLimit {
		end := min(startLine+max(limit, 0), len(allLines))
		selected = strings.Join(allLines[startLine:end], "\n")
		userLimited = end - startLine
	} else {
		selected = strings.Join(allLines[startLine:], "\n")
	}
	t := truncateHead(selected, ReadMaxLines, ReadMaxBytes)
	switch {
	case t.firstLineExceedsLimit:
		size := formatReadSize(len(allLines[startLine]))
		if isVFS {
			return fmt.Sprintf("[Line %d is %s, exceeds %s limit and cannot be shown by read.]", startDisplay, size, formatReadSize(ReadMaxBytes)), nil
		}
		return fmt.Sprintf("[Line %d is %s, exceeds %s limit. Use bash: sed -n '%dp' %s | head -c %d]", startDisplay, size, formatReadSize(ReadMaxBytes), startDisplay, path, ReadMaxBytes), nil
	case t.truncated:
		end := startDisplay + t.outputLines - 1
		if t.byLines {
			return fmt.Sprintf("%s\n\n[Showing lines %d-%d of %d. Use offset=%d to continue.]", t.content, startDisplay, end, len(allLines), end+1), nil
		}
		return fmt.Sprintf("%s\n\n[Showing lines %d-%d of %d (%s limit). Use offset=%d to continue.]", t.content, startDisplay, end, len(allLines), formatReadSize(ReadMaxBytes), end+1), nil
	case userLimited >= 0 && startLine+userLimited < len(allLines):
		remaining := len(allLines) - (startLine + userLimited)
		return fmt.Sprintf("%s\n\n[%d more lines in file. Use offset=%d to continue.]", t.content, remaining, startLine+userLimited+1), nil
	default:
		return t.content, nil
	}
}

type headTruncation struct {
	content               string
	truncated, byLines    bool
	outputLines           int
	firstLineExceedsLimit bool
}

// truncateHead ports Pi's truncateHead: keep whole leading lines within the
// line and byte limits.
func truncateHead(content string, maxLines, maxBytes int) headTruncation {
	lines := strings.Split(content, "\n")
	if content == "" {
		lines = nil
	} else if strings.HasSuffix(content, "\n") {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= maxLines && len(content) <= maxBytes {
		return headTruncation{content: content, outputLines: len(lines)}
	}
	if len(lines) > 0 && len(lines[0]) > maxBytes {
		return headTruncation{truncated: true, firstLineExceedsLimit: true}
	}
	var out []string
	used := 0
	byLines := true
	for i := 0; i < len(lines) && i < maxLines; i++ {
		n := len(lines[i])
		if i > 0 {
			n++
		}
		if used+n > maxBytes {
			byLines = false
			break
		}
		out = append(out, lines[i])
		used += n
	}
	if len(out) >= maxLines && used <= maxBytes {
		byLines = true
	}
	return headTruncation{content: strings.Join(out, "\n"), truncated: true, byLines: byLines, outputLines: len(out)}
}

// formatReadSize ports Pi's formatSize.
func formatReadSize(bytes int) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%dB", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	}
}
