# Profiling gi

Set `GI_PPROF=127.0.0.1:6060` to serve Go's `net/http/pprof` from a running gi (TUI or `-web`). The server listens on its own address, never on the web UI's handler. It also enables mutex and block profiling.

```sh
GI_PPROF=127.0.0.1:6060 bin/gi
go tool pprof -top -cum bin/gi "http://127.0.0.1:6060/debug/pprof/profile?seconds=20"   # CPU
go tool pprof -top bin/gi http://127.0.0.1:6060/debug/pprof/allocs                       # allocations
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head                     # goroutines
```

## Idle TUI CPU (2026-10-02)

An idle TUI on a long real session used about 21% CPU. Each forced frame lays out and re-wraps the whole transcript, and frames were being forced while nothing changed:

- **Cursor blink:** a 500 ms timer redrew the whole UI, although the editor's cursor is steady (as in Pi) and never used the blink state. The timer is removed.
- **1 s timer:** it re-rendered unconditionally. It now re-renders only during live activity (a running turn, compaction, the activity indicator) or when the footer data changed.
- **Block markers:** parsed on every render and every 80 ms check (base64 + JSON). Parsed markers are now memoized, and lines are filtered by prefix first.

After the change, the same session idles at about 3%. What remains is go-tui's 60 fps polling loop and gi's 80/120 ms timers.

Rule: a timer must call `MarkDirty` only when its state changed, because a frame costs a full transcript layout.

## Transcript windowing (#34)

A fullscreen frame lays out only the transcript blocks that intersect the viewport, plus 8 rows of margin (`internal/tui/transcript_window.go`). Blocks off screen become fixed-height spacers, so the scroll container's content height, the scroll offsets and stick-to-bottom behave as if every block were laid out.

- **Heights:** cached by a hash of the block's content, its spacing context, the width and the theme. Running blocks are not cached, because their spinner and elapsed time change.
- **Equivalence:** `TestTranscriptWindowMatchesFullLayout` checks that windowed and full layouts render identical rows at several offsets and at the bottom.
- **Cost** (`make bench-tui-transcript-frame`, 600 blocks, 100×40 window): one frame drops from about 94 ms and 81 MB allocated to about 3.9 ms and 3.3 MB.
- **Redraw rate:** the 80 ms activity redraw is Pi's spinner cadence (pi-tui `Loader`, 80 ms). It is kept, now that a frame costs what the screen shows.

Selection and search modes still render from their own flattened rows.
