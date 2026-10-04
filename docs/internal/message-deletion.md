# Message deletion

Gi asks before deleting a prompt with visible assistant replies. Confirming removes the prompt and its replies in one transaction. Cancelling sends no delete request.

## Reply identity

Conversation pages and search results expose `reply_to_id`. An assistant message belongs to the first user message in the same session and native `turn_id`. Live assistant posts expose the same identity as `data.thread_id`. Raw storage, exports and model history keep their existing shape.

Steering user messages and system notices stay intact. Messages without a native turn association have no inferred reply parent.

## HTTP contract

`DELETE /api/sessions/{session}/messages/{message}?cascade=true` deletes the target and its assistant replies. Without `cascade=true`, it deletes only the target. The response includes both `ids` and `deleted`, listing the removed message IDs.

The transaction rejects active or queued sessions and protected message roles, preserves audit turns, events and media, and resets the context checkpoint. A failed reply deletion rolls back the parent deletion and checkpoint change. Session scoping prevents a repeated turn ID in another session from joining the cascade.

The browser confirms a visible cascade with `Delete this message and its N replies?`. If a single-message request receives `Replies exist`, it offers `Delete this message and its replies?` before retrying with cascade. Other errors do not trigger a cascade retry. Successful IDs filter late timeline and search responses so deleted rows cannot reappear.

## Verification

- Store tests cover cascade, session isolation, page/search reply identity, reopen, rollback, checkpoint invalidation and busy-session rejection.
- HTTP tests cover the returned IDs, method/auth checks and invalid cascade values.
- Gi browser regressions cover stale timeline/search responses and session switching.
- Fixtures-vibes `f796ddf`: message-deletion scenarios and `@ux-original-024` pass 36/36 across six browser projects.
