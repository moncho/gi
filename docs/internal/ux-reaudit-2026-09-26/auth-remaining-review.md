# Classic auth 003, 005–014: mounted-path boundary

This extends `auth-basic-review.md`, which already covers `001` (family gap),
`002` (bounded native TOTP), and `004` (bounded policy Retry). The frozen
requirements are `tests/ux/features/classic/canonical/core-auth.feature`.
Gi's mounted entry is `web/src/app.ts` with `GiAuthGate` (`gi-auth.ts`);
`gi-auth-policy.ts` accepts only `single-user`. The separate copied Classic
`login.ts`, `ui/oobe-state.ts`, and `ui/app-main-shell-render.ts` are present in
the source tree but are not mounted by that Gi app entry. Installed login and
OOBE slices have separate disposable browser probes; no invitation or family
runtime was browser-probed here.

| IDs | Requirement-to-Gi trace | Qualification |
|---|---|---|
| `003` | `gi-auth.ts` gates TOTP code controls on `totp_login_available` and passkey button on `passkey_login_available`; `gi-auth-policy.ts` validates their types. An untagged mounted browser check passed 6/6 viewports with a disposable passkey-only policy: no code control, passkey button visible and enabled only where browser credential APIs permit, but the button remains inside a visible `<form>`. Installed Classic 6/6 hid its TOTP `#login-form`. | Native presentation gap under the frozen hidden-form wording; no WebAuthn ceremony or full parity. |
| `005` | Installed Piclaw 3.2.4 standalone login bundle with disposable WebAuthn/HTTP stub passed 6/6 browser viewports: an ambient conditional request began, explicit sign-in aborted it before a required request, and TOTP submission aborted the required request before `/auth/verify`. Mounted Gi has an AbortController and explicit cancel for prompted work but starts no ambient conditional request. Copied Classic `login.ts` is unmounted. | Verified native ambient-preemption capability gap. Synthetic signal ordering does not establish a real WebAuthn ceremony or physical prompt. |
| `006`–`007` | Installed Piclaw 3.2.4 Classic UI with disposable status/model catalogue passed 18/18 browser viewport/mode cases: empty model shows Getting started, opens Settings, and dismisses with a localStorage flag; available options and a current-model hint hide the panel. The copied `ui/oobe-state.ts` helper passed 4/4, but its provider-ready branch differs from the installed resolver and `OobePanel` setup action points to `/login` instead of Open settings. A separate untagged mounted Gi check with empty runtime/session-model responses passed 6/6: the composer appears, with no OOBE panel or Open settings action. | Native mounted capability gap. Installed `006` and two `007` rows have bounded fixture evidence; dismissal persistence after reload, unresolved readiness and popout are not browser-tested. Gi `006`–`007` remain unmapped; no real provider setup or physical input. |
| `008`–`012` | No native Gi invitation page/claim/confirmation endpoint or `recovery_only` flow was found under `web/src` or `internal/web`. Single-user owner setup/passkey Settings are different workflows. | Verified native invitation capability gap for TOTP, recovery-only and passkey invitation, including one-use cancellation cases. No tagged journey. |
| `013` | Family mode is rejected by Gi policy. No mounted family client with blur/hidden masking and identity rechecks. | Verified native family privacy gap; ordinary notification `visibilitychange` is not family masking. |
| `014` | Settings' native sign-out confirms intent, disables while busy, attempts notification cleanup, POSTs logout, checks auth status, and reports uncertainty on failure; `gi-auth.ts` re-reads policy after sign-out. | Partial single-user analogy only. No family-client sign-out/mask state, no tagged frozen journey, and no claim of navigation to Piclaw `/login`. |

`bun test web/src/ui/oobe-state.test.ts` passed **4/4** source-helper
assertions. Focused `@ux-auth-002|004` plus the untagged `003` gap check passed **18/18**;
the installed OOBE probe passed **18/18** and the mounted Gi OOBE gap check
passed **6/6**. These results do not establish whole-clause acceptance. The broad auth suite was interrupted
at 105/120 in the earlier review, not a green gate. No production code,
frozen contract, or live instance was changed here.
