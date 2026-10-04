# Gi browser tests

Browser compliance uses the shared [fixtures-vibes](https://github.com/rcarmo/fixtures-vibes) suite, checked out at `references/fixtures-vibes` and pinned to **`f796ddf`** (Piclaw 3.2.5 scenarios). Gi keeps no second scenario catalogue or parity report.

```sh
make fixtures-vibes       # Chromium/WebKit × phone/tablet/desktop, zero retries, report gate
make test-ux-parity       # alias for fixtures-vibes
make check                # Go, vet, web build, hook checks, adapters, functional tests
make test-web-adapters    # web adapter unit tests
make test-web-regression  # Gi-only browser race/recovery regressions
```

## Shared compliance

`tests/fixtures-vibes/profile.json` declares lifecycle commands, the capabilities Gi claims, selectors and tool names. `tests/fixtures-vibes/skips.json` lists every scenario Gi does not pass, with a reason: `capability-absent` (with the capability), `known-defect` or `not-implemented` (with the Gi issue). Listed scenarios still run. The report gate fails on an unlisted failure, an unlisted capability skip, or a listed scenario that passes.

`make fixtures-vibes` builds `bin/gi-fixtures-vibes` with the `fixtures_vibes` build tag, so the suite's fixture model and seeded workspace are available. Results are written under `references/fixtures-vibes/test-results/`:

- `compliance-report-gi.md` and `.json`: per-scenario status and gate problems;
- `evidence-gi.json`: passing test titles by scenario and project;
- Playwright results and retained failure artifacts.

The latest results and the list of unpassed scenarios are in [the feature matrix](../../docs/feature-parity.md#shared-browser-compliance). Scenarios reported as `no-suite-test` have no shared browser evidence.

To debug one scenario without a full run, start a disposable stack with the fixture model and a `fixtures_vibes` build, then run Playwright from `references/fixtures-vibes` with `--grep "@ux-..."` and a profile that points at it.

## Gi regressions

Gi-only browser tests live in `tests/web-regression/` and use `playwright.web-regression.config.mjs`. They cover native configuration, authentication, receipts, stale responses, draft recovery, tool timing, IndexedDB failures, Settings panes and adapter boundaries. They award no shared scenario credit. The `test-ux-*` targets run them against isolated instances; several (for example `test-ux-thinking` and `test-ux-compose-surface`) start the deterministic Go server in `tests/ux/server/` with feature flags.

`tests/ux/support/` holds Bun unit tests for the web adapters and Gi feature criteria. The opt-in real-provider functional and terminal acceptance probes are separate.

Gi keeps no copy of the shared Gherkin. `features/` holds only Gi-specific, terminal and search contracts, and `support/feature-tree.test.ts` fails if a `features/ux/` copy reappears. Gi's passkey criterion ledger reads the suite's `single-user-passkey-settings.feature` directly.

Two Piclaw sources are vendored verbatim and pinned to the installed 3.2.5 release: the SVG fence renderer (`web/piclaw-svg-3.2.5/`) and the status panel (`web/piclaw-status-3.2.5/`). `support/piclaw-svg-adapter.test.ts` and `support/piclaw-status-adapter.test.ts` check the manifests; the SVG test also compares the files against the 3.2.5 source map when that release is installed (or `PICLAW_325_STATIC_ROOT` is set). The Piclaw 3.2.4 oracle probes are gone; `oracle/` keeps only Pi terminal probes and Gi diagnostics.

## Run hygiene

Run browser tests with `PI_CODING_AGENT_DIR` and `GI_CODING_AGENT_DIR` unset, so the operator's Pi settings cannot change the results. A full fixtures-vibes run takes about 75 minutes on the development host.
