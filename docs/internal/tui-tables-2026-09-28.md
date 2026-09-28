# TUI Markdown table rendering and streaming

The report reproduced on main `5ef8dd8`. A two-column table containing inline code switched to a stacked label/value list at widths 30, 56 and 96. Hidden code-style delimiters were counted as visible characters. At wider widths, rune-count padding misaligned CJK, emoji and combining characters. The old grid also had no top/bottom borders or wrapped-cell layout.

A separate resize defect retained precomputed column widths in the transcript. Narrowing the terminal wrapped those old borders rather than reallocating the table. Regular mode exposed only the last three lines while streaming, often hiding the header and most rows.

## Changes

- `internal/tui/markdown_table.go` follows installed Pi0.87.1's natural/minimum column allocation, uses terminal display widths, and wraps cell contents without splitting grapheme clusters. Private inline-code delimiters do not affect sizing and remain balanced on each wrapped line.
- Tables use a box-drawn grid, including wrapped cells and row separators, rather than switching to a key/value list whenever natural widths exceed the viewport. At widths too small for the grid itself, cells remain wrapped text. A glyph wider than a one-column cell uses a replacement character rather than breaking its byte sequence.
- Table-bearing user/assistant messages retain source Markdown in invisible in-memory transcript metadata. Rendering reallocates columns for the current width, message padding and scrollbar. Persisted messages and `/copy` source remain unchanged.
- Table borders and cell padding use the existing preformatted rich-text path, preserving spaces across styled fragments.
- Regular mode gives streaming tables a bounded larger live region (up to half the terminal height). Completed output is still printed once into terminal-owned scrollback; already-baked scrollback is not redrawn after resize.
- Search renders source-backed tables at the search viewport width. Ordinary non-table rendering remains unchanged.

## Verification

`make test-pi-table-oracle` invokes the installed Pi Markdown renderer. The Go regression compares table text/geometry at widths 30, 56, 96 and 136, excluding ANSI styling and trailing viewport padding outside the grid. No generated-output hashes are used.

`make test-tui-tables` runs race×3 table tests and six disposable native TUI sessions: regular/fullscreen at 60×24, 100×28 and 140×36. A local SSE provider sends header, delimiter, split inline-code cells and Unicode rows in separately gated chunks. Captures cover partial output, active resize to 38 columns and back, finalisation and reload. Tests preserve the unsent draft and assert raw stored Markdown is unchanged. No external provider is called.

Existing regression results:

- Markdown PTYs: 24/24 pass.
- Inline prose PTYs: 6/6 pass.
- Assistant source-copy PTYs: 6/6 pass.
- Scrollbar PTYs: 3/3 pass.
- Table resize/search unit coverage passes with race×3.
- Full `make check` passes Go/vet/build/hooks and 144 functional tests; 11 skipped.

The broader `test-tui-search` PTY fails at `two cross-soft-wrap occurrences`. A clean archived build of pre-fix `5ef8dd8` fails at the same assertion; both logs are retained. This is not presented as passing or fixed. Targeted table search passes. Review delegates timed out; no independent review approval is claimed.

## Limits and evidence

This fixes the reproduced table layout/streaming path, not all Pi terminal styling or full terminal parity. Quote/list nesting and extreme one-column Unicode tables remain outside the PTY fixture. The exact executable behind the user's observed TUI is not identified. No running executable was replaced and no production TUI was interrupted.

Logs: `/workspace/tmp/gi-tables-*.log`. Installed oracle: `test-results/tui-tables/pi-oracle.json`. Latest native captures and `results.json` are under `test-results/tui-tables/run-*/`. The clean old-build search control is under `/workspace/tmp/gi-table-head-baseline/`.
