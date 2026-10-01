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
