# Derived workspace index: clause-level source findings

`features/search/workspace-index.feature` contains 23 Gi proposals derived from pinned Piclaw workspace indexing and Gi-specific safeguards. These are outside the frozen Classic/shared corpus. The existing lineage note, `docs/internal/search/indexing-lineage-20260922.md`, compares pinned source revisions and names the intentional Gi differences. The rows below identify mounted code and tests, but a test declaration or SQL fixture does not award the full scenario. No current Piclaw runtime or deployed Gi indexing acceptance was run for this review.

Gi's native `/api/workspace/index` supports explicit authenticated POST refresh and read-only GET status. `/api/workspace/search` is read-only and rejects `refresh` query parameters (`internal/web/workspace_index.go`). The scheduler, worker, scanner, scoped store and UI/TUI controls are mounted; ordinary reads do not schedule background refresh. The proposal at `:33` requires background refresh on search while `:199` and `:211` require GET and native-write paths to wait for explicit refresh. Resolve that policy before awarding either side. A stored invalidation is not an automatically processed one.

| ID | Mounted source and assertion boundary | Finding |
|---|---|---|
| 001 | `internal/search/indexer/scanner.go`, `scanner_test.go` and `store/refresh_test.go`: scoped roots, extensions, size and excluded directories. | Source-backed scanner; no full derived-scenario mapping. |
| 002 | `store/refresh.go`, `refresh_test.go`, `indexer/scanner_test.go`: stable unchanged IDs, changed/new content, stale-match removal. | Source-backed incremental refresh. |
| 003 | `store/refresh_test.go`, `schema_candidate_test.go`: complete-scan scoped membership cleanup, overlapping owners and FTS cascade. | Source-backed store transaction; scanner completion is a separate assertion. |
| 004 | `internal/web/workspace_index.go` makes GET read-only. No default search-triggered refresh; conflicts with 021/022 policy. | Policy conflict and unimplemented background trigger. |
| 005 | `workspace_index.go`, `web/workspace_index_test.go`, `indexer/scheduler_test.go`: explicit POST joins a bounded worker and returns confirmed status or failure. Query-parameter refresh is rejected, so the search-refresh half is unsupported. | Partial explicit refresh. |
| 006 | `store/refresh.go`, `store/invalidation_test.go`, `indexer/worker_test.go`: durable status, generations and invalidation; no watch-driven auto-refresh. | Partial status lifecycle. |
| 007 | `store/refresh_test.go`, `indexer/worker_test.go`, `web/workspace_index_test.go`: failed/cancelled refresh retains committed rows and status across reopen. | Source-backed failure snapshot. |
| 008 | `indexer/scanner_test.go`: traversal/read/limit errors prevent complete-scan cleanup and ready publication. | Source-backed bounded scan. |
| 009 | `store/refresh_test.go`, `indexer/worker_test.go`: fenced lease, takeover, stale owner refusal and interrupted-worker recovery. | Source-backed native lease. |
| 010 | `store/query.go`, `query_test.go`, `workspace_index.go`: lexical FTS rank/snippets, bounds, literal fallback and scope/workspace filtering. | Source-backed lexical query; no semantic search credit. |
| 011 | `store/schema_candidate_test.go`, `refresh_test.go`: stable chunk IDs, canonical FTS triggers and transaction rollback. | Source-backed storage assertions. |
| 012 | `store/schema_candidate_test.go`, `migration_test.go`: FTS rebuild/integrity and canonical row isolation. | Source-backed storage assertions, no scanner credit. |
| 013 | `store/refresh_test.go`, `migration_test.go`, `web/workspace_index_test.go`: configuration fingerprints and root/version status isolation. Vector invalidation is a separate design, without mounted vector indexing acceptance. | Partial version boundary. |
| 014 | `web/workspace_index_test.go`, `tests/ux/workspace-index-config.spec.mjs`, `internal/tui/workspace_index_test.go`: native index actions and bounded draft/session isolation. | Source-backed UI isolation within disposable fixtures. |
| 015 | `internal/tui/workspace_index.go`, `workspace_index_test.go`, `scripts/test-tui-index.mjs`: temporary index surface, Escape/draft/scroll isolation, three terminal sizes. | Source-backed native index panel; not a general TUI parity award. |
| 016 | `indexer/scanner_test.go`, `store/refresh_test.go`, `web/workspace_index_test.go`: startup policy, optional roots and fingerprint isolation. | Source-backed startup configuration. |
| 017 | `indexer/scanner_test.go`, `store/refresh_test.go`, `tests/ux/workspace-index-config.spec.mjs`: absent optional root preserves committed content; explicit retry can publish. | Source-backed optional-root guard. |
| 018 | `store/invalidation_test.go`, `indexer/worker_test.go`: captured revision and later event remain pending across commit, failures and reopen. | Source-backed durable invalidation. |
| 019 | `indexer/scheduler_test.go`: coalescing, bounded attempts, cancellation-isolated wait, peer publication and pending revision follow-up. | Source-backed explicit scheduler, without automatic request source. |
| 020 | `indexer/scheduler_test.go`, `web/workspace_index_lifecycle_test.go`: drain active/queued requests and join cleanup before store close. | Source-backed shutdown protocol. |
| 021 | `web/workspace_index_lifecycle_test.go`, `workspace_index_test.go`: application-owned explicit POST batching, disconnect isolation, cancellation and shutdown. GET stays read-only, in conflict with 004's default search trigger. | Source-backed explicit lifecycle with unresolved policy conflict. |
| 022 | `internal/tools/workspace_write_test.go`, `internal/turn/engine_test.go`, `store/invalidation_test.go`: native/HTTP/script pre/post invalidation and committed GET snapshot. Shell/external/watch delivery and crash-atomic filesystem/database mutation are not established. Explicit-only refresh conflicts with 004. | Partial write-source coverage and policy conflict. |
| 023 | `internal/tui/workspace_index.go`, `workspace_index_test.go`, `scripts/test-tui-index.mjs`: Alt-I status/Reindex, modal keys, late-result and draft/scroll isolation; three-size native acceptance is documented in ADR-0052. | Source-backed native terminal index actions; no current-run or Piclaw TUI credit. |

The index proposals' tags and existing test declarations do not change Classic/shared mapping counts. The separate `workspace-index.feature:33`, `:199` and `:211` inventory conflicts remain explicit pending a refresh-policy decision.
