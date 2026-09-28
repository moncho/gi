# Pi terminal and Piclaw queue contract

The terminal must clone Pi 0.87.1's presentation and keyboard interaction. Queueing and steering must follow installed Piclaw 3.2.4, with the terminal's input affordances taken from Pi. Gi's existing implementation and fixtures are not independent acceptance criteria.

## References

Installed release: `/opt/piclaw/current`, Piclaw 3.2.4, `@earendil-works/pi-coding-agent` 0.87.1.

- Pi docs: `usage.md`, `how-pi-works.md`, `keybindings.md`, `settings.md`, `sessions.md`, `tui.md`, `terminal-setup.md`, `tmux.md`, `configuration.md`.
- Pi `dist/modes/interactive/interactive-mode.js`: submit dispatch, `handleFollowUp`, `restoreQueuedMessagesToEditor`, `updatePendingMessagesDisplay`, compaction queue, Escape handling.
- Pi `dist/core/agent-session.js`: `_queueSteer`, `_queueFollowUp`, `clearQueue`, `abort`.
- Piclaw `runtime/src/channels/web/agent/agent-control-plane-service.ts`: queue remove, reorder, Steer and storage-failure restoration.
- Piclaw `runtime/src/channels/web/handlers/agent.ts`: compose submission and streaming queue admission.
- Piclaw `runtime/src/channels/web/runtime/queued-followup-lifecycle-service.ts`: deferred/placeholder queue ownership and lifecycle.

`make test-pi-piclaw-queue-oracle` invokes installed production methods with in-memory collaborators. Eight cases record method inputs, outputs and events. This verifies bounded method behaviour, not keyboard dispatch, persistence, actual tool-boundary delivery or complete UI parity.

## Input and delivery distinctions

| Surface/action | Reference behaviour | Gi status |
|---|---|---|
| Pi Enter, idle | Submit normally | Existing; further presentation audit needed |
| Pi Enter, streaming | Queue steering after current assistant turn and tool calls, before next model call | Admission exists; complete boundary ordering needs native-provider proof |
| Pi Alt+Enter, streaming | Separate follow-up, only after pending work finishes | Fixed in this change: was the same callback/intent as Enter |
| Pi Alt+Enter, idle | Ordinary submission | Native PTY verified with explicit modified-key sequence |
| Pi Shift+Enter/Ctrl+J | Newline, no submit | Unit regression retained |
| Pi Alt+Up | Clear all steering/follow-up queues; put steering then follow-up text before current draft | Open: Gi pops one local string and does not remove durable delivery |
| Pi Escape while working | Abort and restore pending messages | Open: verify abort/queue/draft/media transaction and actual key dispatch |
| Pi pending display | Distinct `Steering:` and `Follow-up:` rows and edit-all hint | Open: current Gi transcript/queue presentation differs |
| Piclaw web Return | Return one queued row by replacing editor text/refs, then schedule removal | Replacement now native-tested against installed browser oracle. Gi retains durable attachment recovery and persist-before-delete; edits during async recovery require explicit retry. |
| Piclaw web Steer, active | Remove queued row, persist user message, steer active run | Partial Gi ownership fixture only; full exact contract open |
| Piclaw web Steer, idle or run ending before admission | Dispatch as normal chat processing | Explicit idle send implemented with claim/hold fences; stale-active automatic fallback remains open (Gi conflicts for explicit retry). |
| Piclaw Steer storage failure | Restore removed queued row and report error | Installed method/browser oracle verified; native idle storage failure retains row for retry. Full active-path parity remains open. |
| Piclaw Steer missing row | Idempotent no-removal response | Method oracle verified; native Gi parity open |

The `@gi-ux-004` and `@gi-ux-005` scenarios are retained as migration-gap regressions for existing code. They are no longer acceptable substitutes for clone behaviour. Historical Piclaw snapshots remain unchanged.

## First implementation slice

`multilineInput` now has a distinct follow-up callback for Alt+Enter. `chatTUI` carries delivery intent through both ordinary and durable input paths. `SubmitTUIComposerIntent` accepts only `prompt` or `queue`; prompt/media still come exclusively from the stored claim. Ordinary prompt submission may steer active work; `queue` creates a distinct follow-up without entering the active steering queue.

Tests:

- `make test-tui-queue-input-core`: race×3 for key dispatch, missing-model draft retention, stored-claim delivery, rejection of invalid delivery intent and existing composer receipt guards.
- `make test-tui-queue-input`: real tmux regular/fullscreen, held local shell fixture, Enter steering row, Alt+Enter separate queued turn, session ownership and idle Alt+Enter. No live credentials/providers.
- Retained `/workspace/projects/gi/gi` binary: both native PTYs fail at Alt+Enter follow-up admission; failure captures retained.
- `make check`: Go tests, vet, build, hook checks and 139 functional tests passed; 11 functional tests skipped.
- `make ux-parity-inventory`: 195 support tests passed; no new parity mappings awarded.
- Stored-claim tests additionally check model/media ownership and prevent a settled follow-up from being replayed as steering.

A narrow independent review attempt timed out; no independent approval is recorded.

The PTY fixture verifies admission and eventual completion, not the full model/tool execution ordering contract. Shell-fixture steering can follow a different loop from provider inference. It must not receive parity credit for the latter.

## Remaining acceptance work

- [ ] Pi transcript: roles, backgrounds, spacing, Markdown/ANSI, code/tables/links/images, thinking disclosure, tool call/output/error presentation and cancellation.
- [ ] Pi editor: history boundaries, completion, paste, multiline keys, external editor, keymap configuration, model/thinking pickers, undo/selection and focus ownership.
- [ ] Pi terminal: regular scrollback versus fullscreen viewport, footer, progress, resize, Unicode cell widths, mouse/selection/link precedence and terminal-protocol differences.
- [ ] Queue display and storage: text/media/references, per-session ownership, counts, identifiers, FIFO, one-at-a-time/all modes and reload.
- [ ] Delivery transitions: idle, streaming, active tool, post-tool model, retry/error, abort, compaction, run-end race and session switch.
- [ ] Atomic dequeue/abort restoration so recovered input cannot still execute from its old entry; preserve drafts and attachments on storage errors.
- [ ] Piclaw web return/remove/reorder/Steer behaviour, including missing rows, restore failure, active-to-idle race and duplicate requests.
- [ ] Differential reference/native PTYs and browser journeys, plus independent contract review before mapping IDs as passing.

No production restart, validation-binary replacement or deployment is part of this change.
