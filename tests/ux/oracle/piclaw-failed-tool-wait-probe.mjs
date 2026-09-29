// Installed Piclaw 3.2.4 translator and shipped Classic status UI. One
// synthetic success and one failure; no provider, backend, or stored turn.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { chromium, webkit } from 'playwright';
import { installPixelHost } from '../support/pixel-adapter.mjs';

const root = process.env.PICLAW_ORACLE_ROOT || '/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root, 'VERSION'), 'utf8')).trim(), '3.2.4');
const reference = JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json', import.meta.url), 'utf8'));
const base = JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json', import.meta.url), 'utf8'));
const html = (await fs.readFile(path.join(root, 'app/runtime/web/static/classic/index.html'), 'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__', '1');
const { createStreamingEventHandler } = await import(pathToFileURL(path.join(root, 'app/runtime/src/channels/web/sse/agent-events.ts')).href);
const cases = [];
for (const [browserName, type] of Object.entries({ chromium, webkit })) for (const [viewportName, viewport] of Object.entries({ phone: { width: 390, height: 844 }, tablet: { width: 820, height: 1180 }, desktop: { width: 1440, height: 900 } })) {
  const browser = await type.launch({ headless: true }), page = await browser.newPage({ viewport, serviceWorkers: 'block' });
  const state = { ...base, theme: 'dark', sessionId: 'web:default' };
  const host = await installPixelHost({ page, host: 'piclaw', root, state, reference, allowPresenceBeacon: true });
  let status = { status: 'idle', data: null };
  const frames = [];
  const emitter = { status: payload => { frames.push(payload); status = { status: 'active', data: { ...payload, chat_jid: state.sessionId } }; host.emit('agent_status', status.data); } };
  const handler = createStreamingEventHandler({ emitter, agentId: 'default', threadId: 'fixture', turnId: 'failed-tool', displayUpdateIntervalMs: 0 });
  try {
    await page.route(host.origin + '/', route => route.fulfill({ contentType: 'text/html', body: html }));
    await page.route('**/agent/status?*', route => route.fulfill({ json: { status, model: state.model, context: state.model.context_usage, metrics: state.metrics, errors: [] } }));
    await page.route('**/agent/picker-pins', route => route.fulfill({ json: { scope: 'failed-tool', revision: 0, models: [], sessions: [] } }));
    await page.goto(host.origin); await host.connected();
    const input = page.locator('.compose-box textarea'); await input.fill('unsent failure draft');
    const seen = [];
    for (const [name, isError, title] of [['success', false, 'Waiting for model...'], ['failure', true, 'Reviewing failed tool result...']]) {
      const id = 'call-' + name;
      handler({ type: 'tool_execution_start', toolCallId: id, toolName: 'shell', args: { command: 'printf ' + name } });
      await page.getByText('Running: shell', { exact: false }).first().waitFor();
      handler({ type: 'tool_execution_end', toolCallId: id, toolName: 'shell', result: { content: [{ type: 'text', text: name + ' output' }] }, isError });
      const last = frames.at(-1);
      assert.equal(last.type, 'waiting'); assert.equal(last.title, title); assert.equal(last.last_completed_tool.status, isError ? 'failed' : 'completed');
      await page.getByText(title, { exact: true }).first().waitFor();
      assert.equal(await page.locator('[data-panel-key="tool-output"]').count(), 0);
      assert.equal(await page.locator('.post').count(), 0);
      seen.push({ name, title, completedStatus: last.last_completed_tool.status });
    }
    assert.equal(await input.inputValue(), 'unsent failure draft');
    host.assert(); cases.push({ browser: browserName, viewport: viewportName, seen });
  } finally { await host.dispose(); await browser.close(); }
}
console.log(JSON.stringify({ scope: 'Installed 3.2.4 event translator and UI with synthetic successful/failed tools; no provider, persisted result or physical input.', cases }, null, 2));
