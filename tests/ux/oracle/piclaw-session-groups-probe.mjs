// Mounted installed Piclaw 3.2.4 Classic picker with disposable session metadata.
// Read-only catalogue; no real branch mutations, stored history, or physical input.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { chromium, webkit } from 'playwright';
import { installPixelHost } from '../support/pixel-adapter.mjs';

const root = process.env.PICLAW_ORACLE_ROOT || '/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root, 'VERSION'), 'utf8')).trim(), '3.2.4');
const reference = JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json', import.meta.url), 'utf8'));
const base = JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json', import.meta.url), 'utf8'));
const current = 'web:default';
// Deliberately mix source order: the expected output must come from grouping.
const chats = [
  { chat_jid: 'web:other', root_chat_jid: 'web:other', agent_name: 'Other fixture', is_active: false },
  { chat_jid: 'web:archived', root_chat_jid: current, parent_branch_id: 'root', branch_id: 'archived', agent_name: 'Archived fixture', archived_at: '2026-09-28T00:00:00Z', is_active: false },
  { chat_jid: 'web:active', root_chat_jid: 'web:active', agent_name: 'Active fixture', is_active: true },
  { chat_jid: current, root_chat_jid: current, agent_name: 'Current fixture', is_active: false },
  { chat_jid: 'web:child', root_chat_jid: current, parent_branch_id: 'root', branch_id: 'child', agent_name: 'Tree fixture', is_active: false },
  { chat_jid: 'web:pinned', root_chat_jid: 'web:pinned', agent_name: 'Pinned fixture', is_active: false },
];
const cases = [];
for (const [browserName, type] of Object.entries({ chromium, webkit })) {
  for (const [viewportName, viewport] of Object.entries({ phone: { width: 390, height: 844 }, tablet: { width: 820, height: 1180 }, desktop: { width: 1440, height: 900 } })) {
    const browser = await type.launch({ headless: true });
    const page = await browser.newPage({ viewport, serviceWorkers: 'block' });
    const state = { ...base, sessionId: current, theme: 'dark' };
    const host = await installPixelHost({ page, host: 'piclaw', root, state, reference, allowPresenceBeacon: true });
    try {
      const html = (await fs.readFile(path.join(root, 'app/runtime/web/static/classic/index.html'), 'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__', '1');
      await page.route('**/agent/active-chats*', route => route.fulfill({ json: { chats } }));
      await page.route('**/agent/branches*', route => route.fulfill({ json: { chats } }));
      await page.route('**/timeline?*', route => route.fulfill({ json: { posts: [], has_more: false } }));
      await page.route(host.origin + '/', route => route.fulfill({ contentType: 'text/html', body: html }));
      await page.addInitScript(() => localStorage.setItem('piclaw:session-picker-preferences:v1', JSON.stringify({ pinnedChatJids: ['web:pinned'] })));
      await page.goto(host.origin);
      await host.connected();
      const input = page.locator('.compose-box textarea');
      await input.fill('group fixture draft');
      await page.locator('[data-testid="session-switcher"]').first().click();
      const popup = page.locator('[data-testid="session-popup"]');
      await popup.waitFor();
      const snapshot = await popup.evaluate(el => ({
        headings: [...el.querySelectorAll('.compose-session-section-heading')].map(heading => ({ text: heading.textContent.trim(), role: heading.getAttribute('role') })),
        namedGroups: el.querySelectorAll('[role="group"][aria-label]').length,
        rows: [...el.querySelectorAll('[data-testid="session-item"]')].map(row => ({
          jid: row.innerText.match(/web:[a-z]+/)?.[0] ?? '',
          selected: row.getAttribute('aria-selected'),
          current: row.getAttribute('aria-current'),
          text: row.innerText,
          pin: row.parentElement?.querySelector('.compose-session-row-pin')?.getAttribute('aria-pressed') ?? null,
        })),
      }));
      assert.deepEqual(snapshot.headings, ['Current', 'Pinned', 'Active', 'This session tree', 'Other sessions', 'Archived'].map(text => ({ text, role: 'presentation' })));
      assert.equal(snapshot.namedGroups, 0);
      assert.deepEqual(snapshot.rows.map(row => row.jid), [current, 'web:pinned', 'web:active', 'web:child', 'web:other', 'web:archived']);
      assert.deepEqual(snapshot.rows.filter(row => row.selected === 'true').map(row => row.jid), [current]);
      assert.deepEqual(snapshot.rows.filter(row => row.current !== null), []);
      assert.equal(snapshot.rows[1].pin, 'true');
      assert.match(snapshot.rows[2].text, /ACTIVE/i);
      assert.match(snapshot.rows[5].text, /ARCHIVED/i);
      assert.equal(await input.inputValue(), 'group fixture draft');
      assert.equal(new URL(page.url()).searchParams.get('chat_jid'), null);
      host.assert();
      assert.equal(host.calls.some(call => call.method !== 'GET' && /\/agent\/(?:branches|active-chats)/.test(call.path)), false);
      cases.push({ browser: browserName, viewport: viewportName, headings: snapshot.headings.map(heading => heading.text), orderedJids: snapshot.rows.map(row => row.jid), selected: current, namedGroups: snapshot.namedGroups, pinned: snapshot.rows[1].pin });
    } finally {
      await host.dispose();
      await browser.close();
    }
  }
}
console.log(JSON.stringify({ scope: 'Mounted installed Piclaw 3.2.4 read-only six-entry picker metadata; no stored branch graph or backend mutations.', cases }, null, 2));
