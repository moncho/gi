# Gi browser verification

Classic compliance uses the shared suite at `references/fixtures-vibes`, pinned to **`0874ea2`** (the coordinator-authorised post-v0.1.0 patch). Gi does not maintain a second Classic scenario catalogue or parity report.

```sh
make fixtures-vibes       # Chromium/WebKit × phone/tablet/desktop, zero retries
make test-ux-parity       # compatibility alias for fixtures-vibes
make check                # Go, vet, web build, hook checks, adapters, functional tests
make test-web-adapters    # runtime/adapter unit tests and frozen provenance
make test-web-regression  # Gi-only browser race/recovery regressions
```

## Shared compliance

`tests/fixtures-vibes/profile.json` declares lifecycle commands, capabilities, selectors and tool names. `skips.json` lists absent capabilities and issue-linked defects per scenario. A listed skip does not prevent execution, and a passing listed scenario fails the stale-skip gate.

Results are written under `references/fixtures-vibes/test-results/`:

- `compliance-report-gi.md` and `.json`: per-scenario status and gate;
- `evidence-gi.json`: passing test titles by scenario and project;
- Playwright results and retained failure artifacts.

The shared suite replaces the owner-side `tests/ux/*.spec.mjs`, `support/catalogue.mjs` and `scripts/ux-parity-report.mjs`. Legacy `test-shared-*-evidence` targets also use this gate. Scenarios reported as `no-suite-test` have no shared browser evidence.

## Gi regressions

Non-duplicated Gi tests live in `tests/web-regression/`, using `playwright.web-regression.config.mjs`. They cover native configuration, authentication, receipts, stale responses, draft recovery, tool timing, IndexedDB failures and adapter boundaries. Local Classic cases with no shared spec yet are also kept here until the coordinator ports them into a tag and that replacement passes. Their results do not award Classic scenario credit. Specialised `test-ux-*` targets run these isolated regressions; targets whose owner spec was retired delegate to shared compliance.

`tests/ux/server/` remains the deterministic Go runtime used by these regressions and some functional tests. `tests/ux/support/` retains adapter tests and Gi feature/criterion checks. The opt-in real-provider functional and TUI acceptance probes remain separate.

Frozen `features/ux/upstream/` and `web/upstream/` files remain read-only historical provenance. `support/provenance.mjs` verifies their hashes without loading an active scenario catalogue. The Piclaw 3.2.4 SVG asset pin is unchanged. Comparing those assets to a historical release requires `PICLAW_324_STATIC_ROOT`; the installed 3.2.5 release is not a substitute. The asset-integrity tests run unconditionally.

## Scope

Gi #35 tracks cascade deletion. Gi #37 tracks the messages tool's missing all-chat text-search contract. The profile keeps both user-visible capabilities claimed and lists affected scenarios explicitly.

Compaction is claimed. Scenarios 001–004 have an agreed `environment-limit` listing: Gi summarises natively without a model call, so the fixture model cannot hold its compaction in progress. Scenarios 006–008 run. The disposable fixture build registers both models with the shared 128K context contract and is built once before workers start. Settings parity, family features and iPad annotation are outside this task.
