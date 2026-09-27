# Classic recovery and card rejection: bounded clause review

Current oracle: shipped Piclaw 3.2.4 asset `990f0c49a932`, inspected via its
`app.bundle.js.map`. These three Classic scenarios have native Gi browser
journeys. Piclaw's corresponding post/card UI was **not** run with the fixture;
source equivalence and a Gi pass alone cannot establish current-oracle UX
parity. All Gi tests below used disposable session/fixture data.

| Frozen ID | Installed Piclaw source | Gi assertion → handler; remaining boundary |
|---|---|---|
| `@ux-extra-003` rejected card action | `ui/adaptive-card-renderer.ts:435–445` clears prior notice, marks the card busy, reports a rejected `onAction` as an error notice and removes busy state. This module is byte-identical to frozen source. Installed `components/post.ts:1988–2009` awaits `Action.Submit`; Post has changed elsewhere, so the shipped source was checked. | `tests/ux/card-rejection.spec.mjs:6+` activates a fixture card by mouse and Enter. It checks rejection notice (no success), retained card input and composer draft through reload, unchanged stored messages, no turn/write and no page error. Gi `web/src/ui/adaptive-card-renderer.ts:311–320` and `web/src/components/post.ts:972–995` own that path. This fixture cannot prove a successful server/card submission or hardware assistive technology. |
| `@ux-extra-012` hidden recovery control | Installed `components/post.ts:170+` validates the protected recovery control-intent block and returns no visible post when accepted (`2031`). The module **changed** versus frozen source; source hash alone is not behaviour. | `tests/ux/recovery-controls.spec.mjs:5+` checks validated control rows are absent while malformed lookalikes stay visible, then reloads/searches without mutating native messages or losing draft/media. Gi `web/src/gi-recovery-control.ts` validates display-only typed fields; `scripts/patch-post-recovery-control.mjs` applies the guarded hide after hooks. This does not grant control authority or cover every malformed value. |
| `@ux-extra-013` silent informational recovery placeholder | Installed `components/post.ts:2022–2031` returns no post only for an agent informational recovery marker with no renderable text, media, card or submission. | `tests/ux/recovery-placeholders.spec.mjs:5+` checks empty info rows are hidden while prose, attachments, cards, submissions, references, resources and warnings remain visible. It also verifies draft preservation, search, unchanged stored messages and no writes. Gi `web/src/gi-recovery-placeholder.ts` and the guarded Post patch implement this display-only rule. Search/fixture coverage is not live Piclaw backend acceptance. |

`make test-ux-steer` with `GI_UX_CARD_REJECTION=1
GI_UX_RECOVERY_CONTROLS=1 GI_UX_RECOVERY_PLACEHOLDERS=1`, these three specs
and `--grep "@ux-extra-00[3]|@ux-extra-01[23]"` passed **18/18** in six
Chromium/WebKit viewport projects. The output has no current Piclaw browser
probe or physical-device assertions. These rows stay source-backed/native-test
qualified, not parity-complete.
