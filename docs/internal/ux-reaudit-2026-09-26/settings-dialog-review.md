# Classic settings dialog 001–005: bounded native shell

`tests/ux/features/classic/settings/settings-dialog.feature` is a frozen
Piclaw Classic contract. Gi mounts `web/src/gi-settings.ts` with a modal
`BodyPortal`, a General snapshot cache, a cold loading shell, and pane modules
loaded on visit by `gi-settings-lazy.ts`. The installed 3.2.4 Classic Settings
shell was probed separately with disposable responses. The new shortcut probe
checks one target-sensitive opening rule; these tests do not establish parity
for every setting or subsection.

| ID | Tagged native assertion | Boundary |
|---|---|---|
| `001` | Three rapid Control-comma presses from a focused composer leave one Gi Settings dialog and portal; Escape restores composer focus. `GiSettings` opens in a capture-phase window listener without an editable-target guard. Installed Classic 3.2.4 blocks all three presses from its focused composer textarea, but opens one dialog after three presses from noneditable body focus. | **Target-sensitive Gi divergence.** The frozen scenario does not specify where focus starts; the Gi test does. Installed and Gi assertions cannot be merged into a whole-clause pass. Browser key events do not establish OS/physical shortcuts. |
| `002` | Gi opens via hamburger, visits panes, then reopens while `/api/runtime/config` is held: General shows a cached snapshot in under one second, one portal, no draft/media loss or writes. Installed Piclaw 3.2.4 mounted Classic independently reopens with a cached General compose-upload value `64` while a second `/agent/settings-data` read is held; it renders within one second, then updates to `96` after release with one dialog and an intact draft. | Both times come from disposable browser fixtures, not a global latency SLO. Piclaw performs a fresh read on reopen while showing cached data; Gi's route and cached values differ. No production config mutation or whole-clause credit for every pane. |
| `003` | Held first runtime-config read shows immediate `Loading settings…`, General selected first, values resolve within two seconds without blank shell. | Same fixture timing boundary. |
| `004` | Compaction numeric spinbutton accepts typed `128000` without saving/mutating settings. | Typability only, not valid policy/application of that value. |
| `005` | General shows without other pane chunks; Models and subsequent built-ins load on click and cached revisits reuse modules. | Test is stricter about Gi's five pane chunks, but does not prove Piclaw's exact module graph. |

Focused `make test-ux-parity` filter for all five tags previously passed
**30/30** across Chromium/WebKit phone, tablet and desktop projects. The
current `@ux-settings-dialog-001` Gi run passed **6/6**. `make
test-piclaw-settings-shortcut` passed **6/6** installed mounted Classic cases:
focused textarea blocks Control-comma, noneditable body opens one dialog on
three presses and retains a draft. The separate installed shell probe covers
General/Escape/backdrop, not this shortcut boundary. No settings writes,
physical input, deployed Gi or whole-settings acceptance was tested. The Gi
handler and frozen Gherkin were left unchanged pending a decision on the
composer-focus divergence.

`make test-piclaw-settings-reopen` passed **6/6** installed Classic cases
across Chromium/WebKit phone/tablet/desktop. The focused Gi `002` journey
passed **6/6** separately; it includes draft/media ownership and pane visits that the
installed probe did not exercise. The installed fixture held a second read;
it did not verify live settings persistence or latency under network load.
