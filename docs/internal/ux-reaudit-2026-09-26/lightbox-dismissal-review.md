# Classic lightbox dismissal 013–016: bounded clause review

Shipped Piclaw 3.2.4 `components/image-modal.ts` is byte-identical to the
frozen 70d33bc component (`frozen-to-oracle-sources.json`). It closes on
Escape and on a click inside the modal wrapper, including the image. Gi uses
the supplied `web/src/components/image-modal.ts` through `BodyPortal` with
native stored-media URLs. Piclaw's own lightbox was **not** opened in this
review; Gi browser tests cannot stand in for current Piclaw UI or physical
touch acceptance.

| Frozen ID | Gi assertion → code | Limit |
|---|---|---|
| `@ux-timeline-013` | `tests/ux/lightbox.spec.mjs:31–35` opens a real stored image, presses Escape, checks dismissal and timeline, draft and attachment preservation, including after reload. `image-modal.ts` owns Escape. | No screen-reader focus-trap claim. |
| `@ux-timeline-014` | `lightbox.spec.mjs:36–40` presses Space, Enter, a letter and an arrow; each leaves the modal open with no send/queue mutation or draft change. | No IME/hardware keyboard pass. |
| `@ux-timeline-015` | `lightbox.spec.mjs:41+` clicks both backdrop and image and checks modal closes. The wrapper's click handler receives the image click. | Mouse/browser pointer only. |
| `@ux-timeline-016` | `lightbox.spec.mjs:78+` uses `hasTouch:true`, taps backdrop and image, checks closure and four trusted single-touch starts while retaining native media and drafts. | Emulated trusted touch is **not** a physical phone/tablet or OS gesture pass. |

The fixture uploads `native-image.png`, reads the native stored bytes back,
and leaves an unsent draft with `keep.txt`. `make test-ux-parity
UX_PARITY_PORT=19134 UX_PARITY_ARGS='tests/ux/lightbox.spec.mjs --grep
"@ux-timeline-01[3-6]"'` passed **24/24** across Chromium/WebKit
phone/tablet/desktop projects. This bounds four native journeys; no current
Piclaw 3.2.4 runtime or device acceptance is claimed.
