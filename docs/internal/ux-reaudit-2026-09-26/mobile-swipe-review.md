# Classic mobile swipe 001–006: clause review

The installed Piclaw 3.2.4 `ui/chat-swipe-navigation.ts` is byte-identical to
frozen 70d33bc (`frozen-to-oracle-sources.json`). The source checks target
eligibility, text selection, direction, candidate ordering and Safari wheel
gates. It is not a current Piclaw physical-touch or browser acceptance run.
Playwright dispatches browser touch/wheel events against disposable Gi sessions.

| Frozen ID | Gi assertion → handler | Limit |
|---|---|---|
| `@ux-mobile-001` | `tests/ux/session.spec.mjs:297–331` swipes eligible timeline space, checks adjacent session and wrap, and restores per-session drafts. Gi `web/src/ui/chat-swipe-navigation.ts` handles candidate selection and threshold. | Browser-dispatched touch, not physical iOS/Android. |
| **`@ux-mobile-002`** (seven outline examples) | `web/src/ui/chat-swipe-navigation.ts:59–105` lists exclusion selectors; mounted `web/src/app.ts:858–878` also gates starts to the timeline or sibling status panels. Real composer and Settings-control exclusions have adjacent assertions in `status-swipes.spec.mjs`; `session.spec.mjs` checks Copy and input targets. | **Unmapped outline:** no individually tagged seven-example journey. Mounted Gi has read-only preview tabs (`editor-pane-container`) but no terminal dock: `onOpenTerminalTab` and `onOpenVncTab` are empty. The explorer mounts as `.workspace-sidebar`, which is outside the swipe host; `.workspace-explorer` alone is not proof for it. The attachment modal is a body portal outside the swipe host. Card controls and picker menus mount, but their exclusion has no tagged gesture check. Selector-only or synthetic unmounted DOM does not earn an expanded-case pass. |
| `@ux-mobile-003` | `tests/ux/status-swipes.spec.mjs:16+` checks real streamed draft/status panel link gestures, selected-text and direction guards, excluded input/settings controls, pen-contact guard and actual timeline navigation. Gi `chat-swipe-navigation.ts:89–105` permits designated thinking/status ancestors. | The separate thought variant `@gi-swipe-003` is not a Piclaw or physical touch pass. |
| `@ux-mobile-004` | `tests/ux/session.spec.mjs:770–846` mixes active, pinned, ordinary and archived sessions, checks deduplicated active-first/chat-JID order despite selection/unpinning, and keeps drafts. Gi `chat-swipe-navigation.ts:106+` filters archived and sorts. | Does not test an arbitrarily changing remote roster on a live peer. |
| `@ux-mobile-005` | `session.spec.mjs:332–365` first checks predominantly vertical gesture cancellation, then performs a fresh eligible horizontal swipe as a positive control. Gi `chat-swipe-navigation.ts:177+` gates direction/threshold. | Browser event emulation, not OS gesture arbitration. |
| `@ux-mobile-006` | `session.spec.mjs:366–404` checks no wheel navigation under Chrome/iOS mode and positive desktop-Safari path with draft ownership. Gi `chat-swipe-navigation.ts` gates browser and axis. | Emulated browser mode and wheel events do not establish physical trackpad behaviour. |

`make test-ux-parity UX_PARITY_PORT=19134
UX_PARITY_ARGS='tests/ux/session.spec.mjs --grep "@ux-mobile-00[1456]"'`
passed **24/24**. `make test-ux-status-swipes` passed **12/12**, six of which
carry `@ux-mobile-003`; the other six are Gi thought-panel variants. These
native runs qualify the named assertions but do not fill `@ux-mobile-002`'s
seven-surface gap or establish current Piclaw runtime/physical-device parity.
Where a frozen example names a surface absent from mounted Gi, preserve that
capability gap instead of constructing a selector-only substitute.
