# Classic session switching 001–006: bounded clause review

Shipped Piclaw 3.2.4 asset `990f0c49a932` supplies the current source oracle.
`ui/compose-session-switcher.ts`, `ui/app-browser-events.ts` and
`ui/chat-swipe-navigation.ts` are byte-identical to the frozen 70d33bc source
(`frozen-to-oracle-sources.json`). A mounted installed-Classic fixture now
checks search, Escape dismissal and filtered keyboard selection against a
disposable two-chat catalogue. The other rows are source→Gi native-assertion→Gi-handler
traces; the fixture does not establish whole-feature or physical-input acceptance.

| Frozen ID | Gi clause evidence and handler | Boundary |
|---|---|---|
| `@ux-session-001` selected timeline | `tests/ux/session.spec.mjs:526–568` creates two native sessions with different completed messages, holds the old session's message response, selects research in the real picker and checks selection, correct timeline/draft and rejection of the late old read. Gi selected-session reads and generation guards are exercised. | The test does not compare Piclaw rendering or physical picker. |
| `@ux-session-002` groups | `session.spec.mjs:169–212` checks Current, Pinned, Active, This session tree, Other sessions and Archived in order, current marker and an active-entry action gate. Gi `web/src/ui/compose-session-switcher.ts:112–157` groups current metadata. | Fixture-created catalogue; no fresh-browser/all-client roster acceptance. |
| `@ux-session-003` search | `session.spec.mjs:405–452` searches identifier and model metadata, navigates a filtered list with keyboard and preserves drafts while selecting. Gi `compose-session-switcher.ts:112–129` matches metadata and `compose-box.ts` renders it. Installed Piclaw's mounted picker filters a disposable two-chat list for the research name and restores both on query reset; Home, End and ArrowDown retain the sole filtered selection and Tab activates it, changing the fixture URL to `web:research`. | Gi's tagged native search/arrow/Enter journey passed 6/6 separately. Its untagged Tab-focus journey passed 6/6: Tab moves to Pin and then the row without selecting, while installed Classic uses Tab to activate the selected row. Model metadata and physical IME were not compared in the installed probe. |
| `@ux-session-004` archive/restore | `session.spec.mjs:648–706` holds an archive PATCH acknowledgement to reject optimistic success, verifies accepted archive/restore and catalogue refresh against native state, then aborts another PATCH and checks an error and unchanged state/draft. Gi session mutation handler in `compose-box.ts` owns the captured chat. | Does not prove every unavailable action or cross-device session sync. |
| `@ux-session-005` swipe eligibility | `session.spec.mjs:213–296` checks archived exclusion, active-first/chat-ID carousel order, selected text and control exclusions, native session/draft changes. Gi `web/src/ui/chat-swipe-navigation.ts` filters archived candidates and sorts active-first. | Playwright touch events in viewport projects are emulated; not a physical phone or tablet. |
| `@ux-session-006` dismissal | `session.spec.mjs:79–122` opens Gi's native picker, enters a query, presses Escape and checks popup dismissal, query reset on reopen, trigger focus, unchanged selected session/draft and history. Installed Piclaw 3.2.4 mounted picker independently filters a read-only two-chat list, dismisses on Escape, restores trigger focus asynchronously, clears the query on reopen and keeps the URL chat unchanged. | Installed fixture does not test a saved draft, backend mutation, typeahead state, outside-click or assistive technology. Gi and installed evidence are separate. |

`make test-ux-parity UX_PARITY_PORT=19134
UX_PARITY_ARGS='tests/ux/session.spec.mjs --grep "@ux-session-00[1-6]"'`
passed **36/36** in Chromium/WebKit phone/tablet/desktop projects with a
disposable Gi server. Passing tags alone did not establish the clauses; the
assertions and source above bound the six findings.
`make test-piclaw-session-picker-dismiss` passed **6/6** installed Piclaw
3.2.4 mounted Classic cases across Chromium/WebKit phone/tablet/desktop. It checks
search, Escape and single-result keyboard selection, including Tab activation.
Piclaw timeline content after selection, grouped roster actions, backend
archive/restore, real sessions and physical input still need separate comparison.
Gi's focused `@ux-session-003` and untagged Tab-focus runs passed **6/6** each;
the earlier focused `@ux-session-003/006` run passed **12/12**. Inventory passed 218 helper tests
and 8,004 assertions without adding whole-clause parity credit.
