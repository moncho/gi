# Session import and export

`/export` writes a session as Pi's session format (JSONL, version 3) or as
Pi's HTML page (`internal/sessionexport`). `/import <path.jsonl>` reads a Pi
session file into a new gi session (`internal/sessionimport`), so sessions move
between Pi and gi both ways.

## /export

As in Pi, `/export [path]` writes HTML unless the path ends in `.jsonl`; the
default is `gi-session-<session>.html` in the workspace.

The JSONL is the active session as Pi records one:

- Assistant messages carry what the response was. The turn engine records
  each response's usage and cost, stop reason, thinking blocks (with their
  signatures) and the thinking level the turn asked for in the message
  payload (`usage`, `stop_reason`, `thinking_blocks`, `thinking_level`).
  Thinking blocks come first in the content, as the model produced them.
- A `model_change` entry precedes the first assistant message and each one
  whose model differs; a `thinking_level_change` entry likewise for the
  thinking level.
- gi's `shell` tool is Pi's `bash` (the same `command` argument), in tool
  calls and tool results; `/import` maps it back.

The HTML is Pi's export page: Pi's `template.html`, `template.css` and
`template.js` with the marked and highlight.js builds Pi inlines, copied
unmodified into `internal/sessionexport/template` (MIT; `NOTICE`). The page
carries the header, entries and leaf as base64 session data, and the active
theme as CSS variables: Pi's resolved colours (`getResolvedThemeColors`) in
the theme's order and its export colours, or colours derived from
`userMessageBg` (`deriveExportColors`). Built-in themes come from Pi
(`pi_export_themes_gen.go`); custom and system themes are resolved as Pi
does, with `""` tokens filled from the terminal's colours or Pi's guess for
the theme's appearance. Pi assembles the page with `String.replace`, whose
`$$` and `$&` patterns alter the inlined code (the info panel's cost loses
its `$`); gi reproduces the same page.

Goldens: `scripts/golden-export-html.mjs` runs Pi's `exportFromFile` on
`internal/sessionexport/testdata/pi-session.jsonl` and records the page,
its session data and the theme variables for built-in, custom (every colour
form, fallbacks, export colours) and system themes, and refreshes gi's copy
of the template. `TestRenderHTMLMatchesPi` compares gi's page with Pi's,
`TestExportThemeMatchesPi` the theme colours.

## /import

As in Pi, `/import` asks "Import session / Replace current session with
<path>?" (Pi's confirm dialog), then reports `Session imported from: <path>`.
Pi copies the file into its session directory and resumes it. gi creates a new
session from it and switches to it; the current session stays in `/resume`.
Paths are resolved as Pi's `resolvePath` does: `~` is the home directory and
relative paths are against the workspace. A missing file is Pi's
`File not found: <path>` error.

Reading follows Pi's session manager:

- Blank and malformed lines are skipped. A file without a `session` header is
  rejected.
- Versions 1 and 2 are migrated as Pi does: linear entries get ids, compaction
  `firstKeptEntryIndex` becomes `firstKeptEntryId`, `hookMessage` becomes
  `custom`.
- The active branch runs from the root to the last entry (Pi's leaf),
  following `parentId`. Other branches are not imported.
- The session name is the latest `session_info` entry. The model is the last
  `model_change` or assistant message on the branch. The thinking level is
  the last `thinking_level_change`; without one, gi's default applies, as
  Pi's does.

Entries become gi messages:

| Pi | gi |
| --- | --- |
| user message | user message; images become session media |
| assistant message | assistant message; tool calls in the `tool_calls` payload with full arguments, the text in `display_text`, the model as `provider/model` |
| toolResult message | `tool_result` message |
| bashExecution message | user message with Pi's `bashExecutionToText`; `excludeFromContext` (`!!`) ones are system messages |
| custom message, `custom_message` entry | user message, kind `custom_message` (Pi's model sees it as a user message) |
| `branch_summary` | user message, kind `branch_summary` (`/tree`) |
| `compaction` | compaction message; the latest one is also the context checkpoint |
| `custom` entry `gi.system` | system message (gi's own notices) |
| other `custom` entries | skipped (extension state) |
| `context_edit` | applied to its target: the replacement content, or the target left out |
| `label` | `/tree` label |

The latest compaction on the branch becomes gi's context checkpoint: its
summary replaces the messages before its `firstKeptEntryId`, as in Pi's
`buildContextEntries`. Imported messages record their Pi entry in
`pi_entry_id`.

Goldens: `scripts/golden-session-import.mjs` builds sessions with Pi's own
`SessionManager` (branches, compaction, a branch summary, labels, a context
edit, custom messages, `!` commands, an image, model and thinking changes),
reopens them as Pi's `/import` does and records the active branch, Pi's
context entries, name, model, thinking level and labels.
`TestImportMatchesPiContext` checks that gi's context checkpoint and context
messages are Pi's; `TestExportImportRoundTrip` that export → import → export
is stable (ids and the session's creation time aside).

## Export additions for the round trip

`/export` writes gi's compaction messages as `compaction` entries (the latest
keeps the first message its checkpoint does not cover), branch summaries as
`branch_summary` entries, custom messages as `custom_message` entries and
`!` command messages as `bashExecution` messages.

## Differences from Pi

- Messages recorded before responses kept their records export Pi's zero
  usage, a stop reason from whether they called tools, and no thinking.
- Tool calls recorded before full arguments were stored have only the
  `tool.started` preview: whitespace collapsed and at most 200 characters,
  exported under `command` (shell) or `path` (read, write, edit, ls), or as
  `preview` for other tools.
- The HTML page has no system prompt or tool list, as Pi's
  `exportFromFile`: gi's `/export` does not reach the engine's prompt. Tools
  other than Pi's built-ins render with the template's generic renderer, not
  pre-rendered from gi's TUI.

- gi keeps one copy of a message, so a `context_edit` changes what gi shows,
  not only what the model sees; a removed message is not imported.
- gi's model context replays tool calls as text and leaves tool results out,
  as it does for its own sessions.
- The header `cwd` is not used: gi sessions belong to the workspace gi runs in.
