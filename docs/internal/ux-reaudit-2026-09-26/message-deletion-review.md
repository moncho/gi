# Classic timeline deletion 017–022: direct deletion versus cascade

`tests/ux/features/classic/timeline/message-deletion.feature` separates a
single post without visible replies from backend-detected or visible reply
cascades. The pinned Piclaw 3.2.4 source map's
`src/ui/app-timeline-actions.ts` counts visible `thread_id` children, prompts
before deleting them, sends `cascade=true`, marks returned IDs removing, and
contains a second prompt/retry branch if direct deletion throws `Replies exist`.
The installed 3.2.4 backend source takes `cascade` from
`src/channels/web/http/dispatch-content.ts`, forwards it through
`src/channels/web/endpoints/channel-endpoint-facade-service.ts`, and in
`src/channels/web/timeline-service.ts:204–225` calls
`deleteMessageByRowId` for a direct deletion or `deleteThreadByRowId` for a
cascade. `src/db/messages.ts:519–567` directly deletes one row without checking
`thread_id` children; the cascade query selects the parent and its direct
children by `chat_jid` and `thread_id`. Neither path emits `Replies exist`.
Thus the frozen `018`/`019` backend-rejection premise is unsupported by the
installed source. With replies outside the loaded view, the source path can
remove only the parent. This is a source-level oracle risk, not a reproduced
3.2.4 runtime deletion; no current Piclaw backend/UI cascade was exercised.

| ID | Bounded Gi evidence | Result |
|---|---|---|
| `017` | `tests/ux/message-delete.spec.mjs` holds native DELETE acknowledgment, checks no premature removal, transient removing class, eventual removal, absent stored/search row and reload absence; draft/file survive. `web/src/app.ts:571–600` performs only `deletePost(id,false,originChat)` and animates after success. | Tagged focused run **6/6** Chromium/WebKit phone/tablet/desktop in disposable Gi; no live/Piclaw runtime acceptance. |
| `018` | Gi always sends `cascade=false`. `internal/web/message_delete.go` rejects `cascade=true` with HTTP 400; no reply-detection retry in Gi `handleDeletePost`. Installed Piclaw backend source also lacks the specified `Replies exist` rejection, although its UI has the matching retry branch. | Verified Gi capability gap; frozen Piclaw premise needs a disposable runtime check. |
| `019` | Gi has no follow-up prompt after `Replies exist`; it displays a deletion error and leaves the post on failure. Installed Piclaw direct-delete source also has no reply guard. | Cancellation interaction absent; generic failure preservation is insufficient. No tagged journey. |
| `020` | Gi does not count visible `thread_id` replies to build the three-reply confirmation prompt. | Verified native prompt gap; no tagged journey. |
| `021` | Gi server disallows cascade, UI removes only the acknowledged direct ID. | Verified native cascade gap for parent and replies; no tagged journey. |
| `022` | With no visible-reply prompt there is no cancel path to exercise; ordinary failed deletion preserves state but is not this scenario. | Verified native prompt/cancel gap; no tagged journey. |

Gi's `internal/store/message_delete.go` enforces native session/turn constraints
and deletes a single flat row. Its `internal/store/schema.go` messages table has
no `thread_id` column; `web/src/components/timeline.ts` can render a
`data.thread_id` supplied by the Classic side, but this does not give Gi a
persisted reply graph. `tests/ux/shared-copy-delete.spec.mjs` checks busy-turn
rejection and later direct deletion, not cascade. The HTTP/store deletion tests
cover direct deletion and cascade rejection; a focused
`go test ./internal/web ./internal/store -run 'Test.*(DeleteMessage|MessageDelete|DeletePost)' -count=1`
passed. No Gi test constructs and deletes a persisted reply graph. An API
argument named `cascade`, CSS removal animation, and generic failure
preservation cannot satisfy `018`–`022`. Reply-aware Gi deletion needs an
explicit identity/data-model and destructive-action policy decision. No
production code, frozen feature, or live chat was changed.
