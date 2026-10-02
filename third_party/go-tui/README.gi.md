# go-tui runtime snapshot for Gi

Source: `github.com/grindlemire/go-tui v0.22.1`, tag commit
`cadf283dd4d5b8e6d4f2d9031915ce90c1c86e47`. Upstream LICENSE is retained.

This snapshot contains root runtime Go files/tests and their internal dependencies
(`debug`, `highlight`, `layout`, `markdown`, `tailwind`). It deliberately omits
upstream CLI, code generator, LSP, examples and website. Those are not used by Gi.
No generated or upstream file was changed except the five files captured in
`gi-row-redraw.patch` (three modified, two new). Module metadata is unchanged.

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
