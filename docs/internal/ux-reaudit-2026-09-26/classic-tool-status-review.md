# Classic027: tool execution status

Gi's mounted `web/src/app.ts` renders `ToolActivity` only for fresh selected
session activity outside compaction. `web/src/gi-tool-activity.ts` labels tool
name/state/preview and increments elapsed display on a one-second interval
while running; completed and failed entries use terminal timing. The native
activity route supplies a tool-call and turn identity rather than relying on
the tool's display name alone.

`tests/ux/tool-activity.spec.mjs` uses a gated real native turn. It checks the
running preview, advancing elapsed label, reload identity, completed duration,
absence of a spinner, stable terminal duration, a later failed occurrence
with a different tool-call ID, a held stale activity response, and session
switch isolation. Focused `test-ux-steer` run: **6/6** across Chromium/WebKit
phone, tablet and desktop.

`make test-piclaw-output-oracle` also passed **6/6** using the installed
Piclaw 3.2.4 event translator and shipped browser assets alongside Gi's
adapter. Its synthetic tool start/update/end shows call arguments, a bounded
Output preview, and Waiting for model after completion. A separate thought
and draft preview survives that intra-turn transition, with no tool-result
conversation post; an idle event clears all three panes. The fixture does
not test an authoritative reload. A stricter shipped-WebKit lifecycle reload
probe still reports SSE and presence cancellation/access-control errors.

This covers the visible status path and stale-response guard in disposable Gi.
Full tool-pane lifecycle reconstruction, concurrent calls, reduced-motion
presentation, production Piclaw routing and deployed Gi remain unverified.
Classic027 stays unmapped. The separate WIP tool-terminal provenance branch
has no CI/deployment credit from this review. No production code or frozen
contract changed.
