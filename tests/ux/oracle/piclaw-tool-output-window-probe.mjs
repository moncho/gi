// Deterministic installed Piclaw 3.2.4 tool-output status projection.
// Text-only cases overlap Go ToolOutputPreview's input domain; block/details
// cases describe installed behaviour but have no Go-string analogue.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const root = process.env.PICLAW_ORACLE_ROOT || '/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root, 'VERSION'), 'utf8')).trim(), '3.2.4');
const { createStreamingEventHandler } = await import(pathToFileURL(path.join(root, 'app/runtime/src/channels/web/sse/agent-events.ts')).href);
const input = [
  { name: 'empty', text: '' },
  { name: 'spaces', text: '  \r\n  ' },
  { name: 'surrounding-space', text: '  first  \r\nsecond\r\n' },
  { name: 'trailing-newline', text: 'first\n' },
  { name: 'last-100-lines', text: Array.from({ length: 120 }, (_, i) => `line-${String(i).padStart(3, '0')}`).join('\r\n') },
  { name: 'utf8-byte-window', text: 'β'.repeat(6145) },
  { name: 'utf8-emoji-window', text: '🙂'.repeat(3073) },
  { name: 'utf8-split-boundary', text: 'β'.repeat(6145) + 'x' },
  { name: 'utf8-three-byte-boundary', text: '€'.repeat(4097) + 'x' },
  { name: 'utf8-four-byte-boundary', text: '🙂'.repeat(3073) + 'x' },
  { name: 'two-blocks', blocks: ['alpha', 'beta'] },
  { name: 'reported-truncation', text: 'short', details: { truncation: true } },
];
const cases = [];
for (const sample of input) {
  const frames = [];
  const handler = createStreamingEventHandler({
    emitter: { status: payload => frames.push(payload) }, agentId: 'default', threadId: 'window', turnId: sample.name,
    displayUpdateIntervalMs: 0,
  });
  const id = 'fixture-call';
  handler({ type: 'tool_execution_start', toolCallId: id, toolName: 'shell', args: { command: 'fixture' } });
  handler({ type: 'tool_execution_update', toolCallId: id, toolName: 'shell', partialResult: {
    content: (sample.blocks ?? [sample.text]).map(text => ({ type: 'text', text })),
    ...(sample.details ? { details: sample.details } : {}),
  } });
  const status = frames.at(-1);
  assert.equal(status.tool_call_id, id, sample.name);
  const projection = Object.fromEntries(['output_preview', 'output_total_lines', 'output_preview_lines', 'output_truncated'].filter(k => Object.hasOwn(status, k)).map(k => [k, status[k]]));
  cases.push({ name: sample.name, projection });
}
assert.deepEqual(cases.map(c => c.name), input.map(c => c.name));
const byName = Object.fromEntries(cases.map(c => [c.name, c.projection]));
assert.deepEqual(byName.empty, {});
assert.deepEqual(byName.spaces, { output_preview: '  \n  ', output_total_lines: 2, output_preview_lines: 2, output_truncated: false });
assert.deepEqual(byName['surrounding-space'], { output_preview: '  first  \nsecond\n', output_total_lines: 3, output_preview_lines: 3, output_truncated: false });
assert.deepEqual(byName['trailing-newline'], { output_preview: 'first\n', output_total_lines: 2, output_preview_lines: 2, output_truncated: false });
const lineWindow = byName['last-100-lines'];
assert.deepEqual({ total: lineWindow.output_total_lines, preview: lineWindow.output_preview_lines, truncated: lineWindow.output_truncated }, { total: 120, preview: 100, truncated: true });
assert.equal(lineWindow.output_preview.split('\n')[0], 'line-020');
assert.equal(lineWindow.output_preview.split('\n').at(-1), 'line-119');
for (const [name, prefix, byteLength] of [
  ['utf8-byte-window', 'β', 12288], ['utf8-emoji-window', '🙂', 12288],
  ['utf8-split-boundary', '�β', 12290], ['utf8-three-byte-boundary', '��€', 12292],
  ['utf8-four-byte-boundary', '���🙂', 12294],
]) {
  const projection = byName[name];
  assert.equal(Buffer.byteLength(projection.output_preview), byteLength, name);
  assert.ok(projection.output_preview.startsWith(prefix), name);
  assert.deepEqual([projection.output_total_lines, projection.output_preview_lines, projection.output_truncated], [1, 1, true], name);
}
assert.deepEqual(byName['two-blocks'], { output_preview: 'alpha\nbeta', output_total_lines: 2, output_preview_lines: 2, output_truncated: false });
assert.deepEqual(byName['reported-truncation'], { output_preview: 'short', output_total_lines: 1, output_preview_lines: 1, output_truncated: true });
console.log(JSON.stringify({ scope: 'Installed Piclaw 3.2.4 event translator; 12 deterministic cases, no UI, persistence or provider.', cases: cases.map(({ name, projection }) => ({ name, bytes: Buffer.byteLength(projection.output_preview || ''), lines: projection.output_preview_lines ?? 0, truncated: projection.output_truncated ?? false })) }, null, 2));
