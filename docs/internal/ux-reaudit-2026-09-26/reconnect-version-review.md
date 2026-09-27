# Classic SSE reconnect 001–005: two version-drift paths

The frozen `tests/ux/features/classic/compose/sse-reconnection.feature` says a
first changed UI asset version must show a manual-reload warning, **even with
clean editors and composer**; repeated notices for that version are suppressed.
This conflicts with its cited Piclaw source, not just with a Gi test. The
pinned 3.2.4 source manifest marks `runtime/web/src/ui/app-connection-lifecycle.ts`
byte-identical to the copy at `web/src/ui/app-connection-lifecycle.ts`.
`handleUiVersionDriftEvent` records the new version and schedules
`window.location.reload()` after 350 ms when there are no unsaved tabs,
composer draft, active agent or pending request. With any such blocker it
instead shows a `New UI available` toast; a repeated version returns early.
`tests/ux/support/version-drift-lifecycle.test.ts` verifies both branches on
the copied helper (2 passing tests), **not** the installed Piclaw UI runtime.
The clean-state frozen wording and source are therefore inconsistent. Do not
rewrite either contract or label Piclaw's current runtime as observed until a
version-drift browser probe and a policy decision resolve that inconsistency.

Gi's mounted browser entry point is `web/src/app.ts`. Its selected-session SSE
`connected` handler uses `createAssetVersionGuard(loadedAssetVersion(document))`
to set `newUIVersion`; the rendered `gi-version-warning` says `Reload manually`.
It does not call the auto-reload helper. The separate composition chain
`app-main-orchestration-composition.ts` → `app-main-lifecycle-composition.ts` →
`app-agent-status-lifecycle.ts` → `app-sse-events.ts` does pass `serverVersionContext`
to `handleUiVersionDriftEvent`, and that handler returns before reconnection
resync on drift. No call from the mounted `web/src/app.ts` into that
composition chain was found in the Gi entry point; its presence and copied
oracle logic do **not** prove that clean-state auto-reload happens in Gi.

| Frozen ID | Gi assertion / code | Boundary |
|---|---|---|
| `001` | `tests/ux/queue.spec.mjs` severs SSE, checks that transient agent display clears while the user's draft survives; `web/src/app.ts` disconnect handler clears previews, pending/run state. | Focused real SSE-proxy/browser run: 6/6; current Piclaw UI not run. |
| `002` | `tests/ux/reconnect.spec.mjs` holds a stale response across reconnect, checks authoritative timeline, queue and activity; `activationRefresh` and `refreshAfterConnection` reload selected state. | Focused reconnect tags `002`–`005`: 30/30 across six projects; Gi fixture, not deployed acceptance. |
| `003` | Tagged search test checks no general messages refresh over search, but fresh activity/queue/model data and retained draft/file; search-aware refresh in `web/src/app.ts`. | Same focused run; no Piclaw search-reconnect browser probe. |
| `004` | Existing dirty-draft test checks manual warning, no navigation, unchanged asset URL, then manual reload. New **clean-composer** test waits beyond the helper's 350 ms window, checks no navigation on two version notices, and then manually reloads. `createAssetVersionGuard` unit test checks deduplication. | Clean case 6/6; focused tags 30/30; copied source helper test shows opposite clean-state result. Current Piclaw runtime and desired policy unresolved. |
| `005` | Tagged test checks one initial refresh per selected session and a new refresh after actual disconnect; `createActivationRefreshGate` unit test covers readiness ordering. | Same focused run; not an exactly-once transport guarantee. |

A broader unfiltered reconnect command reached 63/72 before its external
260-second timeout; it has **no pass result**. The focused runs above do not
cover all additional Gi reconnect journeys in that command. No production code,
frozen Gherkin, installed Piclaw or live Gi state was changed here. A future
policy decision must state whether frozen no-auto-reload overrides the copied
source's clean-state reload, and a Piclaw 3.2.4 version-change UI probe should
pin actual behaviour before any such change.
