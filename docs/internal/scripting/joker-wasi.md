# Joker WASI feasibility and dependency baseline

Status: dependency upgrade and smoke probe; **not shipped MCP codemode**.

## Dependency pins

- Joker: `v1.8.1-0.20260921221227-cd9de78c1f44`, upstream commit
  `cd9de78c1f4493b35303a166ac6df18bda134950` (also tagged upstream `v42.11.2`).
  Go resolves this commit to a v1 pseudo-version because the module retains its
  unsuffixed import path; do not substitute a v42 module path.
- go-ai: `v0.99.2-0.20260930214227-3edf860c4e8b`, upstream Pi 0.99.2 parity
  commit `3edf860c4e8b28f16b2e0499bdc6b3e0d221a869`. Remote tags checked during
  this upgrade stop at `v0.99.1`; replace the commit pin with a proper release
  tag when available. No tag was created in the dependency repository.
- wazero: `v1.12.0`, already the previous baseline and current Joker requirement.

These are targeted runtime dependency updates, not a blanket dependency refresh.
Existing privileged in-process scripting remains unchanged.

## Reproducible probe

```sh
make test-joker-wasi
```

Builds `tests/joker-wasi/guest` with `GOOS=wasip1 GOARCH=wasm` into
`bin/joker-wasi-probe.wasm`, then runs the guest with both wazero's interpreter and native compiler backends.
The guest imports Joker core, initializes its streams, and reads/parses/evaluates
one form supplied as an argument. Parsing and evaluation take place inside WASM.
It does not import Gi's bridge or Joker's broad standard-library registrations.

The test checks arithmetic, collection transformation, deadline termination of
an infinite Lisp loop, and a fresh successful invocation after cancellation.
Each invocation uses a separate module instance, a 256 MiB linear-memory limit,
context-driven termination, and no filesystem mounts, inherited environment or
network imports. Ordinary `make test` skips guest execution unless
`GI_JOKER_WASI_GUEST` points to a built guest; use the dedicated target for evidence.

## ARM64 native compiler fix

The unmodified guest exceeded wazero v1.12.0's ARM64 per-function code-size
limit: `29079668 > 16777215`. The oversized function was Go's synthetic package
initializer for Joker's generated bootstrap payloads, not a user Lisp function.

The probe now copies Joker into `bin/joker-wasi-module` and uses a Go build
overlay to extract larger bootstrap variable initializers (expressions at least
512 source bytes) into `//go:noinline` helpers. The pinned source yields 1,460
helpers. Small initializers remain inline: extracting all 33,510 eligible values
instead exceeded Go's own 65,536-basic-block limit for the synthetic initializer.
No compiler limit is raised and no module-cache files are modified.

Variable declarations retain their initializer calls. Go's dependency analysis
follows helper bodies, preserving initialization dependencies, order and evaluation
count rather than moving assignments into later `init()` functions. Regression
tests compare original and transformed initialization ordering with `go/types`,
check helper directives and ensure the original source remains unchanged. The
transformation fails explicitly when no eligible initializers are found.

Both backends now pass arithmetic, collection transformation, infinite-loop
cancellation and a fresh successful invocation after cancellation on this ARM64
host. The dedicated target rebuilds the transformed guest reproducibly; normal
Gi builds and native Joker execution do not use this overlay. Recheck this target
when updating Joker, Go or wazero: the source-size threshold is a build heuristic,
not a guarantee about future compiler output. This is a local build workaround;
Joker's upstream generator has not been changed.

## Security and scope limits

This is a feasibility harness, **not a complete security sandbox**. No MCP host
imports, configuration loader, call authorization, approvals, nested-call audit,
production output limits, or model-visible codemode tool are implemented here.
The test captures stdout in an unbounded buffer because its fixture is trusted;
production execution must bound it. The linear-memory ceiling does not bound
host allocations or compiled-module memory. We have not proven that every
accessible Joker operation fails closed or tested hostile guest programs beyond
the infinite loop. Cancellation of future host MCP calls needs its own tests.

Before selecting WASI instead of a subprocess, implement the capability-limited
host broker, audit available guest procedures, enforce host/output limits, and
measure startup and throughput. Keep the Pi-compatible MCP configuration work
separate from language/runtime choice.
