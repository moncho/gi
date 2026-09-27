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

This covers the visible status path and stale-response guard in disposable Gi.
It does not validate full tool-pane lifecycle reconstruction, reduced-motion
presentation, current Piclaw 3.2.4 browser UI, or deployed Gi. The separate
WIP tool-terminal provenance branch has no CI/deployment credit from this
review. No production code or frozen contract changed.
