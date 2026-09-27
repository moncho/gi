# Classic028: code copy and speech ownership

`web/src/components/post.ts` supplies a code-block copy control and the
read-aloud button. `web/src/gi-post-speech.ts` holds one active owner, cancels
prior playback and fences callbacks from an older utterance. Gi speech text
omits fenced code and has a 1600-character cap.

The tagged `@ux-original-028` case in `tests/ux/speech-contract.spec.mjs`
checks a trusted plain-text code-copy event without HTML, then transfers
speech ownership between assistant posts while preserving native draft and
stored messages. Focused `test-ux-steer` run: **6/6** across Chromium/WebKit
phone, tablet and desktop. The browser speech API is stubbed for deterministic
ownership assertions; audible output and physical/assistive-device behaviour
were not tested. The installed Piclaw 3.2.4 speech UI was not run.

`timeline-rendering-review.md` contains the narrower `@ux-timeline-024`,
`027` and `028` evidence. This case binds the two actions in one journey but
does not add current-oracle acceptance. No production code or frozen Gherkin
changed.
