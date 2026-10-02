package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

func readFixture(t *testing.T) (ToolRuntime, *[]string) {
	t.Helper()
	ws := t.TempDir()
	var lines, wide strings.Builder
	for i := 1; i <= 3000; i++ {
		fmt.Fprintf(&lines, "line %d\n", i)
	}
	for i := 1; i < 400; i++ {
		fmt.Fprintf(&wide, "w%04d %s\n", i, strings.Repeat("x", 200))
	}
	files := map[string]string{"lines.txt": lines.String(), "wide.txt": wide.String(), "huge.txt": strings.Repeat("y", 60000) + "\nsecond\n"}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := store.Open(filepath.Join(t.TempDir(), "gi.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := s.SaveVFSFile(context.Background(), "mcp-output", "s/out.txt", "text/plain", []byte(files["lines.txt"]), nil); err != nil {
		t.Fatal(err)
	}
	attached := &[]string{}
	return ToolRuntime{Store: s, WorkspaceRoot: ws, AttachImage: func(mime string, data []byte) {
		*attached = append(*attached, fmt.Sprintf("%s:%d", mime, len(data)))
	}}, attached
}

func readWith(t *testing.T, rt ToolRuntime, args map[string]any) (string, error) {
	t.Helper()
	return ExecuteReadTool(context.Background(), rt, goai.ToolCall{Name: "read", Arguments: args})
}

// Golden head, tail and length come from Pi's own read tool on the same files.
func TestReadPagesLikePi(t *testing.T) {
	rt, _ := readFixture(t)
	for _, c := range []struct {
		args       map[string]any
		head, tail string
		length     int
	}{
		{map[string]any{"path": "lines.txt"}, "line 1\nline 2\nline 3\nline 4\nline 5\nline ", "line 2000\n\n[Showing lines 1-2000 of 3001. Use offset=2001 to continue.]", 18954},
		{map[string]any{"path": "lines.txt", "offset": float64(2001)}, "line 2001\nline 2002\nline 2003\nline 2004\n", "line 2999\nline 3000\n", 10000},
		{map[string]any{"path": "lines.txt", "offset": float64(5), "limit": float64(10)}, "line 5\nline 6\nline 7\nline 8\nline 9\nline ", "line 14\n\n[2987 more lines in file. Use offset=15 to continue.]", 129},
		{map[string]any{"path": "wide.txt"}, "w0001 xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "\n\n[Showing lines 1-247 of 400 (50.0KB limit). Use offset=248 to continue.]", 51202},
		{map[string]any{"path": "huge.txt"}, "[Line 1 is 58.6KB, exceeds 50.0KB limit.", "[Line 1 is 58.6KB, exceeds 50.0KB limit. Use bash: sed -n '1p' huge.txt | head -c 51200]", 88},
		// vfs:// paths page the same way.
		{map[string]any{"path": "vfs://mcp-output/s/out.txt", "offset": "5", "limit": 10}, "line 5\nline 6", "[2987 more lines in file. Use offset=15 to continue.]", 129},
	} {
		got, err := readWith(t, rt, c.args)
		if err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		if !strings.HasPrefix(got, c.head) || !strings.HasSuffix(got, c.tail) || len(got) != c.length {
			t.Fatalf("%v:\nlen %d (want %d)\nhead %q\ntail %q", c.args, len(got), c.length, got[:min(40, len(got))], got[max(0, len(got)-140):])
		}
	}
	if _, err := readWith(t, rt, map[string]any{"path": "lines.txt", "offset": float64(5000)}); err == nil || err.Error() != "Offset 5000 is beyond end of file (3001 lines total)" {
		t.Fatalf("beyond end: %v", err)
	}
}

// Images are attached (Pi), from the workspace or the VFS; an oversized
// first line in a VFS file gets a notice without the bash fallback.
func TestReadImagesAndVFSOversizedLine(t *testing.T) {
	rt, attached := readFixture(t)
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	if err := os.WriteFile(filepath.Join(rt.WorkspaceRoot, "pic.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := readWith(t, rt, map[string]any{"path": "pic.png"}); err != nil || got != "Read image file [image/png]" || len(*attached) != 1 || (*attached)[0] != "image/png:16" {
		t.Fatalf("image: %q %v %v", got, err, *attached)
	}
	noImages := rt
	noImages.AttachImage = nil
	if got, _ := readWith(t, noImages, map[string]any{"path": "pic.png"}); !strings.Contains(got, "cannot be attached") {
		t.Fatalf("image without attach support: %q", got)
	}
	if _, err := rt.Store.SaveVFSFile(context.Background(), "mcp-output", "s/huge.txt", "text/plain", []byte(strings.Repeat("z", 60000)), nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := readWith(t, rt, map[string]any{"path": "vfs://mcp-output/s/huge.txt"}); got != "[Line 1 is 58.6KB, exceeds 50.0KB limit and cannot be shown by read.]" {
		t.Fatalf("vfs oversized line: %q", got)
	}
}

func TestReadAbsoluteWorkspacePaths(t *testing.T) {
	rt, _ := readFixture(t)
	relative, err := readWith(t, rt, map[string]any{"path": "lines.txt", "limit": 3})
	if err != nil {
		t.Fatal(err)
	}
	absolute, err := readWith(t, rt, map[string]any{"path": filepath.Join(rt.WorkspaceRoot, "lines.txt"), "limit": 3})
	if err != nil || absolute != relative {
		t.Fatalf("absolute read: %q %v", absolute, err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("outside secret"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readWith(t, rt, map[string]any{"path": outside})
	if err == nil || !strings.Contains(err.Error(), "path escapes workspace") || strings.Contains(got, "outside secret") {
		t.Fatalf("outside read: %q %v", got, err)
	}
}

func TestReadAbsoluteWorkspacePaginationAndMissingFile(t *testing.T) {
	rt, _ := readFixture(t)
	for _, args := range []map[string]any{
		{"offset": float64(2001)},
		{"offset": float64(5), "limit": float64(10)},
	} {
		args["path"] = "lines.txt"
		relative, err := readWith(t, rt, args)
		if err != nil {
			t.Fatal(err)
		}
		args["path"] = filepath.Join(rt.WorkspaceRoot, "lines.txt")
		absolute, err := readWith(t, rt, args)
		if err != nil || absolute != relative {
			t.Fatalf("absolute pagination %v: %q %v", args, absolute, err)
		}
	}
	missing := filepath.Join(rt.WorkspaceRoot, "missing.txt")
	if _, err := readWith(t, rt, map[string]any{"path": missing}); err == nil || !strings.Contains(err.Error(), missing) || strings.Contains(err.Error(), filepath.Join(rt.WorkspaceRoot, missing)) {
		t.Fatalf("missing absolute path should retain original, not double root: %v", err)
	}
}

// details.truncation is Pi's truncateHead result (less content), computed by
// Pi on the same fixtures.
func TestReadTruncationDetailsMatchPi(t *testing.T) {
	rt, _ := readFixture(t)
	for path, want := range map[string]string{
		"lines.txt": `{"firstLineExceedsLimit":false,"lastLinePartial":false,"maxBytes":51200,"maxLines":2000,"outputBytes":18892,"outputLines":2000,"totalBytes":28893,"totalLines":3000,"truncated":true,"truncatedBy":"lines"}`,
		"wide.txt":  `{"firstLineExceedsLimit":false,"lastLinePartial":false,"maxBytes":51200,"maxLines":2000,"outputBytes":51128,"outputLines":247,"totalBytes":82593,"totalLines":399,"truncated":true,"truncatedBy":"bytes"}`,
		"huge.txt":  `{"firstLineExceedsLimit":true,"lastLinePartial":false,"maxBytes":51200,"maxLines":2000,"outputBytes":0,"outputLines":0,"totalBytes":60008,"totalLines":2,"truncated":true,"truncatedBy":"bytes"}`,
	} {
		var details map[string]any
		rt.SetDetails = func(d map[string]any) { details = d }
		if _, err := readWith(t, rt, map[string]any{"path": path}); err != nil {
			t.Fatal(err)
		}
		got, _ := json.Marshal(details["truncation"])
		if string(got) != want {
			t.Fatalf("%s:\n got %s\nwant %s", path, got, want)
		}
	}
	var details map[string]any
	rt.SetDetails = func(d map[string]any) { details = d }
	if _, err := readWith(t, rt, map[string]any{"path": "lines.txt", "offset": float64(5), "limit": float64(10)}); err != nil || details != nil {
		t.Fatalf("untruncated read set details %v (%v)", details, err)
	}
}
