# Classic auth 001, 002, 004: scope and clause review

The frozen Classic auth Gherkin includes family-shared and single-user modes.
Gi currently exposes **single-user** browser authentication only:
`internal/web/auth.go:46–72` returns `mode: "single-user"`, and
`web/src/gi-auth-policy.ts` rejects another mode. `docs/feature-parity.md:50`
also lists family mode as absent. This review does not treat a single-user
login pass as family-account authorization or Piclaw 3.2.4 browser acceptance.

| Frozen ID | Native requirement → assertion → code | Disposition |
|---|---|---|
| `@ux-auth-001` family username + TOTP | Piclaw's frozen scenario requires a visible required username, trimmed username in verification and a network-error state. Gi's native policy parser rejects family mode; the server never advertises it. No `@ux-auth-001` browser test exists. | **Verified native family-mode gap.** Do not borrow the `002` code-only login test. |
| `@ux-auth-002` single-user TOTP | `tests/ux/auth.spec.mjs:133–169` loads the real native policy, verifies code-only sign-in and no username requirement, invalid-code/network errors, disabled pending controls, authenticated cookie, session-expiry re-gate and draft retention. Gi `web/src/gi-auth.ts`, `gi-auth-policy.ts`, `internal/web/auth.go`, `auth_session.go` own the route. | **Bounded native journey.** It does not exercise a Piclaw login backend, account-family scope or physical authenticator. |
| `@ux-auth-004` failed policy load and Retry | `auth.spec.mjs:170+` fails the first `/api/auth/status`, checks credentials stay hidden and Retry appears, then returns valid single-user policy and checks the code field with no auth write. Gi `gi-auth.ts` fetches no-store, validates policy, and gates the app. | **Bounded native journey.** Other malformed policy fields and false-success response checks run in separate untagged native tests; a tag alone cannot import them. |

The full `make test-ux-auth` was interrupted after 105/120 tests and has no
green result. A focused `GI_UX_AUTH=1 GI_UX_SERVER_BIN=/tmp/gi-reaudit-auth-server
bun x playwright test --config playwright.ux.config.mjs tests/ux/auth.spec.mjs
--grep '@ux-auth-00[24]'` passed **12/12** across six Chromium/WebKit viewport
projects. Current Piclaw 3.2.4 login UI was not run in this slice; source and
frozen Gherkin remain separate from Gi native acceptance.
