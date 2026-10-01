# MCP client (`internal/mcp`)

Status: the client core (#25, phase 1), tool exposure (phase 2) and
`tool_search` (phase 3) are in place. Codemode, `/mcp` and OAuth are later
phases
([plan](mcp-codemode-plan.md)). The TUI and web server call
`Engine.EnableMCP()` at startup; engines built for tests never read the user's
`mcp.json`.

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

## How tools reach the model

- **Naming:** tools are named like Pi's: `mcp__<server>__<tool>`, with every
  character outside `[A-Za-z0-9_]` replaced by `_`.
  - A name over 64 characters, or one that collides, gets `_` plus 8 hex
    characters of SHA-256 of `server\0tool`.
  - Tools of one server that sanitize to the same name all get the suffix.
  - The server's namespace is `mcp__<server>` with `-` replaced by `_`.
- **`direct` tools:** registered in the tool registry like built-in tools. Hooks,
  permissions, events and audit apply, and the source is `mcp:<server>`. A
  `readOnlyHint` annotation marks the tool as `read`.
  - Before a turn's tool set is admitted, admission waits up to 10 s, outside
    the runner lock, for servers that can expose direct tools.
  - When a server announces a changed tool list, its tools are re-registered,
    and withdrawn tools are unregistered.
- **`codemode` and `deferred` tools** are registered as *deferred*:
  executable, but not declared to the model until loaded. Admission's default
  tool set skips them.
- **`tool_search`** (Pi's, #25 phase 3) is registered whenever an enabled
  server can give tools `deferred` exposure. It is decided from the config,
  before servers connect.
  - **Ranking:** a port of Pi's BM25 ranker, using Pi's tokenizer, stop words
    and naive stemming. The search text is the name, the description, schema
    descriptions, property names, and the server namespace with its
    description and instructions. It searches deferred tools the session has
    not loaded yet.
  - **Loading:** matches are recorded in the session state (`loaded_tools`),
    so they survive restarts and resume, and are copied to forks and clones.
  - **Result:** pi's text (`Loaded N tools. They are available from your next
    call:` followed by `- name: first description line`). The tool-result
    message carries `AddedToolNames`, and the definitions join the running
    turn's tools, so the next model call declares them. go-ai uses the marker
    to load them at that point on providers with deferred tools; others get
    them in the normal tool list.
  - **Later turns** declare and allow every tool the session has loaded.
  - **Errors and defaults** are pi's: `query must not be empty`, `limit must
    be a positive integer`, a default limit of 8, and `No matching tools
    found.` when nothing matches.
- **`hidden` tools** are unreachable.
- **System prompt:** servers with codemode or deferred tools are listed in an
  `<mcp_servers>` section, using Pi's renderer (intro line, `- mcp__<server>
  (codemode|tool_search): <summary>`, 4096-character budget).
  - **Placement, as in Pi:**
    - The section's value when the session starts is part of the system
      prompt.
    - A later change (for example, a server connecting and its instructions
      becoming available) is added once to the conversation, as a system
      message updating the section, before the prompt that introduced it.
      It keeps that position on later turns, so earlier messages stay cached.
    - If MCP appears mid-session, its section is appended rather than added
      to the system prompt.
    - gi records the first value and the updates in the session state
      (`mcp_servers_context`), and inserts the updates when requests are
      built, so stored messages and compaction coverage are unchanged.
    - If compaction removes an update's anchor, that value moves into the
      system prompt.
    - go-ai sends the updates as mid-conversation system messages, or folds
      them into the system prompt for providers without support.
- **Results** follow Pi's `convertMcpResult`:
  - Text and embedded text resources pass through.
  - Resource links name `read_mcp_resource`.
  - Empty content falls back to `structuredContent` as JSON.
  - An `isError` result becomes a tool error.
  - Text over 20 KB keeps its start and end around `…N chars truncated…`,
    with Pi's warning header.
  - **gi differences:**
    - The full text, and any binary resource, is saved to
      `vfs://mcp-output/<session>/<id><ext>`, where the `read` tool can open
      it. Pi uses temp files. Saved outputs are pruned after 7 days (checked at MCP
      start and every 6 hours); Pi leaves its temp files to the OS.
  - **Images** (image blocks and embedded image resources) are attached to
    the tool result as image blocks, as in Pi. go-ai replaces them with
    placeholders for models without image input, and the stored transcript
    adds an `[image <mime>, <size>]` line. Any tool can attach images through
    `ToolRuntime.AttachImage`.
- **Resource tools:** `list_mcp_resources`, `list_mcp_resource_templates` and
  `read_mcp_resource` are registered while an enabled, non-hidden server with
  resources has direct exposure.
  - Without a `server` argument, the list tools return every resource from
    every server; with one, they return a page plus `nextCursor`.
  - Listing and reading are retried once after a transient error.

## `gi mcp` CLI (`internal/mcp/cli.go`)

`gi mcp` is a port of `pi mcp` that works without starting a session.

- **Commands:** `add`, `remove`, `list [--json]`, `login` and `logout`. The options and messages are Pi's.
- **`add`:**
  - Writes the server entry in Pi's shape and key order to the user `mcp.json`, or with `-l`/`--local` to the project's.
  - Keeps the rest of the file and its indentation (an ordered JSON edit followed by re-indenting).
  - Validates the entry with the same rules as `LoadConfig`.
- **`remove`:** deletes an entry. When the server is defined in the other scope, the error says so.
- **`list`:**
  - Connects to every enabled server and prints its state, its tools (marking tools whose exposure differs from the server's), its resource counts and any errors.
  - Exits 1 when an entry is invalid or an enabled server does not connect.
- **Project config:** gi does not read project MCP configuration until it has project trust (#16), so the project file is reported as ignored.
- **Sign-in:** `login` and `logout` report that OAuth is not supported yet (#25 phase 6c).
