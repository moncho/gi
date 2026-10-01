# MCP client (`internal/mcp`)

Status: the client core is in place (#25, phase 1). It is not yet wired into
turns: tool exposure, `tool_search`, codemode, `/mcp` and OAuth are later
phases ([plan](mcp-codemode-plan.md)).

## Configuration

The format is Pi's `mcp.json`, read from:

1. user level: `~/.gi/agent/mcp.json`, otherwise `~/.pi/agent/mcp.json`;
2. project level: `<workspace>/.gi/mcp.json`, otherwise
   `<workspace>/.pi/mcp.json`. **This is read only for trusted projects.** gi
   has no project trust yet (#16), so project MCP configs are ignored for now.

For each file the first existing location wins; the `.gi` and `.pi` files are
not merged (#26). A project entry replaces a user entry with the same name, and
a project `autoEnableCodemode` overrides the user value.

```json
{
  "autoEnableCodemode": true,
  "mcpServers": {
    "fs":   {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "."], "env": {"TOKEN": "${MY_TOKEN}"}, "cwd": "."},
    "docs": {"url": "https://example.com/mcp", "headers": {"Authorization": "Bearer ${DOCS_TOKEN}"}, "timeout": 30,
             "exposure": "deferred", "toolExposure": {"search": "direct", "delete_*": "hidden"}, "description": "Product docs"}
  }
}
```

Rules, as in Pi:

- **Transport:** `command` selects stdio and `url` selects streamable HTTP.
  An optional `type` must be `stdio`, `http` or `streamable-http`. `sse` is
  rejected.
- **Names:** server names use letters, digits, `_` and `-`. Names that differ
  only in `-` and `_` are the same server, so a second one is rejected.
- **Common fields:**
  - `timeout`: seconds, default 60.
  - `enabled`: `false` keeps the entry without connecting.
  - `description`: what the server offers.
  - `exposure`: `codemode` (the default), `deferred`, `direct` or `hidden`;
    `codemode-deferred` is an alias of `codemode`.
  - `toolExposure`: per-tool overrides. An exact tool name wins, otherwise
    the first matching `*` pattern.
- **Values:** `env` and `headers` values expand `${VAR}`. A value that is
  wholly `!command` runs the command and uses its trimmed output. Expansion
  happens only when connecting. A leading `~/` expands in `command`, `args`
  and `cwd`, and a relative `cwd` resolves from the workspace.
- **Invalid entries** are reported (`Config.Errors`) and skipped; the other
  servers still connect.

## Lifecycle

- **Connecting:**
  - `Manager` connects lazily on first use, or in the background with
    `ConnectAll`.
  - Concurrent callers share one session per server.
  - HTTP connections retry transient failures (408, 429, 5xx, network) twice.
- **Calls:**
  - Tool calls are never retried, because the server may already have
    performed them.
  - Each request has the server's `timeout`.
  - Results with `isError` are returned as results, not errors.
- **Failures:**
  - A transport failure drops the session, and the next call reconnects.
  - Server-side JSON-RPC errors, cancellation and timeouts keep the session.
- **Tool lists** are paginated and cached until the server announces a change.
  New tools then appear and withdrawn ones become unreachable.
- **stdio servers:**
  - They inherit gi's environment plus the expanded `env`.
  - Each runs in its own process group.
  - Stopping one closes its stdin, sends SIGTERM to the group, waits up to
    2 s, then sends SIGKILL to anything left, so wrapper children (`npx`,
    `uvx`) do not survive.
- **Logging:**
  - Server logging notifications are written to `mcp.log` as
    `<time> [<server>] <level> <logger>: <message>`, and rotated to
    `mcp.log.1` past 5 MB.
  - Servers on protocol 2026-07-28 get the level per request through
    `_meta`; older servers through `logging/setLevel`.
- **Status:** `Status()` reports each server's state (`disabled`,
  `disconnected`, `connecting`, `connected` or `failed`), its error, tool
  count, exposure, source file, instructions and stderr tail.
