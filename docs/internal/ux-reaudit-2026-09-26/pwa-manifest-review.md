# Classic PWA icon clauses 001–006: audit disposition

The shipped Piclaw 3.2.4 oracle is
`/opt/piclaw/current/app/runtime/src/channels/web/manifest.ts` and
`http/dispatch-shell.ts`. These server files are readable in the installed
release; they are not part of the Classic UI source-map diff. No Piclaw avatar
mutation or home-screen installation was run. Gi tests use a disposable native
web server, not the live chat/auth database.

| Frozen ID | Current Piclaw source and Gi evidence | Disposition |
|---|---|---|
| `@ux-pwa-001` | Piclaw's default manifest has static 192/512 PNG icons (`manifest.ts`). Gi `tests/ux/pwa.spec.mjs:4–36` gets `/manifest.json`, checks name, icons, sizes, type/purpose, PNG bytes, `HEAD` and the HTML manifest link. Gi `internal/web/server.go:1127–1166` serves the static manifest. | **Bounded default-static pass.** The fixture has no configured avatar; it does not establish the avatar branch. Six Gi browser projects passed. |
| `@ux-pwa-002` | Piclaw `manifest.ts` builds 192/512 PNG `/avatar/agent` URLs with an avatar version after successful cache preparation. Gi `serveManifest` hard-codes `/static/icon-192.png` and `/static/icon-512.png`; there is no agent-avatar manifest branch. | **Gi capability gap**, untagged and not implemented. Do not borrow `001`. |
| `@ux-pwa-003` | Piclaw falls back to static icons when avatar/cache metadata is absent. Gi's static-only manifest and `001` native test show the no-avatar branch. | **Bounded default-static evidence.** The test does not toggle an avatar configuration from present to absent. |
| `@ux-pwa-004` | Piclaw `dispatch-shell.ts` requests PNG avatar output for Apple 180/167/152/unsized (180) paths; only a successful PNG response is returned, otherwise a matching static asset. Gi's `internal/web/static/` has four static routes. An untagged `tests/ux/pwa.spec.mjs` test checks all four 200 PNG responses/signatures. | **Avatar/fallback-decision gap:** static routes pass, but no Gi avatar-first handler, success/error branches or per-size request evidence. Four outline examples remain unaccepted as written. |
| `@ux-pwa-005` | Piclaw's favicon path requests a size-48 PNG avatar and falls back to `/favicon.ico` if unsuccessful. The untagged Gi browser test checks only a nonempty static favicon response. | **Avatar/fallback-decision gap.** The frozen note explicitly does not require a PNG-decoded fallback. |
| `@ux-pwa-006` | Piclaw version-stamps avatar icon URLs from cache metadata. Gi's static manifest URLs have no avatar version. | **Gi capability gap**, untagged and not implemented. |

`make test-ux-parity UX_PARITY_PORT=19134
UX_PARITY_ARGS='tests/ux/pwa.spec.mjs'` passed **12/12** across Chromium/WebKit
phone/tablet/desktop: six tagged `001`, six untagged static Apple/favicon route
checks. The test and source trace qualify `001` and the static half of `003`.
They give no Piclaw avatar-branch, Gi dynamic-avatar, physical home-screen or
cache invalidation acceptance. A native avatar implementation and adversarial
PNG/non-PNG/fallback tests would be needed before mapping `002`, `004`, `005`
or `006` to current Piclaw behaviour.
