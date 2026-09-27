# Classic timeline deletion 017–022: direct deletion versus cascade

`tests/ux/features/classic/timeline/message-deletion.feature` separates a
single post without visible replies from backend-detected or visible reply
cascades. The pinned Piclaw 3.2.4 source map's
`src/ui/app-timeline-actions.ts` counts visible `thread_id` children, prompts
before deleting them, sends `cascade=true`, marks returned IDs removing, and
retries after a `Replies exist` direct-delete rejection with a second prompt.
The current Piclaw backend/UI cascade flow was not exercised in this review;
the source is evidence of intended UI branching, not a completed runtime gate.

| ID | Bounded Gi evidence | Result |
|---|---|---|
| `017` | `tests/ux/message-delete.spec.mjs` holds native DELETE acknowledgment, checks no premature removal, transient removing class, eventual removal, absent stored/search row and reload absence; draft/file survive. `web/src/app.ts:571–600` performs only `deletePost(id,false,originChat)` and animates after success. | Tagged focused run **6/6** Chromium/WebKit phone/tablet/desktop in disposable Gi; no live/Piclaw runtime acceptance. |
| `018` | Gi always sends `cascade=false`. `internal/web/message_delete.go` rejects `cascade=true` with HTTP 400; no reply-detection retry in Gi `handleDeletePost`. | Verified native capability gap: no second confirmation or cascade retry. |
| `019` | No Gi follow-up prompt is offered after `Replies exist`; Gi displays a deletion error and leaves the post on failure. | Cancellation interaction absent, despite the failure-preservation aspect being analogous. No tagged journey. |
| `020` | Gi does not count visible `thread_id` replies to build the three-reply confirmation prompt. | Verified native prompt gap; no tagged journey. |
| `021` | Gi server disallows cascade, UI removes only the acknowledged direct ID. | Verified native cascade gap for parent and replies; no tagged journey. |
| `022` | With no visible-reply prompt there is no cancel path to exercise; ordinary failed deletion preserves state but is not this scenario. | Verified native prompt/cancel gap; no tagged journey. |

Gi's `internal/store/message_delete.go` enforces native session/turn constraints,
which are separate from Piclaw's reply graph. `tests/ux/shared-copy-delete.spec.mjs`
checks busy-turn rejection and later direct deletion, not cascade. Do not count
an API argument named `cascade`, CSS removal animation, or generic error
preservation as evidence for the five unsupported interaction clauses. Any
reply-aware implementation needs an explicit data model and policy decision;
no production code was changed here.
