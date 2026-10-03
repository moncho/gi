# TUI settings

## /settings

`/settings` opens Pi's settings selector, `SettingsSelectorComponent`, built
on pi-tui's `SettingsList`. The code is in `internal/tui/settings_menu.go`.
Pi's golden is `scripts/golden-settings.mjs`, checked by
`TestSettingsMenuMatchesPi`.

The selector lists the Pi settings that gi implements, with Pi's labels,
descriptions, values and order:

- Auto-compact
- Hide thinking
- Quiet startup
- TUI mode
- Fullscreen wheel scrolling

Keys:

- Enter or Space moves the selected setting to its next value. Space does
  this only while the search is empty.
- Typing searches the labels with Pi's fuzzy filter.
- Escape or Ctrl+C closes the selector.

Each change applies at once and is saved to the project's settings. Pi saves
them to the global settings. Differences from Pi:

- **Auto-compact** updates the running engine and is saved as
  `compaction.enabled`.
- **TUI mode** is saved and takes effect the next time gi starts. Pi switches
  modes live.

Pi settings that gi does not implement are not listed, because they would
have no effect. These include images, transport, cache warming, Mermaid,
telemetry, project trust, padding and others. The theme submenu comes with the
custom themes work (#12).

## /config

`/config` is a read-only, live runtime summary.
They do not reread the settings file, constitute a complete serialized config,
or claim that every Pi setting is supported. Disk changes may require reload
or restart; model and thinking selections reflect the TUI's current session.

The summary covers runtime/tool state, selected model, editor settings, session,
discovery, compaction, provider retry and peering. It intentionally excludes
system-prompt content, credential values and full routing/identity documents.

## Display rules

- Workspace paths are emitted in full even on narrow terminals; visual wrapping
  must not discard information.
- `thinking` uses the same effective-level calculation as the editor/footer;
  `thinking_configured` reports the selected raw level. Known non-reasoning
  models are identified explicitly, and an absent selection defaults to medium.
- `theme` is the applied terminal palette; `theme_configured` retains the theme
  name or light/dark pair. An absent setting is labelled `(auto)`.
- `fullscreen_wheel_scroll_lines` reports the fixed count or `auto`.
- An unset clipboard mode describes both defaults: selection uses OSC 52 while
  plain `/copy` is transcript-only. Explicit modes remain authoritative.
- Compaction includes context window, threshold, recent-token and reserve
  budgets, plus strategy. Each budget has its own line.
- Provider retry uses `Retry.Policy()`, including defaults and safety clamps,
  rather than printing optional pointer fields or unbounded input values.
- Peering reports credential references (environment/keychain names), never
  their values. Individual fields have separate labels.

This output remains ordinary transcript content and is subject to the configured
scrollback limit. It is not a settings editor or a configuration provenance view.

## Coverage

Command-level Go tests exercise both aliases, full paths, effective defaults,
explicit editor settings, normalized retry overrides and credential-value
omission. The settings/approvals Gherkin scenario checks terminal navigation.

Validation: the full `internal/tui` suite, `make vet`, and the isolated
settings/approvals terminal Gherkin scenario passed. The latter also built the
web assets and main binary. No running instance was restarted.
