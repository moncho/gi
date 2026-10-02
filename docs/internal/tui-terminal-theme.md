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

The Pi `theme` setting accepts `system`, `dark`, `light`, or a `light/dark`
pair. Whitespace is trimmed. As in Pi 1.0, an unset, invalid, or unavailable
name selects the default `system` theme. `/settings` reports effective and
configured theme values separately. Detection is startup-only.

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

The built-in palettes are RGB goldens generated from Pi 1.0.0's own theme
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

## Remaining scope — do not close #12 yet

- Pi custom theme JSON/resource loading, variable references and runtime
  OKHSL/OKLCH conversion. Unknown custom names currently fall back as above.
- Live `/theme` switching and scheme-change notifications.
- Windows console probing and preservation of unrelated startup input.

Built-in and system themes are in place; custom theme files are not.

Final verification for this slice: full `make test`, `make vet`, all 12
`make test-tui-theme-pty` scenarios, `make build-web`, and `make bun-checks`
passed. Build verification reused existing dependencies; no packages were
installed and no npm command was run. Generated web output was restored because
this change is TUI-only. Browser suites were skipped under the repository's
ChromeOS-host guidance. No deployment or issue closure is claimed.
