# messages

Reads chat history. The tool has two contracts, selected by the arguments:

- **Piclaw actions**, used whenever the call has `action` or `query` (or any
  other Piclaw-only field). This ports the read actions `search` and `get` of
  Piclaw's `messages` tool (`runtime/src/extensions/messages-crud.ts`) in its
  single-user mode.
- **Bounded JSON retrieval**, used otherwise: current session only, row IDs or
  numeric windows with cursors. See [message-retrieval.md](../message-retrieval.md).

`content_bytes` and `cursor` belong to the JSON contract and are rejected with
`action`/`query`. Unknown fields and nulls are rejected in both.

## Piclaw `search`

`{action:"search", query}`. `action` defaults to `search`.

- `query` is required: `"*"` lists every post and `"#tag"` matches the tag as a
  substring. Other queries are split into terms matched as case-insensitive
  substrings: a plain query matches any term (Piclaw's default `or` mode); an
  FTS operator query (`AND`/`OR`/`NOT`/`NEAR`, quotes, parentheses, `col:`)
  needs every term, with operator words dropped, as in Piclaw's fallback.
- `limit` 1–50 (default 10), `offset` ≥ 0. Newest row first.
- Filters: `role` (`user`, or `assistant` for every non-user post), `sender`
  (matches the author label), `after`/`since`/`before` (ISO time strings),
  `after_row`/`before_row` (exclusive row IDs).
- `excerpt_chars` 0–1000 replaces each line's text with Piclaw's excerpt:
  centred on the first matching term, terms wrapped in `[[…]]`, `…` at clipped ends.
- `details_max_chars` clips content in the details payload only.

Output: `Found N message(s).` then one `[row] author: text` line per row. Empty
results print `No matching messages found.`; a missing query prints
`Provide query for action=search.`

## Piclaw `get`

`{action:"get", row_ids:[…]}` with up to 50 distinct IDs, `context_before` and
`context_after` 0–20. Each found row prints `- [row] author: text`, followed by
`  before:` / `  after:` blocks of neighbours (same session, row-ID order, same
role/sender filters). Missing IDs are omitted from the text and listed in
`details.missing_row_ids`. Empty content prints `[empty message]`.

## Scope

`chat_jid` omitted means the current session for `search`. For `get`, as in
Piclaw, omitted means any session: row IDs are global. `"*"` or `"all"` reaches
every session; `"gi:<id>"` or `"<id>"` selects one. This is Piclaw's single-user
behaviour; gi has one owner.

## Rows

Both actions read gi's conversation view (`internal/store/conversation.go`), the
equivalent of Piclaw's message table of chat posts: tool results and
tool-call-only assistant rows are excluded and tool-call markers are stripped.
Row IDs are the durable `message_rows` identities.

## Differences from Piclaw

- Only `search` and `get`. `grep`, `extract`, `diff`, `add`, `post`, `delete`
  and `move` fail with an error; `get`'s `content_lines`/`content_grep` are
  not accepted.
- gi has no message FTS index. Piclaw matches FTS5 tokens; gi matches
  substrings, so `log` also finds `logged`. Piclaw evaluates operator
  expressions (`a OR b`, `NOT`, phrases) in FTS5; gi requires every term, which
  is Piclaw's own fallback. Piclaw's `searchMatchMode: "and"` setting has no gi
  equivalent. As in Piclaw's fallback, `%` and `_` in terms act as LIKE wildcards.
- The author is the stored role (`user`, `assistant`, `system`); gi has no
  per-message sender name. `sender` matches that role.
- Character counts (`excerpt_chars`, `details_max_chars`) are in code points;
  Piclaw counts UTF-16 code units.
- No attachment or annotation lines in `get` output.
- Details rows carry `rowid`, `id`, `chat_jid` (`gi:<session>`), `sender`/`role`,
  `content`, `created_at`; there is no `is_bot_message` or `content_blocks`.

## Verification

`scripts/golden-messages-search.mjs` runs Piclaw's `isFtsOperatorQuery`,
`extractFtsFallbackTerms`, `extractSearchTerms` and `buildContentExcerpt` under
bun and writes `internal/tools/testdata/piclaw-messages-search.json`.
`TestPiclawMessagesSearchHelpersMatchGolden` compares gi's ports with it;
`TestPiclawMessagesSearchAndGet` covers output lines, filters, scope and context.
Both run under the race detector in `make test-message-retrieval`.
