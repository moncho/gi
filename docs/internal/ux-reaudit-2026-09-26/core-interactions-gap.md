# Classic core interactions: five remaining clause audits

`tests/ux/features/classic/canonical/core-interactions.feature` contains
three previously reviewed recovery/card-rejection clauses (`003`, `012`,
`013`), which are not rescored here. The pinned Piclaw 3.2.4 source manifest
marks `btw-panel.ts`, `floating-widget-pane.ts`,
`adaptive-card-submission.ts` and `notification-delivery-coordinator.ts`
identical; `generated-widget.ts` changed in Gi. Source presence is not a
mounted Gi capability or current Piclaw runtime acceptance.

| ID | Frozen clause versus mounted Gi | Bounded finding |
|---|---|---|
| `@ux-extra-001` | `btw-panel.ts` has the expected question/thinking/answer/Retry/Inject conditions, and `app-main-shell-render.ts` supplies callbacks. The mounted Gi entry `web/src/app.ts` does **not** render that shell's BTW panel. | Native mounted-capability gap; no tagged browser journey. Unused copied component cannot earn parity. |
| `@ux-extra-002` | Copied `adaptive-card-submission.ts` checks only string `card_id`, number `source_post_id`, string `submitted_at`; it does not enforce nonempty/bounded identifier, positive safe integer or parseable date. `post.ts` accepts `Action.Submit` events but `api.ts:590` deliberately rejects submission until identity/authorization/persistence exist. | Native validation/submission gap. Do not infer support from the type declaration or rejection UI (`003`). |
| `@ux-extra-004` | `generated-widget.ts` builds persisted payloads only with usable artifact content and distinguishes live `loading`/`streaming`/`final`/`error` status. `post.ts` and `FloatingWidgetPane` mount in Gi, but `gi-sse-client.ts` does not bind the widget events and `app.ts` has no mounted live-widget event branch. | Installed persisted artifact probe 6/6 and Gi mounted HTTP/SQLite persisted slice 6/6; installed widget SSE probe 6/6 reports raw widget frames delivered without a pane. See [widget audit](classic-core-widget-review.md). No live Gi journey or whole-clause acceptance. |
| `@ux-extra-005` | `web/src/app.ts` closes the mounted floating pane with `setFloatingWidget(null)` and has no explicit dismissed-session-key update; the separate, unused composition chain implements such a dismissal. Close does not call queue mutation in the mounted callback. | Partial native behaviour: no live pane mounts from SSE in either shipped Piclaw or Gi, so the frozen visible-live-widget dismissal precondition was not exercised. No tagged browser journey or clause acceptance. |
| `@ux-extra-011` | `notification-delivery-coordinator.ts` computes same-device live presence, suppresses for visible candidates, otherwise chooses lowest client ID; withdraw removes stored presence. `gi-notifications-state.ts` uses the coordinator with locking and cleanup. | Helpers **10/10**, focused `make test-ux-notifications` **24/24** across six browser projects. Tests are untagged to this frozen ID and mock browser permission/delivery; no current Piclaw or OS acceptance. |

The earlier `@ux-extra-003`, `012`, `013` bounded review remains in
`recovery-card-review.md`; none of these five findings upgrades those clauses
or the full Classic shell. No production code or frozen Gherkin changed.
