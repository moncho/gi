# Keychain

Gi keeps a keychain modelled on Piclaw's: named credentials encrypted in the session database, managed in Settings →
Keychain and made available to shell commands by Piclaw's substitution rules (`internal/keychain`).

## Storage

Entries live in the `keychain_entries` table of gi's database, in Piclaw's format:

- The payload `{"secret": …, "username": …}` is sealed with AES-256-GCM, with a 12-byte nonce and the entry name as
  additional data, so a row cannot be moved to another name.
- The key is derived per entry with PBKDF2-SHA256 (150,000 iterations, 16-byte salt) from the master key.
- Name, type (`secret`, `token`, `password`, `basic`), the user note, the agent note and the timestamps are not
  encrypted.
- The database runs with `secure_delete`, so deleted rows are overwritten.

Rows written by Piclaw's keychain open in gi's with the same master key (`TestMatchesPiclaw`).

The master key is `GI_KEYCHAIN_KEY`, or the trimmed contents of the file named by `GI_KEYCHAIN_KEY_FILE`. Piclaw's
`PICLAW_KEYCHAIN_KEY` and `PICLAW_KEYCHAIN_KEY_FILE` are read when gi's are unset. Without a key the keychain is
disabled: entries can still be listed and deleted, but not added, revealed or injected.

## Settings → Keychain

The web Settings dialog has a Keychain section (`web/src/gi-settings-keychain.ts`, adapted from Piclaw 3.2.5):

- The list shows each entry's name, type, shell variable and update date, never its secret, with a count
  ("N entries, encrypted at rest") and a filter over names, types, variables and notes.
- Add entry takes a name, a type, the secret, an optional username, a user note and an agent note. Saving under an
  existing name replaces the entry.
- Reveal asks for the master password first, then shows that entry's secret (and username) with Copy controls; Hide
  closes it. Only one entry is revealed at a time.
- Delete asks for confirmation on the row (Yes / No).

The API is `GET`/`POST`/`DELETE /api/settings/keychain`, `POST /api/settings/keychain/notes` and
`POST /api/settings/keychain/reveal`, behind the web session. Changes and reveals follow the provider-key policy:
HTTPS or loopback only, same origin, JSON bodies. Reveal answers 401 with `needs_master_password` until the master key
is given.

## Shell substitution

The agent's `shell` tool, the web tool runner and the TUI's `!!` local commands prepare each command as Piclaw's bash
tool does:

- **Variable names.** An entry's variable is its name when that already is a shell variable name. Otherwise each run
  of `/`, `-` or `.` becomes `_`, other characters outside `[A-Za-z0-9_]` are dropped and the result is upper-cased
  (`fixtures/kc-x.v1` → `FIXTURES_KC_X_V1`). A result that is empty or starts with a digit has no variable. When two
  entries map to one variable, the first by name keeps it.
- **Injection.** A secret enters a command's environment only when the command text names its variable literally:
  `$NAME`, `${NAME}`, `$env:NAME` (PowerShell) or `%NAME%` (cmd). Indirect expansion, `printenv` and environment
  enumeration see nothing else. A variable already set in gi's own environment keeps its value.
- **Placeholders.** `keychain:<name>` in the command text is replaced by the entry's secret,
  `keychain:<name>:username` or `:user` by its username, and `:secret`, `:password` or `:token` by its secret. A
  placeholder naming no entry, or a username the entry lacks, fails the command before it runs, with Piclaw's message.

`scripts/golden-keychain.mjs` records Piclaw 3.2.5's own results (sealed rows, variable names, injected environments
and placeholder substitutions) for `TestMatchesPiclaw`.

## Differences from Piclaw

- Piclaw gates reveal with the web TOTP code when TOTP login is configured; gi always asks for the master password.
- Piclaw also exposes the keychain to the agent as a `keychain` tool, through its CLI and to SSH, Proxmox and
  Portainer tools, and redacts known secrets from tool output. Gi has none of these yet.
- Per-entry note editing in the list is not ported; notes are set when an entry is added or replaced.
