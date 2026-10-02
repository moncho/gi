# TUI clipboard and media

Pending reference and admission recovery now use the [durable session journal](tui-media-journal.md). Staged refs survive process reopen; unresolved claims are held without automatic resend. `/detach unresolved` discards only the observed claim references, keeping stored files and any admitted work. parity

Status: fullscreen text selection copies via OSC 52 by default; command-driven and native clipboard writes remain opt-in; clipboard image paste is supported via `/paste-image`; `/attach <path> [prompt]` is the terminal-safe file media fallback.

## Clipboard image paste

`/paste-image [prompt]` (alias `/paste`) reads an image from the system clipboard, stores it in the session media store (`source=tui-paste`), and either reports the `media:<id>` reference or, with a prompt, submits the prompt with the image attached via the shared media ingestion contract.

The clipboard image reader is platform-aware and best-effort:

- Linux Wayland: `wl-paste --type image/png`;
- Linux X11: `xclip -selection clipboard -t image/png -o`;
- macOS: `pngpaste -`;
- Windows: PowerShell `System.Windows.Forms.Clipboard.GetImage()` to PNG.

If no helper is available, `/paste-image` returns a clear error. The reader is injectable for tests. Images larger than 10 MiB are rejected.

## Current behavior

Fullscreen mouse selection (drag, double-click word, or triple-click line) sends the selected text to the terminal using OSC 52 when `tuiClipboardMode` is absent or empty. Releasing the mouse copies; Ctrl-C/Ctrl-X repeats the copy while a selection exists. Regular mode still leaves selection to the terminal.

The status is **“Selection sent to terminal (OSC 52)”**, not “copied”: Gi can confirm dispatch, but cannot confirm that the terminal accepted the clipboard write. Terminal/tmux permissions still apply, including over SSH. The existing 64 KiB payload limit, selection invalidation and native-helper serialization remain unchanged.

Explicit settings remain authoritative: `off` disables selection copying, `osc52` selects terminal dispatch, and `native`/`auto` retain their existing helper behavior. Invalid modes remain disabled rather than enabling clipboard access. Existing persisted `off` settings are **not** migrated. To opt out or opt back in, use `/copy --off --persist` or `/copy --osc52 --persist`. These commands also copy/fall back to the latest assistant message and require one to exist before saving the preference.

Unlike the interactive selection gesture, `/copy` remains transcript-only by default: without flags, it locates the last non-empty assistant message and prints it back into the transcript with a clear `copy:` prefix. It does **not** write to the OS clipboard and does **not** emit terminal escape sequences unless the user opts in.

This preserves the tmux/script-friendly baseline and keeps transcript storage deterministic.

## `/copy` targets

Supported forms:

```text
/copy
/copy --fallback
/copy --osc52
/copy --native
/copy --auto
/copy --mode <off|osc52|native|auto> [--persist]
```

Modes:

- `off` / `--fallback`: transcript fallback only.
- `osc52` / `--osc52`: write an OSC 52 sequence directly to the terminal output path, then print a plain transcript confirmation.
- `native` / `--native`: use a detected native helper, or fall back with an error note if none is available.
- `auto` / `--auto`: try native helper first, then fall back to transcript output.

`--persist` stores the selected mode in `.pi/settings.json` as `tuiClipboardMode`. When unset, selection defaults to `osc52` and `/copy` defaults to `off`; an explicit mode applies to both. Loading configuration preserves the unset value as an empty string instead of normalizing it to `off`. `/settings` displays the two effective defaults.

## OSC 52 support

OSC 52 copies text to a terminal clipboard by writing an escape sequence like:

```text
ESC ] 52 ; c ; <base64 payload> BEL
```

OSC 52 is the default for interactive fullscreen selection and opt-in for `/copy`, with these guardrails:

- escape sequences are written to the terminal writer, not stored in transcript lines;
- transcript output contains only a plain dispatch/failure message;
- payload size is capped to avoid terminal/tmux limits;
- default `/copy` remains transcript-only.

Terminal caveats still apply:

- many terminals gate or disable OSC 52 clipboard writes;
- tmux may require passthrough configuration or its own clipboard integration;
- SSH sessions may copy to the local terminal clipboard depending on passthrough.

## Native clipboard helper support

Native helper support is opt-in and dependency-light. Gi detects helpers already present on the system rather than installing anything.

Helper order:

- macOS: `pbcopy`;
- Linux Wayland: `wl-copy`;
- Linux X11: `xclip -selection clipboard`, then `xsel --clipboard --input`;
- Windows/WSL fallback: `clip.exe`.

Native execution uses a short timeout and returns bounded transcript errors. Tests cover helper selection without requiring a desktop clipboard in CI.

## TUI media fallback

Direct terminal image paste is still not available in the current `go-tui` keyboard parser: key handling is rune/key-event oriented and does not expose clipboard image payloads or bracketed-paste metadata that would safely distinguish image data from ordinary text paste.

Gi therefore provides an explicit fallback command:

```text
/attach <path> [prompt]
```

Behavior:

- reads a local path relative to `workspace_root` when the path is not absolute;
- stores the file through the shared session media store with `source=tui`;
- enforces the same 10 MiB single-file limit as the web/API media endpoint;
- with no prompt, prints the created `media:<id>` reference without submitting a turn;
- with a prompt, submits the prompt through the same media metadata path used by web/API submissions.

Ordinary text paste remains unchanged and is still handled as terminal text input.

## Media/image ingestion boundary

Pi-style image paste is not just a keybinding. Gi still needs a media ingestion contract first:

- where pasted media is stored;
- how media is referenced from messages/turns;
- how tools and providers receive image parts;
- how TUI, web, and API submissions share the same payload shape;
- what limits and cleanup policies apply.

The shared contract is documented in [`media-ingestion-contract.md`](media-ingestion-contract.md) and its store/API primitives are implemented. `/attach` and `/paste-image` both project media through that same contract.

## Current support summary

- `/copy`: supported as transcript fallback by default.
- OSC 52 copy: default for fullscreen selection; opt-in for `/copy` via `--osc52` or persisted `tuiClipboardMode=osc52`.
- Native clipboard helpers: supported opt-in via `/copy --native` or `--auto`.
- Ordinary text paste: unchanged terminal-rune behavior.
- Bracketed paste: reassessed, deferred pending parser/editor support.
- Command-driven clipboard image paste: supported via `/paste-image [prompt]` (alias `/paste`).
- Raw Ctrl-V image payloads and inline terminal image protocols: deferred pending terminal/parser support.
- TUI file fallback: supported via `/attach <path> [prompt]` and the shared media ingestion contract.

## Selection-default regression coverage

Config tests distinguish absent/empty settings from explicit `off`, validate supported modes and keep invalid values disabled. TUI tests exercise fresh-workspace mouse-release/repeat-copy bytes, truthful dispatch status and persisted opt-out; plain `/copy` also asserts that its terminal writer receives no bytes by default. The real-PTY `make test-tui-selection` harness now starts without a clipboard setting and checks default OSC 52 dispatch plus explicit opt-out at three sizes.

Validation for this policy change: `make test`, `make vet`, `make build-web`/isolated binary build and `make bun-checks` passed. Full PTY acceptance remains unverified here: `make test-tui-selection` stopped because the `sqlite3` executable is missing. `make test-terminal-links` could not start its race tests on this ARM64 host (ThreadSanitizer reports a 39-bit VMA range; 48 required). `make test-ux` ran on a fresh isolated instance: 36 passed, 3 skipped, 116 failed to launch due to missing Playwright Chromium/WebKit executables. These environment failures are not passing acceptance evidence. No running instance was restarted.
