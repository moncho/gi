# Classic timeline rendering/actions 023–028: scoped native tests

The active clauses are in `features/ux/classic/timeline/rendering.feature`.
Piclaw 3.2.4's pinned source manifest marks `runtime/web/src/components/post.ts`
changed in Gi. A mounted installed-Classic probe now checks one existing Gi
Markdown-table path. Other rows remain Gi-native checks against source-backed
contracts, without byte-level or whole-feature parity credit.

| ID | Gi assertion/code path | Bounded result |
|---|---|---|
| `023` | Gi `rendering.spec.mjs` checks computed table display, auto layout, full post width and Unicode rows on a stored assistant message. Mounted Piclaw 3.2.4 independently renders the same Markdown text from a disposable assistant post: computed `display:table`, `table-layout:auto`, table width within 1 px of its parent, two rows and Chinese text. | Each path passed **6/6** across Chromium/WebKit phone/tablet/desktop. Source CSS alone is insufficient; both browser measurements establish this geometry subset. Piclaw fixture does not test persisted backend content or pixel equivalence. |
| `024` | Gi's `rendering.spec.mjs` checks a top-right code-copy button and a trusted native `copy` event containing exact code text, not markup. Mounted installed Piclaw 3.2.4 independently renders the same fenced code from a disposable assistant post: button within 12 px of the block's top/right, trusted `copy` event with exact Unicode/source text and draft retained. | Each path passed **6/6** across Chromium/WebKit phone/tablet/desktop. This is browser copy-event evidence, not OS clipboard persistence/paste or physical permission acceptance. |
| `025` | Gi `remote-links.spec.mjs` checks native persisted resource/preview new tabs, `noopener noreferrer`, null opener/referrer, retained draft/attachment and stored messages. Mounted Piclaw 3.2.4 independently renders disposable resource/preview blocks: both open intercepted remote pages in new tabs with null opener and no referrer; the draft survives. | Each path passed **6/6** Chromium/WebKit phone/tablet/desktop. The installed fixture does not test Gi storage/search/reload or a live remote target trust boundary. |
| `026` | Gi `outcomes.spec.mjs` checks a recovered chip after timestamp on the same metadata row, persistence through reload/search, and absence on an ordinary turn. Mounted Piclaw 3.2.4 independently renders disposable recovered/ordinary assistant posts: `recovered` follows `.post-time` on one row, its tooltip says `Recovered after 2 attempts`, an ordinary post has no chip, and the chip reappears on fixture reload. | Each path passed **6/6** Chromium/WebKit phone/tablet/desktop. Gi uses a native stale-claim recovery fixture; installed probe does not execute a retry, search or draft persistence. |
| `027` | `speech-contract.spec.mjs` checks speakable native assistant text and supported/unsupported browser speech APIs; `gi-post-speech.ts` normalizes text and checks synthesis methods. | Tagged 6/6 with `GI_UX_SPEECH=1`; no actual audible output/physical-device validation. |
| `028` | `speech.spec.mjs` checks second-post ownership transfer, cancel of earlier utterance, and stale callback fencing; `gi-post-speech.ts` tracks owner and session. | Tagged 6/6 with stubbed speech API, not audible playback acceptance. |

Focused commands passed **12/12** (`023`–`024`), **6/6** (`025`), **6/6**
(`026`), and **12/12** (`027`–`028`). Those independent runs do not amount to
a whole product gate, deployed Gi check, or physical/pixel parity.
`make test-piclaw-markdown-table` passed **6/6** installed mounted Classic
cases, while focused Gi `@ux-timeline-023` passed **6/6** on native stored
messages. The installed probe uses disposable timeline data, not a live
provider. It confirms fidelity of Gi's existing table layout contract without
porting the other timeline features. `make test-piclaw-code-copy` and focused
Gi `@ux-timeline-024` each passed **6/6** separately with the same code text.
The installed oracle uses disposable timeline data. Neither run tests OS
clipboard persistence or the other rendering/action clauses.
`make test-piclaw-remote-links` passed **6/6** mounted installed cases for
resource and preview destinations; the focused Gi `@ux-timeline-025` journey
passed **6/6** with native stored messages. Piclaw URLs were intercepted by
the fixture rather than fetched from real remote services.
`make test-piclaw-outcome-chip` passed **6/6** installed metadata checks and
focused Gi `@ux-timeline-026` passed **6/6** with its native recovery fixture.
The installed fixture uses static marker data, not a real retry. No frozen text
or production code changed.
