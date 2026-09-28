# Historical feature snapshots

`classic-snapshot/**/*.gherkin` contains the 24 byte-for-byte Piclaw Classic
feature files from commit `70d33bc93ab540845bbcf5f80503ca8125c71594`.
`shared-canonical-ux.gherkin` is the original Tau/Vibes shared contract, with
SHA-256 `a08a623880c6f327bc051edc51bb2bbff2959aed86421b5227e61d5a92fc2441`.
`SHA256SUMS` and `PROVENANCE.txt` identify the original sources; the former's
paths refer to `tests/e2e/features/classic/` in the source checkout. The
snapshots use `.gherkin` so normal active-feature discovery does not count them.
`tests/ux/support/catalogue.mjs` checks these bytes on every inventory run.

The editable active Piclaw 3.2.4 contracts remain under
`features/ux/classic/` and `features/ux/shared-canonical-ux.feature`.
Their tags and clauses can be corrected when pinned release evidence conflicts
with the historical snapshots. The `oracle/` fixtures retain narrower evidence
and are not Gi parity cases. See
`docs/internal/ux-reaudit-2026-09-26/gherkin-alignment.md` for the change ledger.
