# Classic timeline rendering/actions 023–028: scoped native tests

The frozen clauses are in `tests/ux/features/classic/timeline/rendering.feature`.
Piclaw 3.2.4's pinned source manifest marks `runtime/web/src/components/post.ts`
changed in Gi; the shipped Piclaw UI interactions were not replayed here.
Gi's post component, table styles, recovery metadata and speech owner are
traced against focused native browser assertions instead of awarding byte-level
source parity.

| ID | Gi assertion/code path | Bounded result |
|---|---|---|
| `023` | `rendering.spec.mjs` checks computed table display, auto layout, full post width and Unicode rows. Markdown/table styling is in `post.ts` and `content.css`; a second Gi test checks wide-table overflow separately. | Tagged 6/6 across six viewport/browser projects. Source CSS alone has a `display:block` base rule; the computed test, not a CSS grep, supplies rendered evidence. |
| `024` | Same spec checks a top-right code-copy button and observes a trusted native `copy` event containing exact code text, not markup. `post.ts` renders the action. | Tagged 6/6; clipboard behavior is browser-emulated, not physical permission acceptance. |
| `025` | `remote-links.spec.mjs` checks resource/preview new tabs, `noopener noreferrer`, null opener/referrer, retained draft and stored messages. `post.ts` includes isolated link attributes. | Tagged 6/6 with `GI_UX_LINKS=1` fixture, not a live remote target trust audit. |
| `026` | `outcomes.spec.mjs` checks a recovered chip after timestamp on the same metadata row, persistence through reload/search, and absence on an ordinary turn. | Tagged 6/6 with `GI_UX_OUTCOMES=1` stale-claim fixture. |
| `027` | `speech-contract.spec.mjs` checks speakable native assistant text and supported/unsupported browser speech APIs; `gi-post-speech.ts` normalizes text and checks synthesis methods. | Tagged 6/6 with `GI_UX_SPEECH=1`; no actual audible output/physical-device validation. |
| `028` | `speech.spec.mjs` checks second-post ownership transfer, cancel of earlier utterance, and stale callback fencing; `gi-post-speech.ts` tracks owner and session. | Tagged 6/6 with stubbed speech API, not audible playback acceptance. |

Focused commands passed **12/12** (`023`–`024`), **6/6** (`025`), **6/6**
(`026`), and **12/12** (`027`–`028`). Those independent runs do not amount to
a whole product gate, direct current-oracle interaction, deployed Gi check,
or physical/pixel parity. No frozen text or production code changed.
