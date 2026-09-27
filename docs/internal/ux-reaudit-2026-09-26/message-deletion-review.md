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
An isolated installed-backend probe (`PICLAW_DB_IN_MEMORY=1 bun
tests/ux/oracle/piclaw-deletion-backend-probe.ts`) pins the installed version
and Classic source-map hash, uses an in-memory SQLite database, and checks both
paths. Direct deletion of a parent with an unseen persisted reply returned 200
with only the parent ID, leaving the reply row with its original `thread_id`.
Cascade deletion returned 200 with the parent and three direct reply IDs and
removed those four rows. This reproduces the `018`/`019` backend-rejection
premise mismatch **at the backend-function boundary**. It does not run the
HTTP route or prove a user-visible deletion in Piclaw.

`tests/ux/oracle/piclaw-deletion-ui-probe.mjs` pins the shipped Classic assets
and runs separate disposable `/timeline` and `/post/:id` fixtures in Chromium
and WebKit desktop. With three visible replies, cancellation sends no DELETE
and confirmation sends `cascade=true` and removes the returned IDs. When the
fixture deliberately returns HTTP 409 `Replies exist` for a hidden reply, the
UI prompts after `cascade=false`: cancellation leaves the parent visible and
confirmation retries with `cascade=true`. These fixture assertions exercise
the shipped UI branch, not the installed backend. The backend probe's real
response is 200 instead of the synthetic 409, so the two independent probes
do not establish `018`/`019` end-to-end.

| ID | Bounded Gi evidence | Result |
|---|---|---|
| `017` | `tests/ux/message-delete.spec.mjs` holds native DELETE acknowledgment, checks no premature removal, transient removing class, eventual removal, absent stored/search row and reload absence; draft/file survive. `web/src/app.ts:571–600` performs only `deletePost(id,false,originChat)` and animates after success. | Tagged focused run **6/6** Chromium/WebKit phone/tablet/desktop in disposable Gi; no live/Piclaw runtime acceptance. |
| `018` | Gi always sends `cascade=false`. `internal/web/message_delete.go` rejects `cascade=true` with HTTP 400; no reply-detection retry in Gi `handleDeletePost`. Installed Piclaw backend-function deletion of a parent with an unseen reply returns 200 and orphans it; the independent shipped-UI fixture retries only after a synthetic 409 `Replies exist`. | Verified Gi gap; frozen Piclaw premise fails in an isolated backend-function probe, while the shipped UI conditional branch passes a fixture probe. Integrated HTTP/UI untested. |
| `019` | Gi has no follow-up prompt after `Replies exist`; it displays a deletion error and leaves the post on failure. Installed Piclaw backend-function deletion returns 200 for this setup; the shipped UI's cancellation preserves the fixture parent after synthetic 409. | Gi cancellation interaction absent; generic failure preservation is insufficient. No integrated HTTP/UI probe or Gi tagged journey. |
| `020` | Gi does not count visible `thread_id` replies to build the three-reply confirmation prompt. Shipped Piclaw UI with three fixture replies shows the exact `Delete this message and its 3 replies?` dialog in Chromium and WebKit desktop. | Verified Gi prompt gap; Piclaw fixture branch is bounded, without installed-backend or physical acceptance. |
| `021` | Gi server disallows cascade, UI removes only the acknowledged direct ID. Shipped Piclaw UI with fixture replies sends `cascade=true` and removes all returned IDs; the independent installed-backend probe deletes a parent and three replies with `cascade=true`. | Verified Gi cascade gap; independent probes do not establish an integrated HTTP/UI journey. |
| `022` | Gi has no visible-reply prompt or its cancel path; generic failure preservation is insufficient. Shipped Piclaw UI with three fixture replies leaves parent and replies visible and sends no DELETE when the prompt is cancelled. | Verified Gi prompt/cancel gap; Piclaw fixture assertion is bounded, without integrated backend acceptance. |

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
