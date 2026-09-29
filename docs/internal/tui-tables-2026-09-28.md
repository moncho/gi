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

## Unicode table scroll-redraw repair

`make test-tui-table-scroll` runs an isolated tmux screen-capture test with no
provider, SQLite CLI, or Bun dependency. It renders two Unicode tables separated
by prose, scrolls through every offset in both directions, and compares real
terminal captures with the intended go-tui buffer. It explicitly requires an
overflowing viewport; a static first-frame comparison is not scrolling evidence.
Widths are 38, 60, 100 and 140 columns at 24 rows. The fixture includes variation
selectors, CJK, Arabic, combining accents, ZWJ emoji and regional-indicator flags.
Each subtest owns a disposable tmux server; existing terminals are untouched.

### Cause and fix

The preformatted rich-text path replaced ASCII spaces with NBSP to prevent
another word-wrap pass from collapsing table padding. On tmux 3.5a with go-tui
v0.18.2, a regional-indicator flag following NBSP advances the terminal cursor
less than the renderer expects. A direct terminal probe distinguished this from
column allocation: `Flags 🇵🇹abc` advances 11 columns, but replacing its space
with NBSP advances only 10. Static captures can look correct because they do not
expose the shortened row; incremental scrolling leaves stale separators or text.
The regression failed at all four widths before the fix.

Preformatted rows now use `WithWrap(false)` and retain literal ASCII padding,
including padding inside styled inline code. The Markdown renderer already
wraps table cells to the allocated widths; a second word-wrap pass is redundant.
Leading indentation remains element padding, and ordinary prose/inline-code
wrapping is unchanged. No terminal-wide width override, destructive screen clear,
ASCII-only emoji substitution, or dependency patch is needed.

### Verification and scope

- `make test-tui-table-scroll`: passes 516 frame comparisons (171/131/107/107),
  including tables entering and leaving the viewport in both directions.
- Ordinary `make test` includes `TestMarkdownTablePreservesASCIIPadding`, checking
  literal spaces, inline-code styling boundaries and indentation without NBSP
  normalization in the assertion. Full Go tests pass.
- `make vet`, `make build-web`, `make bun-checks`, and the binary build pass.
- `make test-ux TEST_PORT=19137` starts and cleans up a fresh isolated instance:
  36 pass, 3 skip, 116 fail because Playwright browser executables are missing.
  Browser verification is therefore incomplete, not claimed as passing.

This repairs the reproduced NBSP/flag table redraw case, not every terminal's
emoji/font-width policy. It does not establish that this explains every glyph
in the original report. Native scroll coverage is opt-in (ordinary Go tests skip
the tmux harness). Verification uses the isolated branch's pinned go-tui v0.18.2. The concurrent
worktree upgrade to v0.22.1 also changes rendering APIs elsewhere; substituting
its module files alone fails compilation, so that integration is not verified.
The new test uses the shared `Calculate`/`RenderTree` APIs. No running TUI was
replaced or service restarted.
