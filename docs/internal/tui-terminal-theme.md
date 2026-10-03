# Terminal theme detection (issues #12, #31)

## Current behavior

Before go-tui owns input, Gi queries the controlling terminal for OSC 10/11
foreground/background, OSC 4 palette colours 0–15 (Pi's
`TERMINAL_COLOR_QUERY`) and CSI `?996n` colour scheme, followed by DA1. A
palette is used only when all 16 colours were reported. The
reported background and foreground determine light/dark using Pi's contrast
and OKLab-lightness rules. Fallback order is the scheme reply, `COLORFGBG`,
then dark. Background indices 0–6 and 8 are dark; 7 and 9–15 are light.
Malformed colour replies and invalid indices are ignored.

On Unix the terminal is opened nonblocking and polled within one 150 ms
budget. Interrupted reads do not restart the budget; reply buffering is capped
at 8 KiB. Raw-mode state is restored on every return. A DA1 reply ends the
query early. Windows currently uses environment fallback without probing.
`GI_TUI_NO_THEME_QUERY=1` bypasses probing for tests or incompatible terminals.
The startup probe consumes incoming bytes, so type after the editor appears;
this is not a general-purpose input/reply multiplexer.

The Pi `theme` setting accepts `system`, `dark`, `light`, a custom theme
name, or a `light/dark` pair of any of them. Whitespace is trimmed. As in Pi,
an unset or invalid setting, or a theme that does not load, selects the
default `system` theme (silently at startup). `/config` reports effective and
configured theme values separately. Detection is startup-only.

## Custom themes

`custom_theme.go` ports Pi's custom themes (`theme.js`, `theme-json.js`):
JSON files in `themes/` of the user config dirs (gi's, then Pi's; the first
theme of a name wins), listed by their `name` and loaded by file name.

- Validation is Pi's `validateThemeJson` for the shapes gi checks: Pi's
  message for missing colour tokens, colour values that are neither strings
  nor integers 0-255, and names with "/". Invalid files are left out of the
  list.
- Colours are resolved as Pi's `createTheme`: Pi's fallbacks for optional
  tokens, `vars` references (chained; circular and unknown ones are Pi's
  errors), then pi-tui's `parseColor`: `#rgb`/`#rrggbb`, `okhsl(...)`,
  `oklch(...)`, 256-colour indexes (emitted as indexes) and `""` for the
  terminal's default.
- While a custom theme is active its file is watched (`theme_watcher.go`,
  Pi's `startThemeWatcher`): edits apply 100 ms after the last change, and a
  missing or invalid file keeps the last good theme.

Golden: `scripts/golden-custom-theme.mjs` renders every token of a custom
theme with Pi's own theme code and records Pi's errors for invalid files;
`TestCustomThemesMatchPi`, `TestActiveCustomThemeReloads`.

## Switching themes

`/settings` has Pi's Theme item: Pi's `ThemeSubmenu` with one theme or
automatic mode (a light and a dark theme). Moving through themes previews
them live; Enter saves the setting (Pi's `theme` in the project settings) and
applies it; Escape restores the theme in use. A theme that fails to load
reports Pi's "Failed to load theme" error and falls back to the system theme.
Golden: the theme scenarios of `scripts/golden-settings.mjs`.

## System theme (#31)

`system_theme.go` ports Pi's `generateSystemThemeColors`
(`modes/interactive/theme/system-theme.js`) and `oklab.go` the OKLab, OKHSL
and OKLCH conversions of pi-tui (`oklab.js`, `colors.js`). Every token
belongs to a colour family and must reach a contrast level on the surfaces it
is drawn on; hue and saturation come from the palette (or the family), the
lightness from the levels. Palette colours never gain OKLCH chroma at another
lightness, so pastel palettes (Catppuccin Frappe) stay pastel. Mid-grey
backgrounds relax the levels as little as needed. Body text (`text`,
`userMessageText`, `toolTitle`) uses the terminal's own foreground when it is
clearly stronger than muted text, which gi draws as the terminal default
colour.

`TestSystemThemeMatchesPi` compares every token for eight terminals
(Catppuccin Frappe and Latte palettes, xterm, background-only, light,
mid-grey with and without a palette, a dim foreground) with colours from Pi's
own code (`scripts/golden-system-theme.mjs`).

Differences from Pi:

- Without a reported background Pi renders ANSI palette indices and makes
  neutral tokens faint (SGR 2). gi's palette is colours only, so it uses the
  detected scheme's built-in `dark` or `light` theme there; `/settings` names
  that theme.
- Pi renders grayscale while its query is in flight; gi queries before the UI
  starts, so there is no pending phase.
- Text selection, which gi draws with the text colour as background, uses
  inverse video when the text colour is the terminal default.

The built-in palettes are RGB goldens generated from Pi's own theme
conversion (`pi_themes_gen.go`); Gi retains Pi's truecolor detection and
256-colour quantization. Markdown, tool output/title, diff, user text, and
custom-message text/label roles are independent even when built-in colours
coincide. Existing syntax and thinking roles switch with the palette too.
There is no npm dependency or installation step for this change.

## Verification

`make test-tui-theme-pty` compiles an isolated Go test fixture and runs a Bun
harness using util-linux `script` to obtain a real controlling PTY. No tmux,
external provider, credentials, browser, Python harness, or JS packages are
required. Artifacts live under `test-results/tui-theme/`.

Thirteen scenarios exercise light/dark OSC replies, a full OSC 4 palette,
fragmented replies, scheme-only detection, fallback precedence, silence,
malformed replies, explicit selection, auto pairs, and query disabling.
Scenarios that report a background resolve to `system`; the harness computes
Pi's generated colours for the reply and checks that they are emitted. An
explicit `dark` scenario checks the built-in text RGB. This is protocol/rendering acceptance with simulated terminal
replies, not visual verification in every emulator.

The harness fails with the original blocking startup reader: the silent
terminal scenario times out without reaching the editor. With bounded polling
it completes at about 150 ms. Parser and role-independence unit regressions
cover malformed OSC values, all 16 `COLORFGBG` indices, and independent
Markdown/tool/diff tokens.

## Not ported

- Pi's terminal light/dark change notifications (mode 2031) and late colour
  replies: the scheme and the system theme are fixed at startup.
- Themes registered by extensions; the HTML export colours of a theme.
- Windows console probing and preservation of unrelated startup input.
