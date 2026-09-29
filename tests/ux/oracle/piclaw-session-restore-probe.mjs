// Installed Piclaw 3.2.4 archived-row Restore, against a disposable catalogue.
// Archive is not exposed by this Classic picker; Prune/Purge are distinct actions.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { chromium, webkit } from 'playwright';
import { installPixelHost } from '../support/pixel-adapter.mjs';

const root = process.env.PICLAW_ORACLE_ROOT || '/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root, 'VERSION'), 'utf8')).trim(), '3.2.4');
const reference = JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json', import.meta.url), 'utf8'));
const base = JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json', import.meta.url), 'utf8'));
const current = 'web:default', archived = 'web:archived';
const cases = [];
for (const [browserName, type] of Object.entries({ chromium, webkit })) {
  for (const [viewportName, viewport] of Object.entries({ phone: { width: 390, height: 844 }, tablet: { width: 820, height: 1180 }, desktop: { width: 1440, height: 900 } })) {
    const browser = await type.launch({ headless: true });
    const page = await browser.newPage({ viewport, serviceWorkers: 'block' });
    const state = { ...base, sessionId: current, theme: 'dark' };
    const host = await installPixelHost({ page, host: 'piclaw', root, state, reference, allowPresenceBeacon: true });
    let restored = false, release;
    const gate = new Promise(resolve => { release = resolve; });
    const calls = [];
    const catalogue = () => [
      { chat_jid: current, root_chat_jid: current, agent_name: 'Current fixture', is_active: false },
      { chat_jid: archived, root_chat_jid: current, agent_name: 'Archived fixture', is_active: false,
        ...(!restored ? { archived_at: '2026-09-28T00:00:00Z' } : {}) },
    ];
    try {
      const html = (await fs.readFile(path.join(root, 'app/runtime/web/static/classic/index.html'), 'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__', '1');
      await page.route('**/agent/active-chats*', route => route.fulfill({ json: { chats: catalogue() } }));
      await page.route('**/agent/branches*', route => route.fulfill({ json: { chats: catalogue() } }));
      await page.route('**/timeline?*', route => route.fulfill({ json: { posts: [], has_more: false } }));
      await page.route('**/agent/branch-restore', async route => {
        calls.push({ method: route.request().method(), path: new URL(route.request().url()).pathname, body: route.request().postDataJSON() });
        await gate;
        restored = true;
        await route.fulfill({ json: { ok: true } });
      });
      await page.route(host.origin + '/', route => route.fulfill({ contentType: 'text/html', body: html }));
      await page.goto(host.origin);
      await host.connected();
      const input = page.locator('.compose-box textarea');
      await input.fill('restore fixture draft');
      await page.locator('[data-testid="session-switcher"]').first().click();
      const popup = page.locator('[data-testid="session-popup"]');
      await popup.waitFor();
      const row = popup.locator('[data-testid="session-item"]').filter({ hasText: archived });
      await row.waitFor();
      assert.equal(await popup.locator('.compose-session-section-heading').filter({ hasText: 'Archived' }).count(), 1);
      assert.equal(await popup.getByRole('button', { name: /^Archive / }).count(), 0);
      host.assert();
      await row.click();
      await page.waitForFunction(() => document.querySelector('[data-testid="session-popup"]') === null);
      assert.equal(calls.length, 1);
      assert.deepEqual(calls[0], { method: 'POST', path: '/agent/branch-restore', body: { chat_jid: archived } });
      assert.equal(restored, false);
      assert.equal(await input.inputValue(), 'restore fixture draft');
      assert.equal(new URL(page.url()).searchParams.get('chat_jid'), null);
      host.assert();
      state.sessionId = archived; // A successful restore can select the returned chat.
      release();
      await page.waitForURL(url => new URL(url).searchParams.get('chat_jid') === archived);
      await page.waitForTimeout(200);
      assert.deepEqual(host.failures.filter(error => !/^network: .*\/sse\/stream\?chat_jid=web%3Adefault (?:Load request cancelled|net::ERR_ABORTED)$/.test(error)), []);
      assert.equal(restored, true);
      cases.push({ browser: browserName, viewport: viewportName, callback: calls[0], draftRetained: true, heldURLUnchanged: true, selectedAfterAck: true, noArchiveAction: true });
    } finally {
      release();
      await host.dispose();
      await browser.close();
    }
  }
}
console.log(JSON.stringify({ scope: 'Mounted installed Classic archived-row Restore with held disposable response. Archive and backend branch state are outside this fixture.', cases }, null, 2));
