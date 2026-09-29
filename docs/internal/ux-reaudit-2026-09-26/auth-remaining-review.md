# Classic auth 003, 005–014: mounted-path boundary

This extends `auth-basic-review.md`, which already covers `001` (family gap),
`002` (bounded native TOTP), and `004` (bounded policy Retry). The frozen
requirements are `tests/ux/features/classic/canonical/core-auth.feature`.
Gi's mounted entry is `web/src/app.ts` with `GiAuthGate` (`gi-auth.ts`);
`gi-auth-policy.ts` accepts only `single-user`. The separate copied Classic
`login.ts`, `ui/oobe-state.ts`, and `ui/app-main-shell-render.ts` are present in
the source tree but are not mounted by that Gi app entry. The installed login
policy/form slice has a separate disposable browser probe; no invitation or
family runtime was browser-probed here.

| IDs | Requirement-to-Gi trace | Qualification |
|---|---|---|
| `003` | `gi-auth.ts` gates TOTP code controls on `totp_login_available` and passkey button on `passkey_login_available`; `gi-auth-policy.ts` validates their types. An untagged mounted browser check passed 6/6 viewports with a disposable passkey-only policy: no code control, passkey button visible and enabled only where browser credential APIs permit, but the button remains inside a visible `<form>`. Installed Classic 6/6 hid its TOTP `#login-form`. | Native presentation gap under the frozen hidden-form wording; no WebAuthn ceremony or full parity. |
| `005` | Mounted Gi passkey prompt has an AbortController and explicit cancel; no ambient conditional passkey request is started by `GiAuthGate`. Copied Classic `login.ts` starts conditional mediation but its `passkeyInFlight` guard does not abort ambient work to let an explicit attempt supersede it. | Native race contract gap; cancellation of a prompted attempt is not this two-way preemption scenario. |
| `006`–`007` | `ui/oobe-state.ts` chooses `provider-missing` only with known model readiness, no available/configured model, no dismissal and no popout, with four focused state helper tests passing. `OobePanel` is rendered by copied `app-main-shell-render.ts`, not mounted `GiApp` in `app.ts`. | Source-only, unmounted Classic OOBE; no tagged Gi/browser journey. The five outline rows of `007` do not acquire case acceptance from unit source tests. |
| `008`–`012` | No native Gi invitation page/claim/confirmation endpoint or `recovery_only` flow was found under `web/src` or `internal/web`. Single-user owner setup/passkey Settings are different workflows. | Verified native invitation capability gap for TOTP, recovery-only and passkey invitation, including one-use cancellation cases. No tagged journey. |
| `013` | Family mode is rejected by Gi policy. No mounted family client with blur/hidden masking and identity rechecks. | Verified native family privacy gap; ordinary notification `visibilitychange` is not family masking. |
| `014` | Settings' native sign-out confirms intent, disables while busy, attempts notification cleanup, POSTs logout, checks auth status, and reports uncertainty on failure; `gi-auth.ts` re-reads policy after sign-out. | Partial single-user analogy only. No family-client sign-out/mask state, no tagged frozen journey, and no claim of navigation to Piclaw `/login`. |

`bun test web/src/ui/oobe-state.test.ts` passed **4/4** source-helper
assertions. Focused `@ux-auth-002|004` plus the untagged `003` gap check passed **18/18**;
these do not fill the other ten clauses. The broad auth suite was interrupted
at 105/120 in the earlier review, not a green gate. No production code,
frozen contract, or live instance was changed here.
