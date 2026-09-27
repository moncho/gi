# Classic024–025: combined copy/delete and scoped message retrieval

Both frozen cases join behaviour that narrower Gi tests keep separate.
`@ux-original-024` requires source-Markdown copy, code-copy, reply-aware
cascade confirmation, cancellation and parent/reply removal.
`@ux-original-025` requires bounded explicit row IDs/windows across permitted
single-user scopes and family-owned authorisation.

| ID | Existing Gi evidence | Missing clause |
|---|---|---|
| `024` | `tests/ux/shared-copy-delete.spec.mjs` checks post Markdown and code text copying, then a single idle direct delete. `message-deletion-review.md` records 017 direct delete at 6/6. | `web/src/app.ts` always calls `deletePost(id,false,originChat)`; `internal/web/message_delete.go` rejects `cascade=true` with HTTP 400. No reply count/prompt/cancel or accepted parent-and-replies removal. The combined frozen case cannot pass from separate copy and direct-delete tests. |
| `025` | The native `messages` tool has durable numeric row IDs, explicit anchors, context and windows, bounded output, missing-row reporting and session isolation. `docs/internal/message-retrieval.md` records the accepted Shared38 current-session web mapping. | The tool accepts no `session_id` or all-chat scope and has no family-owned authorisation mode. Its strict current-session boundary deliberately excludes the wider Classic request. Shared38 cannot count as Classic025. |

These are **policy/capability gaps**, not a failure of the narrower accepted
native journeys. The installed Piclaw 3.2.4 cascade backend and scoped
message-tool runtime were not probed here. The family/all-chat extension and
reply graph need independent identity and safety decisions; no production code
or frozen contract changed.
