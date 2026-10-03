# Compaction (#18)

gi compacts as Pi does (`core/compaction/compaction.js`, `utils.js`), in
`internal/compaction`:

- **Cut point** (`FindCutPoint`, Pi's `findCutPoint`): walking back from the
  newest message, Pi's token estimate (characters / 4, counting text,
  thinking, tool-call names and arguments, 4800 characters per image) adds up
  until `keep_recent_tokens`; the cut is the nearest user or assistant message
  at or after that point, never a tool result. A cut at an assistant message
  splits its turn. When the recent budget covers everything, nothing is
  compacted.
- **Summary**: the session model writes it (`Compact`, Pi's `compact`):
  - the older messages are serialized as `[User]:`, `[Assistant]:`,
    `[Assistant thinking]:`, `[Assistant tool calls]: name(k=v, …)` and
    `[Tool result]:` (cut to 2000 characters) inside `<conversation>`;
  - Pi's structured prompt (Goal, Constraints & Preferences, Progress,
    Key Decisions, Next Steps, Critical Context), or, when the context starts
    with an earlier compaction summary, Pi's update prompt with that summary
    in `<previous-summary>`;
  - `/compact <instructions>` adds `Additional focus: <instructions>`;
  - a split turn gets a second, turn-prefix summary, merged under
    `**Turn Context (split turn):**`;
  - files read and modified (`read`, `write`, `edit` calls, plus the lists
    of the previous summary) are appended as `<read-files>` /
    `<modified-files>`;
  - the request uses Pi's system prompt, no tools, the turn's thinking level,
    at most 80% of `reserve_tokens` (50% for the turn prefix, both capped by
    the model's output limit) and no cache writes.
- **Precedence**: a `session_before_compact` hook summary, else the model's,
  else gi's heuristic transcript excerpt. A model summary is refused (and the
  heuristic used) when the response errored, hit the token cap or called a
  tool; Pi fails the compaction there, gi keeps the turn going.
- **Recorded** in the compaction event payload: `summary_source` (`hook`,
  `model`, `heuristic`), `usage` (tokens and cost of the summary requests)
  and `summary_error` when the model's summary was not used.

Golden tests (`TestCompactionMatchesPi`) compare serialization, estimates,
every prompt and token cap, and the merged summary with Pi's own code
(`scripts/golden-compaction.mjs`, a stub model recording the prompts).

Differences from Pi: tool-call arguments are serialized in sorted key order
(Go maps keep none; Pi keeps the model's order); calls made inside codemode
scripts are not counted in the file lists; threshold detection still uses
gi's estimate rather than Pi's last-usage-plus-trailing estimate.
