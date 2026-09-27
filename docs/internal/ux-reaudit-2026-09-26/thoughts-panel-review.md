# Classic thought/draft disclosure 001–005: bounded clause review

The installed Piclaw 3.2.4 `components/status.ts` is byte-identical to the
frozen 70d33bc source (`frozen-to-oracle-sources.json`). Piclaw's live status
panel was **not** run here. The native Gi test uses a disposable streaming
fixture; its tags are connected to assertions below, not automatic parity.

| Frozen ID | Native assertion and Gi handler | Limit |
|---|---|---|
| `@ux-thoughts-001` | `tests/ux/thoughts.spec.mjs:91+` checks `data-expanded=false`, rendered overflow for a wrapped paragraph, and disclosure remeasurement on viewport/container resize and later streamed text. Gi `web/src/components/status.ts` renders disclosure; `web/src/gi-preview-overflow.ts` measures clipping. | Test does not claim a fixed line count or physical pixel identity. |
| `@ux-thoughts-002` | `thoughts.spec.mjs:20–61` keeps a collapsed Thoughts/Draft panel while accepted streamed text grows and checks the new text is present without opening it. | Fixture-supplied text is not provider thought semantics. |
| `@ux-thoughts-003` | The same test activates separate Thoughts and Draft disclosure controls and checks each panel's expansion state without discarding text. `status.ts:377+` exposes state; callback/local-state ownership is source reviewed. | The browser assertion does not isolate every supplied-callback branch. |
| `@ux-thoughts-004` | The same native test presses unmodified Escape to collapse an expanded status panel; editable focus and modified Escape stay with their controls. `status.ts:242–251` guards the event. | No hardware keyboard or assistive-technology pass. |
| `@ux-thoughts-005` | Expanding/collapsing after streaming keeps full thought/draft text and scroll behaviour, with the composer draft unchanged. Gi preview state is in `web/src/ui/app-agent-previews.ts`. | No full Piclaw current-UI comparison. |

An earlier broad `make test-ux-thoughts` run passed 47/48 but Chromium tablet
`@gi-preview-001` stayed on `Loading Gi…` during startup, before thought
assertions. The focused `@ux-thoughts-00[1-5]` run was aborted after 28/30,
so it has no result. An unchanged rerun via `make test-ux-steer` with
`GI_UX_THOUGHTS=1` and the five-tag grep passed **30/30** across six
Chromium/WebKit viewport projects. Neither failure was fixed or credited as a
green broad gate. The current Piclaw runtime and physical devices remain
unprobed.
