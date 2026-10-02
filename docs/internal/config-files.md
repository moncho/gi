# Configuration file lookup (#26)

gi reads Pi's configuration files unchanged and lets gi's own directories
override them (`internal/config/config_paths.go`).

| Level | Lookup order |
|---|---|
| user | `$GI_CODING_AGENT_DIR` or `~/.gi/agent`, then `$PI_CODING_AGENT_DIR` or `~/.pi/agent` (Pi's `getAgentDir`) |
| project | `<workspace>/.gi`, then `<workspace>/.pi` |

For a file, the first existing location wins; files are never merged across
the two. A file is written where it was read; when neither location has it,
it is created in Pi's location, so it stays shared with Pi.

| File | Level | Where |
|---|---|---|
| `settings.json` | project, then user (Pi's merge of global under project) | `config.Load`, `settings_file.go` (reads and writes the project file that exists) |
| `auth.json` | user | `inference.AuthFilePath`, credential store (writes the file read; Pi refreshes tokens in place) |
| `models-store.json` | user | `inference.PiModelsStorePath` (read-only) |
| `mcp.json` | user and project | `internal/mcp` |
| `mcp-auth.json` | user | MCP OAuth credentials, shared with Pi |

Directories of items are scanned `.gi` before `.pi`, the first item of a
name winning: `skills/`, `tools/` (script tool manifests), `extensions/`.
The workspace index includes `.gi/skills` as a skills root only where it
exists, so other workspaces keep their index fingerprint.

Not covered here:

- `.piclaw/config.json` (assistant and user names and avatars) is Piclaw's
  file, not Pi's; gi reads and writes it only there.
- The workspace `AGENTS.md` is not under these directories.
- Pi also loads user-level skills (`~/.pi/agent/skills`) and the agent
  directory's `AGENTS.md`; gi reads skills from the workspace only, because
  the read tool is confined to the workspace.
