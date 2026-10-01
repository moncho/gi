# Codemode engine (`internal/codemode`)

Status: the engine is in place (#25, phase 4). The model-facing `codemode`
tool, its declarations and its toggles are phase 5
([plan](mcp-codemode-plan.md)).

## What it runs

Model-written **JavaScript**, with Pi's codemode semantics.

- The script is the body of an async function, so `return` and top-level
  `await` work.
- Its only capabilities are what the host injects:
  - `tools.<name>(args)`, under the tool's name and its identifier form
    (`my-tool` is also `tools.my_tool`);
  - `ALL_TOOLS`;
  - `text()`, `image()` (base64 `data:` URLs, `{image_url}` or MCP image
    content; remote URLs are rejected) and `console.*`;
  - `exit()`, which ends successfully at once and keeps the output;
  - `store(key, value)` / `load(key)`, with values up to 256 Ki and 1 Mi
    characters in total;
  - configured globals, including `namespace.fn`.
- Scripts get no timers, `fetch`, modules or `WebAssembly`.

## How

- **Runtime:** QuickJS-ng compiled to WASI (`quickjs-wasi` 3.6.2, MIT) runs
  under wazero.
- **Guest side:** Pi's codemode prelude (`@earendil-works/pi-codemode`
  0.99.2, MIT), vendored verbatim, so script behaviour matches Pi by
  construction. `scripts/vendor-codemode.mjs` refreshes both from the
  installed Pi packages (`internal/codemode/vendor`, with licences and
  `VERSIONS`).
- **Host side:** `engine.go` mirrors Pi's worker and host.
  - **Isolation:** each execution gets a fresh module instance (a fresh VM).
  - **Bridge:** it implements `bridge(kind, a, b, c)` for `call`, `global`,
    `output` and `done`.
  - **Calls:** tool and global calls run concurrently in goroutines. Results
    are settled on the VM's goroutine as JSON (or as the error message), and
    each call is recorded with its status and duration.
  - **Jobs and stalls:** pending jobs are drained after every step, and the
    prelude's `stalled()` fails a script that waits on a promise nothing can
    settle.
- **Limits:**
  - default timeout 300 s, enforced through `host_interrupt` and the context;
  - caller cancellation is reported as `aborted`;
  - QuickJS stack 512 KiB, so deep recursion throws a catchable `RangeError`;
  - linear memory capped at 256 MiB, plus an optional QuickJS heap limit;
  - WASI gets no filesystem, environment or arguments, and its stdout and
    stderr are discarded.
- **Errors** use Pi's kinds: `script` (with name, message and stack),
  `timeout`, `aborted` and `sandbox`.
- **Cost on this ARM64 laptop:** compiling QuickJS takes about 0.4 s, once per
  process (`codemode.Default()`); each execution takes about 10 ms, including
  VM start and prelude load.

## API

```go
e, _ := codemode.Default()
res := e.Execute(ctx, code, codemode.Options{Tools: []codemode.Tool{...}, Globals: ..., Store: ..., Timeout: ..., MemoryLimitBytes: ...})
// res.OK, res.Value (JSON), res.Error{Kind,Name,Message,Stack}, res.Output, res.Calls, res.StoreWrites
```

## The `codemode` tool (`internal/turn/codemode_tool.go`)

The tool the model calls is a port of Pi's codemode extension. Its input is
`{ "code": "<JavaScript>" }`. The code may start with a
`// @options: {"max_output_tokens": N, "timeout_ms": N}` line, which
`codemode.ParseSource` reads with Pi's rules and messages.

- **Description:**
  - **Generation:** `codemode.Description` ports `createCodemodeDescription`. Pi's intro and guidance texts are extracted verbatim from Pi's codemode extension by `scripts/vendor-codemode.mjs`.
  - **Tool samples:** rendered by Pi's own `declarations.js`, run in QuickJS (`Engine.RenderDeclarations`). Results are cached by a hash of the declarations.
  - **Listed tools:** in mode `on`, tools without direct exposure (MCP codemode and deferred tools); in mode `only`, every callable tool.
  - **Inline budget:** sections are chosen per namespace, cheapest first, until `codemode.inlineBudget` is spent (default 3000 tokens). Deferred-exposure tools are never listed.
  - **Models API:** omitted (gi has no `models.*`).
- **Loadout (each request):**
  - **Mode `on`:** declared callable tools get their codemode declaration as their description.
  - **Mode `only`:** declarations of direct tools are left out. `codemode` and `tool_search` stay.
- **Callable tools:**
  - **Included:** the turn's active tools, plus every deferred registry entry (MCP codemode/deferred tools).
  - **Excluded:** `ModelOnly` tools (`codemode`, `tool_search`).
- **What scripts receive:**
  - **MCP tools:** their `CallToolResult` (without `_meta`), via `RegisteredTool.StructuredExecutor`, with `OutputSchema` from `codemode.MCPResultSchema`.
  - **Other tools:** their text.
- **Nested calls:**
  - **Hooks:** they run `tool_call`, `approve_tool` and `tool_result` (text tools). Payloads carry `parent_tool_call_id` and IDs look like `<parent>/<n>`.
  - **Events:** `tool_started`/`tool_finished`/`tool_failed` runtime events are published.
  - **Results:** they are not added to the transcript. Only the script's output reaches the model, as in Pi.
  - **Errors:** a blocked or failed call rejects with an Error.
- **Globals:**
  - **Discovery:** `searchTools(query, {limit, namespace})` (BM25 from `tool_search`), `describeTool(name)` and `describeNamespace(name)` (MCP servers).
  - **Store:** `store`/`load` persist in session state (`codemode_store`), applied only when the script succeeds.
- **Output:**
  - **Format:** `Script completed|Script failed\nWall time X seconds\nOutput:\n` followed by the text items. A returned value is appended like `text()`.
  - **Failures:** they add `Script error:\n<stack>` plus Pi's tool-call summary, and the tool result is an error.
  - **Long output:** past `max_output_tokens` (default 10000, 4 characters per token) the text keeps its start and end. The full text is saved at `vfs://codemode-output/<session>/<id>.txt`, which is pruned after 7 days with `mcp-output`.
  - **Images:** attached to the tool result.
- **Limits:** 256 MiB of memory; no timeout unless `timeout_ms` is set. Aborting the turn cancels the script.
- **Enabling codemode:**
  - **Built-in:** codemode is registered unless `extensions` contains `-builtin:codemode`.
  - **Declared by default:** when `defaultTools` enables it (layered user→project: `+codemode`/`-codemode` edit the selection, plain names replace it), or when an enabled MCP server has codemode exposure and `autoEnableCodemode` is not false (an explicit `-codemode` wins).
  - **Session toggle:** `codemode_mode` (`on`/`off`/`only`, set by `Engine.SetSessionCodemode` and the TUI's `/codemode [on|off|only|default|status]`) overrides this at admission. When MCP servers have codemode tools but codemode is off, gi logs a warning at startup and `/codemode status` says so. `only` also forces the `only` presentation, as does `codemode.mode: "only"` in settings.
