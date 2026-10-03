# go-tui runtime snapshot for Gi

Source: `github.com/grindlemire/go-tui v0.22.1`, tag commit
`cadf283dd4d5b8e6d4f2d9031915ce90c1c86e47`. Upstream LICENSE is retained.

This snapshot contains root runtime Go files/tests and their internal dependencies
(`debug`, `highlight`, `layout`, `markdown`, `tailwind`). It deliberately omits
upstream CLI, code generator, LSP, examples and website. Those are not used by Gi.
Runtime files include the patches and extensions listed below. `gi-row-redraw.patch`
records the original row-redraw change; `gi-unicode-perf.patch` records the Unicode
performance delta against Gi main `78a35073` (which already has #34 caching).
Module metadata is unchanged.

The patch adds opt-in `WithRowRedraw`, `RenderRows` and `Buffer.RowDiff`, to
repaint entire changed rows on the existing synchronized output path. Default
cell diffs and inline rendering are unchanged. Gi opts in for fullscreen output,
including switches from regular mode. The dependency has no output interception
API, so a local `replace` is needed; no module-cache edits or reflection are used.

Maintenance: rebase the patch onto a newer runtime snapshot and run
`make test-go-tui-runtime`, Gi's full suite and terminal regressions. Remove the
snapshot and `go.mod` replacement once upstream provides equivalent row redraws.
The patch is suitable for upstream review; it has not been submitted upstream.
- `WithPreFlushHook(func(*Buffer))`: called after the element tree and overlays render and before the frame is flushed, so gi can composite Pi's "Jump to latest message" indicator over the laid-out frame.
- Performance (gi #34): per-element wrap and cluster caches (`text_wrap_cache.go`), ASCII fast paths in `RuneWidth` and `stringWidth`, direct narrow `Fill`, and a `Run` loop that blocks while nothing is dirty (`MarkDirty` wakes it).

- Complex-table performance: cache intrinsic *content* width, reuse unwrapped
  rich-text line clusters, reject narrow-code-point lookahead cheaply, and binary
  search the sorted wide-range tables. Width setters, padding and borders stay
  live; text setters/options reset the content cache. Cluster caches are
  per-element, single-wrap-width and base-style keyed (not a global string map).
  Text/span content is treated as immutable between `SetText`/`SetRichText` calls,
  as required by the existing wrap caches. See `gi-unicode-perf.patch`,
  `gi_unicode_perf_test.go` and `docs/internal/tui-complex-table-performance.md`
  in the parent Gi repository. The frozen-reference tests cover all code points
  and randomized valid/malformed text. Preserve the existing terminal-width
  profile; these optimizations do not implement full UAX #29.
- Bracketed paste: the app enables mode 2004 with input reporting (on start
  and resume, off on exit and suspend) for terminals implementing
  `BracketedPaster`; the reader turns `ESC[200~ … ESC[201~` into one
  `PasteEvent` (held until its end marker arrives, across reads), and
  `App.SetPasteHandler` receives it. Without a handler the paste replays as
  keystrokes, as before. Tests: `paste_test.go`.
