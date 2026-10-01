# MCP support and codemode: completion plan

Status: **plan** (2026-10-01). Nothing here is shipped. This plan builds on two
pieces of earlier work: `chore/joker-wasm-deps`, which is merged as a
feasibility probe, and `feat/joker-mcp-codemode`, which is an unmerged
prototype.

## Target: Pi parity

Pi's MCP and codemode behaviour is documented in its `docs/mcp.md` and in
`@earendil-works/pi-codemode`. gi should match it.

- **Config:** `~/.pi/agent/mcp.json`, plus `.pi/mcp.json` in trusted projects.
  The format is `{"mcpServers": {...}}`.
  - stdio servers use `command`, `args`, `env` and `cwd`.
  - HTTP servers use streamable HTTP with `url`, `headers` and `oauth`. SSE is
    rejected.
  - Both accept `timeout`, `enabled`, `description`, `exposure` and
    `toolExposure`.
  - Values can use `${VAR}`, or `!command` when that is the whole value.
  - Invalid entries are skipped and reported; they do not stop other servers.
- **Naming:** tools are named `mcp__<server>__<tool>`, sanitised, with a hash
  suffix when names collide.
- **Exposure:** each server or tool is `codemode` (the default), `deferred`
  (found through `tool_search`), `direct`, or `hidden`. A `mcp_servers`
  system-prompt section lists servers whose tools are not declared directly.
- **Connections:** servers connect in the background. Only `direct` servers
  block the first prompt, for up to 10 seconds.
  - HTTP calls retry transient errors twice (408, 429, 5xx); tool calls are
    never retried.
  - Stopping a stdio server closes its stdin, then sends SIGTERM, then SIGKILL
    to the whole process group.
  - Server log notifications go to `~/.pi/agent/mcp.log`.
- **Results:** text results over 20 KB reach the model with the middle removed,
  and the full text is saved to a temporary file. Codemode scripts get the
  complete `CallToolResult`.
- **Resource tools:** `list_mcp_resources`, `list_mcp_resource_templates` and
  `read_mcp_resource`.
- **Permissions:** every MCP call goes through the tool pipeline, so hooks and
  permission gates apply to it. Calls from codemode carry the parent tool-call
  ID.
- **Commands:** `/mcp` for status, login, logout, reconnect, exposure and
  enabling; `gi mcp add|remove|list|login|logout` on the CLI.
- **The codemode tool:** model-written **JavaScript** runs in QuickJS compiled
  to WebAssembly.
  - The script body is an async function.
  - Script API: `tools.<name>()`, `ALL_TOOLS`, `text()`, `image()`, `exit()`,
    `store()`/`load()`, `searchTools()`, `describeTool()` and
    `describeNamespace()`.
  - A `// @options:` first line can set `max_output_tokens` and `timeout_ms`.
  - Scripts get no timers, `fetch`, modules or `WebAssembly`.
  - Nested calls never enter the model's context.

## What already exists

| Piece | Where | Reuse |
|---|---|---|
| wazero v1.12 pinned; Joker built for `wasip1` via a bootstrap-splitting overlay; tests for evaluation and cancellation on both wazero backends | `main`: `tests/joker-wasi`, `scripts/joker-wasi-overlay`, `docs/internal/scripting/joker-wasi.md` | Shows that wazero runs, cancels and isolates guests on this ARM64 host. Joker itself needs a 1,460-helper source overlay just to compile. |
| stdio MCP broker on the official Go SDK (`modelcontextprotocol/go-sdk` v1.8.0): lazy connections, allowlists, `mcp__server__tool` naming, bounded frames and output, process-group stop | `feat/joker-mcp-codemode`: `internal/codemode` (unformatted prototype; its own config format; no CLI wiring yet) | Rewrite against Pi's `mcp.json` format and move into `internal/mcp`. Keep the bounds and the process handling. |
| Tool registry and turn pipeline (hooks, events, audit) | `internal/tools`, `internal/turn` | MCP and codemode tools register here, so hooks and permissions apply as in Pi. |

## Decision needed: the codemode language

Two options:

1. **QuickJS under wazero.** This is the recommended option.
   - **Exact parity:** it runs the same JavaScript, the same script API and the
     same `// @options` grammar as Pi.
   - **Same model behaviour:** models already know how to write this script
     format for Pi.
   - **Small:** `quickjs-wasi` 3.6.2 is MIT-licensed and 637 KB. It needs only
     six `env` host imports (`host_call`, `host_interrupt`, …) and six WASI
     calls, and exports the QuickJS C API (450 functions).
   - **Cost:** gi must drive that C API from Go, porting what pi-codemode's
     worker does.
2. **Joker under wazero.** This is gi's own scripting language, and the probe
   exists.
   - **Not Pi-compatible:** scripts would differ from Pi's, and the model
     prompts and script grammar would have to be gi-specific.
   - **Heavier:** a whole-interpreter guest that needs a source overlay to
     compile.
   - **Unmeasured:** startup and throughput have not been measured.

The recommendation is option 1 for the `codemode` tool. Keep the Joker WASI probe
as evidence of sandboxing and as a possible future engine for gi-specific
scripting.

## Phases

Each phase lands with its own tests, through the Makefile, sequentially.

1. **MCP client core (`internal/mcp`)**
   - Load `mcp.json` from user and trusted-project locations with Pi's rules:
     validation, `${VAR}` and `!command` expansion, and override by name.
   - stdio and streamable-HTTP transports on the Go SDK, with lazy background
     connections, a per-request `timeout`, and retries for HTTP only.
   - Process-group shutdown, `mcp.log` with rotation, and tool-list changes.
   - Tests use fake servers built from the SDK's in-memory and stdio test
     servers.
2. **Exposure and tools**
   - Name sanitisation, collision hashing, and exposure resolution, including
     `toolExposure` patterns.
   - Register `direct` tools in the tool registry, so hooks and audit apply.
   - Add the `mcp_servers` prompt section, and the three resource tools.
   - Truncate text results over 20 KB, saving the full text to a temporary
     file.
3. **`tool_search` for `deferred` tools**
   - Search ranked by server description and tool metadata.
   - Tools it loads are recorded in the transcript and stay declared on that
     branch.
4. **Codemode engine (`internal/codemode`, QuickJS on wazero)**
   - Embed `quickjs.wasm`.
   - Run one module instance per execution, with memory limits, a deadline via
     `host_interrupt`, and context cancellation.
   - Implement Pi's globals (`tools`, `ALL_TOOLS`, `text`, `image`, `exit`,
     `store`/`load`, the search and describe functions) and the `// @options`
     line.
   - Use JSON round-trips for arguments and results.
   - Bound output and store sizes (`MAX_STORE_*`).
   - Port pi-codemode's behavioural tests as golden cases.
5. **The `codemode` tool and its activation**
   - Generate TypeScript declarations from tool schemas.
   - Activate automatically when a server with codemode exposure connects;
     respect `autoEnableCodemode` and `defaultTools: ["+codemode"]`.
   - Nested calls go through the tool pipeline with `parentToolCallId`.
   - Show it in TUI and web tool blocks.
6. **OAuth and the UI**
   - MCP OAuth (dynamic client registration, `mcp-auth.json`, refresh, loopback
     callback).
   - `/mcp` in the TUI and web UI.
   - The `gi mcp …` CLI.
7. **Acceptance**
   - Terminal tests and Playwright tests against fake stdio and HTTP servers.
   - Hostile-script tests: infinite loops, memory exhaustion, huge output,
     forged tool names.
   - Measure startup and per-call latency.
   - Document everything in `docs/internal/` and the README.

## Security notes

- Codemode scripts get no capability other than the tools injected into them:
  no filesystem, environment or network imports.
- WASI is limited to clocks, random numbers, and stdout/stderr captured into
  bounded buffers.
- A project's `.pi/mcp.json` is read only after project trust is granted. gi
  has no trust model yet (#16), so until one exists, project MCP configs are
  ignored.
- Credentials in `env` and `headers` are expanded only when a connection is
  made, and are never logged.
