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

## Skills (#36)

`internal/skills` ports Pi's `loadSkills`: user skills first
(`<gi agent dir>/skills`, then `<Pi agent dir>/skills`), then the project's
(`.gi/skills`, then `.pi/skills`); the first skill of a name wins, and a file
reached twice through symlinks loads once. Within a directory, Pi's rules:

- a directory containing `SKILL.md` is one skill (not searched further);
- otherwise `.md` files directly in the skills root are skills, and
  subdirectories are searched for `SKILL.md`;
- dot entries and `node_modules` are skipped; `.gitignore`, `.ignore` and
  `.fdignore` exclude paths below them;
- the YAML front matter must have a `description` (otherwise the file is not
  a skill); `name` defaults to the directory; names are checked against the
  Agent Skills spec as warnings; `disable-model-invocation: true` keeps a
  skill out of the system prompt (it can still be invoked with
  `/skill:name`).

The prompt tells the model to load skills with `read`, so user skill
directories (and the directories of discovered user skills) are readable
outside the workspace; writes there are refused (`tools.SetReadOnlyRoots`).

## Context files (#36)

`config.LoadContextFiles` ports Pi's `loadProjectContextFiles`: the agent
directory's file (gi's, else Pi's), then one file per directory from the
filesystem root down to the workspace, each the first of `AGENTS.override.md`,
`AGENTS.md`, `AGENTS.MD`, `CLAUDE.md`, `CLAUDE.MD`. In a linked git worktree
nested in its main repository, the worktree's file shadows the main
repository's. They become the prompt's `project_context` entries.

`.piclaw/config.json` (assistant and user names and avatars) is Piclaw's
file, not Pi's; gi reads and writes it only there.
