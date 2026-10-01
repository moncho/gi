# Terminal theme detection (issue #12)

## Current behavior

Before go-tui owns input, Gi queries the controlling terminal for OSC 10/11
foreground/background and CSI `?996n` colour scheme, followed by DA1. The
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

The Pi `theme` setting accepts `dark`, `light`, or a `light/dark` pair.
Whitespace is trimmed. An unset, invalid, or unavailable name falls back to
the detected scheme's built-in theme. `/settings` reports effective and
configured theme values separately. Detection is startup-only.

The built-in palettes are RGB goldens generated from Pi 0.99.2's own theme
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

Twelve scenarios exercise light/dark OSC replies, fragmented replies,
scheme-only detection, fallback precedence, silence, malformed replies,
explicit selection, auto pairs, and query disabling. Light/dark scenarios
also start the native fullscreen UI, inspect `/settings`, and check emitted
text RGB values. This is protocol/rendering acceptance with simulated terminal
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
- Optional Pi `system` theme derivation from OSC 4 palette and terminal colours.
- Windows console probing and preservation of unrelated startup input.

This slice establishes reliable built-in light/dark startup, not complete Pi
custom-theme or system-palette parity.

Final verification for this slice: full `make test`, `make vet`, all 12
`make test-tui-theme-pty` scenarios, `make build-web`, and `make bun-checks`
passed. Build verification reused existing dependencies; no packages were
installed and no npm command was run. Generated web output was restored because
this change is TUI-only. Browser suites were skipped under the repository's
ChromeOS-host guidance. No deployment or issue closure is claimed.
