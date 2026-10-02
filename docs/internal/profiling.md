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

## Frame cost (#34)

These are measured with `make bench-tui-transcript-frame`: a 600-block session in a 100×40 window, one fullscreen frame of transcript layout and render into a reused buffer.

| Step | Time per frame | Allocations |
|---|---|---|
| All blocks laid out (before) | ~94 ms | 81 MB |
| Viewport window (spacers for blocks off screen) | ~3.9 ms | 3.3 MB |
| + go-tui wrap cache per element (layout, height and draw shared one wrap) | ~2.1 ms | 1.6 MB |
| + rendered block elements reused across frames (keyed by content hash) | ~1.8 ms | 1.3 MB |
| + block list and keys memoized while transcript lines are unchanged | ~0.86 ms | 0.49 MB |
| + ASCII fast paths (`RuneWidth`, `stringWidth`), direct narrow `Fill` | ~0.54 ms | 0.49 MB |
| + cached cluster segmentation per wrapped line; blocks indexed, not copied | **~0.29 ms** | **41 KB** |

Details:

- **Windowing** (`internal/tui/transcript_window.go`): only blocks that intersect the viewport, plus 8 rows of margin, are laid out. Spacers keep the content height, scroll offsets and stick-to-bottom identical. `TestTranscriptWindowMatchesFullLayout` checks that the rows match a full layout.
- **Block cache:** rendered elements, their click targets and heights are kept per hash of the block's content, its spacing context, the width and the theme. Running blocks are never cached.
- **Block memo:** transcript lines are immutable strings, so pointer equality detects changes without hashing text.
- **go-tui** (`third_party/go-tui`): `textWrapCache` (`text_wrap_cache.go`) keeps an element's last wrap and its clusters, reset when the text changes. Callers must not modify the returned lines.

## Idle wake-ups

- **go-tui's `Run` loop** blocks while nothing is dirty and no events are queued. `MarkDirty` signals a wake channel, so a frame is never missed.
- **gi's 80 ms, 80 ms and 120 ms timers** are merged into one 80 ms tick (Pi's spinner cadence). The draft save runs every second tick.
- **The 1 s check** reads the session for the footer only after a topic event invalidated it, or every 5 s as a fallback.
- **The running-block answer** is reused while transcript lines are unchanged.

On the long real session, idle CPU went from about 21% to about 1%.
