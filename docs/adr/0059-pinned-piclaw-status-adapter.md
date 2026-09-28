# ADR 0059: Pin the installed Piclaw status renderer behind an adapter

- Status: Accepted
- Date: 2026-09-28

## Context

Gi's supplied `web/src/components/status.ts` predates installed Piclaw 3.2.4. It has no tool Output panel and differs in disclosure, tail selection and timing. Editing the supplied components or presenting Gi's separate tool footer as equivalent would break the port contract.

## Decision

Keep the original protected component/UI/pane trees unchanged. Store verbatim installed status source, its small UI dependencies, translation/storage helpers and agent stylesheet in `web/piclaw-status-3.2.4/`, with the upstream MIT licence and a source-provenance manifest.

`scripts/piclaw-status-adapter.mjs` redirects status imports at build time. It retains Gi's API and Markdown adapters and the same Preact instance. Builds verify source hashes, not generated rendering hashes. The renderer and stylesheet are therefore reviewable without requiring an installed Piclaw runtime to build Gi.

Retain the existing measured-overflow safeguard through `patch-pinned-status-resize.mjs`. The installed renderer measures on content/expansion changes but not container-only resizing. A wrapped paragraph can otherwise lose its disclosure control. The guarded patch changes measurement only; it preserves the installed DOM, wording and rendering. Neither source tree is modified.

Configure Marked with Piclaw's `breaks: true` and `gfm: true` when publishing the vendor global. Gi does not run Piclaw's app-shell bootstrap, which normally applies those options. This fixes the independently reproduced missing line breaks without racing module load order.

## Runtime data

Native `tool.output` events hold a bounded status preview: the newest 100 lines and at most 12 KiB, matching the installed translator's limits. UTF-8 clipping starts at a rune boundary. Identity includes turn, call and occurrence because a provider can reuse call IDs.

The reporter persists before notifying clients, throttles intermediate snapshots to 250 ms and flushes the final output. Shell stdout/stderr can stream; tools with only a return value supply their final preview. Raw tool results and model history retain their existing contracts. A preview-persistence failure fails the turn closed. Shell writers kill the command process group and continue draining rather than stranding a child on a full pipe.

Snapshots restore running Output after reload. A terminal tool in an active turn shows `Waiting for model...`; authoritative idle removes the pane. Command arguments remain separate from output. The bootstrap shell no longer sends tool bytes as assistant Draft deltas.

## Verification and limits

`make test-piclaw-output-oracle` executes the installed translator and shipped browser assets independently of the vendored copy, then compares Output DOM, text and selected computed styles against Gi. It covers collapsed/expanded content, sanitisation, waiting and idle transitions in Chromium and WebKit at three viewport sizes. It does not compare screenshot bytes.

Native tool lifecycle, Thoughts/Draft, store reload/identity, failure and race tests remain separate gates. This decision does not accept the entire web UX, multi-tool concurrency, provider/device parity or the still-failing older WebKit reload oracle.
