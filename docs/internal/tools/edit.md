# Tool: `edit`

## Status
Implemented; a port of Pi's `edit` tool (`core/tools/edit.js`,
`edit-diff.js`). Tests check the matching rules, error messages and display
diff against golden output from Pi's own code and jsdiff 8.0.4
(`scripts/golden-edit-diff.mjs`).

## Purpose
Change part of an existing text file with exact-text replacements. Prefer it
to `write` for localized changes.

## Input
```json
{
  "path": "relative/path.go",
  "edits": [
    { "oldText": "return 1", "newText": "return 2" }
  ]
}
```

- `path`: as for `write` (workspace path, absolute path inside the
  workspace, or a writable `vfs://` path).
- `edits`: one or more `{oldText, newText}` replacements.

Models send other shapes; like Pi, gi accepts `edits` as a JSON string or a
single object, and the legacy top-level `oldText`/`newText`.

## Behavior
- A leading BOM is set aside before matching and restored on write; the
  file's line endings (LF or CRLF) are kept. Matching runs on LF text.
- Every `oldText` is matched against the **original** content, not after
  earlier edits. Each must occur exactly once and edits must not overlap.
- An exact match is tried first. Otherwise, a fuzzy match ignores trailing
  whitespace, NFKC differences, smart quotes, Unicode dashes and special
  spaces; only the lines a fuzzy edit touches are rewritten, the rest keep
  their bytes.
- Writes go through `write`'s path (`WriteFile`): the same confinement,
  symlink rules and index invalidation.
- Edits to one path run one at a time (Pi's file mutation queue).

Result text: `Successfully replaced N block(s) in <path>.`, with `details`:

- `diff`: Pi's display diff, lines `+N text`, `-N text` and ` N text` with 4
  context lines and ` ...` for skipped runs;
- `firstChangedLine`: the first changed line in the new file.

Pi's `details.patch` (a unified patch for SDK consumers) is omitted; no
renderer reads it.

## Errors (Pi's wording)
- `Could not edit file: <path>. Error code: ENOENT.` (or `EACCES`)
- `Could not find the exact text in <path>. The old text must match exactly including all whitespace and newlines.`
  (`Could not find edits[i] in <path>. …` with several edits)
- `Found N occurrences of the text in <path>. The text must be unique. Please provide more context to make it unique.`
- `oldText must not be empty in <path>.`
- `edits[i] and edits[j] overlap in <path>. Merge them into one edit or target disjoint regions.`
- `No changes made to <path>. The replacement produced identical content. …`
- `Edit tool input is invalid. edits must contain at least one replacement.`

## TUI rendering (Pi's edit renderer)
The call is a box (`edit <path>`) whose background comes from a preview,
computed when the call starts by applying the edits to the current file
without writing:

- a diff preview: success background, the whole diff below a blank line
  (never truncated or collapsible);
- a preview error: error background, the error in the box;
- no preview: pending, or error once the call fails.

The result's diff replaces the preview. A result error the preview did not
already show is printed below the box, outside its background. Diff lines are
Pi's `renderDiff`: removed lines in `toolDiffRemoved`, added in
`toolDiffAdded`, context in `toolDiffContext`, and for a single changed line,
the changed words (jsdiff `diffWords`) in inverse video. Resumed sessions show
the stored `details.diff`; a failed edit shows its error as the preview.
