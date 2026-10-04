# Shell environment

Gi runs shell commands as Piclaw 3.2.5's bash tool does (`runtime/src/tools/tracked-bash.ts`,
`environment-overrides.ts`, `secure/shell-secrets.ts`). This covers the agent's `shell` tool, the web tool runner
(`/api/tools/execute`) and the TUI's `!!` local commands (`internal/shellenv`, `internal/tools/shell_resolve.go`).

## Shell detection

- A configured `shellPath` (Pi's setting: project `.pi/settings.json`, else the user's) is used alone, as a POSIX
  shell with `-c`. A path that does not exist fails the command: `Custom shell path not found: <path>`.
- Otherwise, on POSIX hosts, the candidates are `$SHELL` when it names an existing file, `/bin/bash`, then `bash` from
  `PATH`, each run as `<shell> -c <command>`.
- On Windows they are `$SHELL` (POSIX, e.g. Git Bash), PowerShell 7, Windows PowerShell, `pwsh.exe`, `powershell.exe`,
  `%ComSpec%` and `cmd.exe`. PowerShell runs `-NoProfile -Command`, cmd `/c`.
- The first candidate that resolves to an executable runs. When none does, the error lists every shell tried:
  `No supported shell found. Tried: …`. Piclaw tries the next candidate when spawning fails with ENOENT. Gi checks the
  same thing first, with `exec.LookPath`.

The `rtk` tool and the fixture-only shell runtime still use `sh -c`.

## Environment

Each command gets the runtime's environment, then the Settings → Environment overrides, then the keychain variables it
names (see [keychain.md](keychain.md)). Keychain placeholders are resolved last. If any step fails, the command does
not run.

## Settings → Environment

- Lists the runtime's variables with their values, overrides included and keychain variables excluded.
- Adding or editing a value stores an override in the database (`kv_store`, namespace `environment`). It applies to the
  next command and survives restarts.
- Clearing an override restores the inherited value, or removes the variable when none was inherited.
- Names must be shell identifiers (`[A-Za-z_][A-Za-z0-9_]*`). Keychain variable names are refused, and overrides
  stored under such a name are ignored once a keychain entry claims it.
- API: `GET /api/settings/environment` returns `{variables, overrides, count, overrideCount}`.
  `POST {name, value}` sets an override and `POST {name, clear: true}` removes it. Writes follow the keychain's
  transport rule: HTTPS or loopback, same origin, JSON.

## Differences from Piclaw

- Piclaw writes overrides into its own process environment. Gi leaves its process environment alone and applies the
  overrides to each command, so other parts of gi do not see them.
- Piclaw's section offers search and grouping. Gi's has a filter and one row per variable.
