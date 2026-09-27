# Native Markdown/ANSI TUI acceptance

Run `make test-tui-markdown` (also included in `make test-tui-gherkin`). Requires `tmux`, `sqlite3`, Go and Bun; no model credentials or network are needed.

`rendering.feature` is parsed and expanded with `@cucumber/gherkin`. Each of the 18 pickles boots a disposable gi TUI in a private tmux socket at the specified mode and terminal size, seeds the **real SQLite store** with a user and assistant message, and reopens the session. `scripts/test-tui-markdown.mjs` implements the feature steps against `capture-pane -p` and `capture-pane -p -e`. Unknown/removed steps fail instead of being silently skipped.

Assertions cover headings, emphasis, lists, quotes, tables, Unicode cells, link text and OSC 8 closure, inline-code SGR highlighting/reset, fenced-code indentation (including source indentation), user background/assistant reset, resize, draft retention, and no extra stored messages. Captures (`.txt` and `.ansi`), a summary, and failure logs are written to `test-results/tui-markdown/`. This tests stored-message rendering; streaming and long/wrapped code need separate coverage.
