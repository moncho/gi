# Classic settings layering 001–004: bounded overlay trace

`tests/ux/features/classic/settings/settings-layering.feature` scopes
backdrop, dialog and workspace interaction to the Classic shell, not other
skins. The pinned 3.2.4 source manifest marks the Settings dialog modules
changed in Gi. The current Piclaw UI overlay was not browser-probed here.

Gi's `web/src/gi-settings.ts` renders a `BodyPortal` and modal backdrop,
inerts the app while open, traps focus and restores prior state on cleanup.
`internal/web/static/css/gi-settings.css` fixes the portal/backdrop across the
viewport, stacks them at 12000 and uses `rgba(0, 0, 0, 0.5)` for the backdrop.
The four tagged cases in `tests/ux/settings-shell.spec.mjs` each use a real
workspace file and trusted pointer interaction. They check coverage geometry,
computed opacity, dialog center hit-testing and viewport bounds, no workspace
click or file read through the overlay, then restored workspace interaction
after dismissal. Focused run: **24/24** across six Chromium/WebKit viewport
projects.

This is Gi-native overlay evidence, not a direct 3.2.4 Piclaw UI comparison,
physical-device acceptance, or a Visual-skin stacking claim. No frozen
Gherkin or production code changed.
