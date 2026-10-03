# Session import and export

`/export` writes a session as Pi's session format (JSONL, version 3) or an
HTML transcript (`internal/sessionexport`). `/import <path.jsonl>` reads a Pi
session file into a new gi session (`internal/sessionimport`), so sessions move
between Pi and gi both ways.

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

- gi keeps one copy of a message, so a `context_edit` changes what gi shows,
  not only what the model sees; a removed message is not imported.
- gi's model context replays tool calls as text and leaves tool results out,
  as it does for its own sessions.
- The header `cwd` is not used: gi sessions belong to the workspace gi runs in.
