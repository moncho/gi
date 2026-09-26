# Classic session switching 001–006: bounded clause review

Shipped Piclaw 3.2.4 asset `990f0c49a932` supplies the current source oracle.
`ui/compose-session-switcher.ts`, `ui/app-browser-events.ts` and
`ui/chat-swipe-navigation.ts` are byte-identical to the frozen 70d33bc source
(`frozen-to-oracle-sources.json`). Piclaw's session picker was **not** run in a
current-UI fixture here. These are source→Gi native-assertion→Gi-handler traces,
not current-release runtime or physical-touch acceptance.

| Frozen ID | Gi clause evidence and handler | Boundary |
|---|---|---|
| `@ux-session-001` selected timeline | `tests/ux/session.spec.mjs:526–568` creates two native sessions with different completed messages, holds the old session's message response, selects research in the real picker and checks selection, correct timeline/draft and rejection of the late old read. Gi selected-session reads and generation guards are exercised. | The test does not compare Piclaw rendering or physical picker. |
| `@ux-session-002` groups | `session.spec.mjs:169–212` checks Current, Pinned, Active, This session tree, Other sessions and Archived in order, current marker and an active-entry action gate. Gi `web/src/ui/compose-session-switcher.ts:112–157` groups current metadata. | Fixture-created catalogue; no fresh-browser/all-client roster acceptance. |
| `@ux-session-003` search | `session.spec.mjs:405–452` searches identifier and model metadata, navigates a filtered list with keyboard and preserves drafts while selecting. Gi `compose-session-switcher.ts:112–129` matches metadata and `compose-box.ts` renders it. | Real text-entry keys; no physical IME pass. |
| `@ux-session-004` archive/restore | `session.spec.mjs:648–706` holds an archive PATCH acknowledgement to reject optimistic success, verifies accepted archive/restore and catalogue refresh against native state, then aborts another PATCH and checks an error and unchanged state/draft. Gi session mutation handler in `compose-box.ts` owns the captured chat. | Does not prove every unavailable action or cross-device session sync. |
| `@ux-session-005` swipe eligibility | `session.spec.mjs:213–296` checks archived exclusion, active-first/chat-ID carousel order, selected text and control exclusions, native session/draft changes. Gi `web/src/ui/chat-swipe-navigation.ts` filters archived candidates and sorts active-first. | Playwright touch events in viewport projects are emulated; not a physical phone or tablet. |
| `@ux-session-006` dismissal | `session.spec.mjs:79–122` opens the native picker, enters a query, presses Escape and checks hidden popup, cleared query/typeahead on reopen, focus returned to the trigger, unchanged selected session/draft and no mutation. Gi `compose-box.ts:1048+` closes with conditional focus restoration. | Does not cover every outside-click or assistive-technology path. |

`make test-ux-parity UX_PARITY_PORT=19134
UX_PARITY_ARGS='tests/ux/session.spec.mjs --grep "@ux-session-00[1-6]"'`
passed **36/36** in Chromium/WebKit phone/tablet/desktop projects with a
disposable Gi server. Passing tags alone did not establish the clauses; the
assertions and source above bound the six findings. A current Piclaw 3.2.4
session-picker browser comparison is still needed for present-release parity.
