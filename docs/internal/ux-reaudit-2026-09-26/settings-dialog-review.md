# Classic settings dialog 001–005: bounded native shell

`tests/ux/features/classic/settings/settings-dialog.feature` is a frozen
Piclaw Classic contract. Gi mounts `web/src/gi-settings.ts` with a modal
`BodyPortal`, a General snapshot cache, a cold loading shell, and pane modules
loaded on visit by `gi-settings-lazy.ts`. The installed 3.2.4 Settings UI was
not exercised here. These tests assert Gi's own shell, not parity for every
setting or subsection.

| ID | Tagged native assertion | Boundary |
|---|---|---|
| `001` | Three rapid Control-comma presses leave one Gi Settings dialog and portal; Escape restores composer focus. `GiSettings` opening uses an `isOpen` ref. | Actual shortcut differs by platform; focused emulated browser journey. |
| `002` | Open via hamburger, visit panes, close, reopen while runtime-config GET is held; General shows cached snapshot in under one second, one portal, no draft/media loss or writes. | Timing measured under disposable Gi fixture, not a global latency SLO. |
| `003` | Held first runtime-config read shows immediate `Loading settings…`, General selected first, values resolve within two seconds without blank shell. | Same fixture timing boundary. |
| `004` | Compaction numeric spinbutton accepts typed `128000` without saving/mutating settings. | Typability only, not valid policy/application of that value. |
| `005` | General shows without other pane chunks; Models and subsequent built-ins load on click and cached revisits reuse modules. | Test is stricter about Gi's five pane chunks, but does not prove Piclaw's exact module graph. |

Focused `make test-ux-parity` filter for all five tags passed **30/30**
across Chromium/WebKit phone, tablet and desktop projects. This does not
exercise current Piclaw UI, physical assistive tech, deployed Gi, or all
settings controls. No production code or frozen Gherkin changed.
