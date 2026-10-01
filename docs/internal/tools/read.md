# Tool: `read`

## Status
Implemented; registered for turns and wired via `/api/tools/execute`. Paging,
limits, notices and image handling follow Pi's `read` tool, and the tests
check them against golden output from Pi's own implementation.

## Purpose
Read a workspace file, a managed VFS asset (`vfs://`) or an FTS result
(`fts://`) in pages, or attach an image file.

## Input
```json
{
  "path": "relative/path.txt",
  "offset": 2001,
  "limit": 200
}
```

### Fields
- `path`: a workspace-relative path, or `vfs://namespace/path`, or `fts://…`.
- `offset` (optional): the 1-indexed line to start from.
- `limit` (optional): the maximum number of lines to return.

Numbers may also arrive as numeric strings.

## Output
Text is split on `\n`. Output is cut at **2000 lines or 50 KB**, whichever
comes first, and only whole lines are returned.

- **Cut by lines:** `…\n\n[Showing lines A-B of N. Use offset=B+1 to continue.]`
- **Cut by bytes:** `…\n\n[Showing lines A-B of N (50.0KB limit). Use offset=B+1 to continue.]`
- **`limit` stopped before the end:** `…\n\n[R more lines in file. Use offset=X to continue.]`
- **An `offset` past the end** is an error:
  `Offset O is beyond end of file (N lines total)`.
- **A first line over 50 KB:**
  - workspace files:
    `[Line A is S, exceeds 50.0KB limit. Use bash: sed -n 'Ap' <path> | head -c 51200]`;
  - `vfs://` files, which shell commands cannot reach:
    `[Line A is S, exceeds 50.0KB limit and cannot be shown by read.]`.
- **Images** (jpeg, png, gif, webp, bmp, detected from content) are attached to
  the tool result as images, as in Pi, with the text `Read image file
  [<mime>]`. Callers without image support, such as the HTTP tool API, get a
  text note instead. gi does not resize images; Pi does.

This lets the model page through large MCP outputs saved to
`vfs://mcp-output/...`, as their `[Full output: … (read it with
offset/limit)]` pointer says.

## Path semantics
- Workspace paths resolve against the configured `workspace_root`.
- `vfs://` paths resolve through the shared resolver: `vfs://skills/...`,
  `vfs://scripts/...`, `vfs://mcp-output/...`, `vfs://reference/...`
  (read-only).

The resolver rejects empty paths and workspace traversal, and enforces VFS
read/write rules.

## Failure modes
- a missing file or path
- path traversal (`path escapes workspace`)
- a missing or invalid `vfs://` path
- an `offset` past the end of the file
