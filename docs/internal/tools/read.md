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
- `path`: a workspace-relative path, an absolute path resolving inside the workspace, or `vfs://namespace/path`, or `fts://…`.
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

When the output was cut, the result's `details.truncation` is Pi's
`truncateHead` result (`truncated`, `truncatedBy`, `totalLines`,
`totalBytes`, `outputLines`, `outputBytes`, `firstLineExceedsLimit`,
`maxLines`, `maxBytes`), less its `content`, a second copy of the output.

This lets the model page through large MCP outputs saved to
`vfs://mcp-output/...`, as their `[Full output: … (read it with
offset/limit)]` pointer says.

## TUI rendering (Pi's read renderer)
- The call line is `read <path>`, with the home directory shown as `~` and
  the requested range as `:start-end` (or `:start`) in the warning colour.
- While collapsed, some reads get a compact call line ending in
  `(ctrl+o to expand)`: `[skill] <dir>` for a `SKILL.md`, `read resource
  <path>` for `AGENTS.md`/`CLAUDE.md` files, and `read docs <path>` for gi's
  reference tree (`vfs://reference/...`, Pi's own docs in Pi).
- A successful read shows its content only when expanded; an error always
  shows (10 lines while collapsed).
- Content is highlighted by file extension (Pi's extension map); text the
  highlighter does not classify keeps the terminal colour, a known extension
  without a highlighter uses `mdCodeBlock`, other files `toolOutput`. Tabs are
  three spaces, trailing empty lines are dropped, lines word-wrap as in
  pi-tui. gi highlights with chroma where Pi uses highlight.js, so a token may
  be classified differently.
- After the content, `details.truncation` adds Pi's notice:
  `[Truncated: showing N of M lines (L line limit)]`,
  `[Truncated: N lines shown (50.0KB limit)]` or
  `[First line exceeds 50.0KB limit]`.

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

Absolute paths are not prefixed with the workspace root. Absolute paths outside
its resolved filesystem boundary fail with `path escapes workspace`; the tool
never reports an invented doubled path. Relative paths and VFS URLs retain their
behavior. Existing symlinks, including ancestors of missing descendants, are
resolved for confinement; unresolved/dangling symlinks are rejected. These checks
are path validation, not a claim of race-free filesystem access. Native write
retains its stricter rejection of existing symlink components.

## Path regression coverage

`make test-tool-paths` covers absolute/relative paths, sibling-prefix and traversal
rejection, missing descendants, safe and escaping symlinks, and dangling links.
Native read tests compare paginated absolute and relative responses and retain the
original path in missing-file errors. Native write/read round trips verify parent
creation and index invalidation; rejected outside writes create no directories or
index events. The new absolute-path regressions fail against the previous resolver.
