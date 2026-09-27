# Plan Gherkin: installed Piclaw add-on versus Gi

Oracle environment: Piclaw 3.2.4 with installed
`@rcarmo/piclaw-addon-plan-sidebar` 0.1.25. The add-on is workspace-installed,
not assumed to be part of every portable Piclaw installation. Its source is
`/workspace/.pi/extensions/node_modules/@rcarmo/piclaw-addon-plan-sidebar/`.
The versioned source-reviewed Gherkin is
`tests/ux/features/oracle/piclaw-3.2.4-plan-sidebar.feature`. It has four
parse-checked scenarios, **no browser execution yet**.

The add-on stores per-chat Markdown and `updated_at` (`index.ts`), and the web
editor posts `{chat_jid,markdown}`. It clears dirty state only if the editor still
matches the submitted text (`web/index.ts:470-498`). Same-chat `plan.changes`
refresh preserves dirty text with a warning (`web/index.ts:405-449`). Clicking
Refresh calls `loadPlan()` without dirty preservation (`web/index.ts:573`), with
**no extra discard confirmation**. Submit first saves, then sends a nonempty saved
checklist to the captured chat in auto mode (`web/index.ts:520-556`). The model
`plan` tool reads and writes Markdown with parsed checklist items; this add-on
has no server-side compare-and-swap revision token, although the editor tracks
local `editRevision` to reject stale UI responses.

| Frozen scenarios | Oracle/implementation disposition |
|---|---|
| Classic009–012 | Add-on-dependent clauses broadly correspond to the installed add-on. Gi does not expose a native Plan API/tool/sidebar; they remain unmapped. Do not infer presence from streamed `agentPlan` text in `web/src/app.ts`. |
| Shared18–19 | Requires revision 1→2 and a native Plan tool. The installed add-on uses `updated_at` without server revision CAS; Gi lacks the feature. The shared contract is a strengthening, not current oracle parity. |
| Shared20 | Requires confirmation before discarding dirty Plan text. Installed add-on Refresh does not confirm; it deliberately loads stored Markdown. Gi will retain the frozen no-loss requirement: an explicit Refresh must offer cancellation and must not overwrite dirty text until discard is confirmed. This is a deliberate deviation from the installed add-on and no shared-case pass. |
| Shared21 | Save-before-submit/captured chat are source-backed in the add-on; Gi lacks the Plan workflow. |
| Shared22 | Checklist Markdown/tool semantics exist in the add-on, but Gi has no canonical Plan store/tool/sidebar or revision. No Shared22 credit. |

The Gi target uses monotonic per-session revision CAS, a bounded Plan tool with
runtime-owned scope and a browser editor that keeps local text during remote
updates. For explicit Refresh while dirty, cancel leaves both editor text and
revision unchanged; confirmed discard loads the latest stored Markdown and
revision. Save-before-submit must target the captured session; cross-tab and
reload checks are required. These requirements follow the frozen shared no-loss
contract. The installed add-on's destructive Refresh and timestamp-only storage
remain oracle differences. No Plan production changes were made in this audit.
