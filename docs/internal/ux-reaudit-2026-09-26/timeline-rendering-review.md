# Classic timeline rendering/actions 023–028: scoped native tests

The active clauses are in `features/ux/classic/timeline/rendering.feature`.
Piclaw 3.2.4's pinned source manifest marks `runtime/web/src/components/post.ts`
changed in Gi. A mounted installed-Classic probe now checks one existing Gi
Markdown-table path. Other rows remain Gi-native checks against source-backed
contracts, without byte-level or whole-feature parity credit.

| ID | Gi assertion/code path | Bounded result |
|---|---|---|
| `023` | Gi `rendering.spec.mjs` checks computed table display, auto layout, full post width and Unicode rows on a stored assistant message. Mounted Piclaw 3.2.4 independently renders the same Markdown text from a disposable assistant post: computed `display:table`, `table-layout:auto`, table width within 1 px of its parent, two rows and Chinese text. | Each path passed **6/6** across Chromium/WebKit phone/tablet/desktop. Source CSS alone is insufficient; both browser measurements establish this geometry subset. Piclaw fixture does not test persisted backend content or pixel equivalence. |
| `024` | Same spec checks a top-right code-copy button and observes a trusted native `copy` event containing exact code text, not markup. `post.ts` renders the action. | Tagged 6/6; clipboard behavior is browser-emulated, not physical permission acceptance. |
| `025` | `remote-links.spec.mjs` checks resource/preview new tabs, `noopener noreferrer`, null opener/referrer, retained draft and stored messages. `post.ts` includes isolated link attributes. | Tagged 6/6 with `GI_UX_LINKS=1` fixture, not a live remote target trust audit. |
| `026` | `outcomes.spec.mjs` checks a recovered chip after timestamp on the same metadata row, persistence through reload/search, and absence on an ordinary turn. | Tagged 6/6 with `GI_UX_OUTCOMES=1` stale-claim fixture. |
| `027` | `speech-contract.spec.mjs` checks speakable native assistant text and supported/unsupported browser speech APIs; `gi-post-speech.ts` normalizes text and checks synthesis methods. | Tagged 6/6 with `GI_UX_SPEECH=1`; no actual audible output/physical-device validation. |
| `028` | `speech.spec.mjs` checks second-post ownership transfer, cancel of earlier utterance, and stale callback fencing; `gi-post-speech.ts` tracks owner and session. | Tagged 6/6 with stubbed speech API, not audible playback acceptance. |

Focused commands passed **12/12** (`023`–`024`), **6/6** (`025`), **6/6**
(`026`), and **12/12** (`027`–`028`). Those independent runs do not amount to
a whole product gate, deployed Gi check, or physical/pixel parity.
`make test-piclaw-markdown-table` passed **6/6** installed mounted Classic
cases, while focused Gi `@ux-timeline-023` passed **6/6** on native stored
messages. The installed probe uses disposable timeline data, not a live
provider. It confirms fidelity of Gi's existing table layout contract without
porting the other timeline features. No frozen text or production code changed.
