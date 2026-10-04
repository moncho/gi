import { test, expect } from '@playwright/test';
import { readFileSync, mkdirSync } from 'node:fs';
const feature = readFileSync('features/gi/settings/gi-settings.feature', 'utf8');
const inputName = 'Message (Enter to send, Shift+Enter for newline)...';
const dialogFor = page => page.getByRole('dialog', { name: 'Settings', exact: true });
async function setup(page, request, info) {
  await info.attach('gi-gherkin', { body: feature, contentType: 'text/plain' });
  const create = async suffix => {
    const response = await request.post('/api/sessions', { data: { agent_id: `settings-${info.project.name}-${Date.now()}-${suffix}` } });
    expect(response.status()).toBe(201); return (await response.json()).id;
  };
  const a = await create('a'), b = await create('b');
  await page.addInitScript(id => { if (!localStorage.getItem('gi_session_id')) localStorage.setItem('gi_session_id', id); }, a);
  await page.goto('/');
  const input = page.getByRole('textbox', { name: inputName, exact: true });
  await expect(input).toBeVisible(); await input.fill('settings draft');
  const open = async () => { await page.keyboard.press('Control+,'); await expect(dialogFor(page)).toBeVisible(); };
  const models = async () => { await dialogFor(page).getByRole('button', { name: 'Models', exact: true }).click(); await expect(dialogFor(page).getByLabel('Session model', { exact: true })).toBeVisible(); };
  const switchTo = async id => {
    await page.getByRole('button', { name: /Manage sessions for/ }).last().click();
    await page.locator(`[data-session-jid="gi:${id}"]`).getByRole('option').click();
    await expect.poll(() => page.evaluate(() => localStorage.getItem('gi_session_id'))).toBe(id);
  };
  return { a, b, input, open, models, switchTo, dialog: dialogFor(page) };
}

// Gi-derived cases intentionally do not carry @ux- tags or frozen parity credit.
test('@gi-settings-001 @gi-settings-002 Single scoped modal, responsive backdrop, focus and unchanged draft', async ({ page, request }, info) => {
  const { a, input, open, dialog } = await setup(page, request, info);
  if (!await page.locator('.workspace-sidebar').isVisible()) {
    await page.getByRole('button', { name: 'Menu', exact: true }).click();
    await page.getByRole('menuitem', { name: 'Show workspace', exact: true }).click();
  }
  await expect(page.locator('.workspace-sidebar')).toBeVisible();
  const beforeClass = await page.locator('.app-shell').getAttribute('class');
  await input.focus();
  await open(); await page.keyboard.press('Control+,'); await page.keyboard.press('Control+,');
  await expect(dialogFor(page)).toHaveCount(1);
  await expect(dialog.getByRole('navigation', { name: 'Settings sections' }).getByRole('button')).toHaveText(['General', 'Models', 'Appearance', 'Keyboard', 'Compaction', 'Providers', 'Keychain', 'Environment', 'Authentication']);
  await expect(dialog.getByText('Active instance settings · read-only')).toBeVisible();
  await expect(dialog.getByText(/Edit the files and restart Gi/)).toBeVisible();
  await expect(dialog.locator('input, select')).toHaveCount(3);
  await expect(dialog.locator('.gi-settings-values')).toContainText('test-model');
  expect(await page.locator('#app').evaluate(el => el.inert)).toBe(true);
  expect(await page.locator('.settings-dialog-backdrop').evaluate(el => getComputedStyle(el).backgroundColor)).toBe('rgba(0, 0, 0, 0.5)');
  expect(await page.locator('.settings-portal').evaluate(el => el.parentElement === document.body)).toBe(true);
  const box = await dialog.boundingBox(), viewport = page.viewportSize();
  expect(box.x).toBeGreaterThanOrEqual(0); expect(box.y).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(viewport.width); expect(box.y + box.height).toBeLessThanOrEqual(viewport.height);
  expect(await dialog.evaluate(el => { const box = el.getBoundingClientRect(); return el.contains(document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2)); })).toBe(true);
  await dialog.getByRole('button', { name: 'Close settings' }).focus(); await page.keyboard.press('Shift+Tab');
  // General ends with Instance Configuration's upload-limit stepper (Piclaw 3.2.5).
  await expect(dialog.getByRole('button', { name: 'Increase Upload limit (MB)', exact: true })).toBeFocused();
  await page.keyboard.press('Tab'); await expect(dialog.getByRole('button', { name: 'Close settings' })).toBeFocused();
  await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0); await expect(input).toBeFocused();
  expect(await page.locator('#app').evaluate(el => el.inert)).toBe(false);
  await expect(input).toHaveValue('settings draft'); expect(await page.locator('.app-shell').getAttribute('class')).toBe(beforeClass);
  await page.getByRole('button', { name: 'Menu', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Settings', exact: true }).click();
  await expect(dialog).toBeVisible(); await dialog.getByRole('button', { name: 'Close settings' }).click();
  await open(); await page.locator('.settings-dialog-backdrop').click({ position: { x: 2, y: 2 } }); await expect(dialog).toHaveCount(0);
  expect(await page.evaluate(() => localStorage.getItem('gi_session_id'))).toBe(a);
});

test('@gi-settings-003 Loading shell, native read failure and retry', async ({ page, request }, info) => {
  const { open, dialog } = await setup(page, request, info);
  let release, entered = false; const gate = new Promise(resolve => { release = resolve; });
  await page.route('**/api/runtime/config', async route => { entered = true; await gate; await route.abort(); });
  try {
    await open(); await expect.poll(() => entered).toBe(true); await expect(dialog.getByRole('status').filter({ hasText: 'Loading settings…' })).toBeVisible();
    release(); await expect(dialog.getByRole('alert')).toBeVisible();
    await page.unroute('**/api/runtime/config'); await dialog.getByRole('button', { name: 'Retry' }).click();
    await expect(dialog.locator('.gi-settings-values')).toContainText('test-model');
    await page.keyboard.press('Escape');
    let releaseAgain; const holdAgain = new Promise(resolve => { releaseAgain = resolve; });
    await page.route('**/api/runtime/config', async route => { const response = await route.fetch(); await holdAgain; await route.fulfill({ response }); });
    try { await open(); await expect(dialog.locator('.gi-settings-values')).toContainText('test-model'); } finally { releaseAgain(); }
  } finally { release(); }
});

test('@gi-settings-004 @gi-settings-005 @gi-settings-006 Native model confirmation, failure, reload and draft isolation', async ({ page, request }, info) => {
  const { a, b, input, open, models, dialog } = await setup(page, request, info);
  await page.locator('.compose-box input[type=file]').setInputFiles({ name: 'settings.txt', mimeType: 'text/plain', buffer: Buffer.from('settings draft file') });
  const runtime = await (await request.get('/api/runtime/config')).json();
  await open(); await models();
  const select = dialog.getByLabel('Session model', { exact: true });
  await expect(dialog.getByLabel('Filter models', { exact: true })).toBeFocused();
  await expect(dialog).toContainText(`gi:${a}`);
  const modelState = await (await request.get(`/api/sessions/${a}/model`)).json();
  expect(modelState.supports_thinking).toBe(false);
  await expect(dialog.getByRole('combobox',{name:'Session thinking level',exact:true})).toHaveCount(0);
  await expect(dialog.getByTestId('settings-context-capacity')).toHaveText(modelState.context_window > 0 ? String(modelState.context_window) : 'Unknown');
  await dialog.getByLabel('Filter models', { exact: true }).fill('bootstrap');
  await expect(select.locator('option')).toHaveCount(2); await select.selectOption('test/bootstrap');
  expect((await (await request.get(`/api/sessions/${a}/model`)).json()).current).toBe('test/test-model');
  let release, entered = false; const gate = new Promise(resolve => { release = resolve; });
  await page.route(`**/api/sessions/${a}/model`, async route => {
    if (route.request().method() !== 'PATCH') return route.continue();
    const response = await route.fetch(); entered = true; await gate; await route.fulfill({ response });
  });
  try {
    await dialog.getByRole('button', { name: 'Apply model' }).click(); await expect.poll(() => entered).toBe(true);
    await expect(dialog.getByRole('button', { name: 'Applying…' })).toBeDisabled();
    await expect(dialog.locator('header').getByLabel('Filter models', { exact: true })).toBeDisabled();
    // The "bootstrap" filter hides the Current model row until it matches (Piclaw 3.2.5).
    await expect(dialog.getByTestId('settings-current-model')).toHaveCount(0);
    release(); await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/bootstrap');
    await expect(dialog.getByLabel('Filter models', { exact: true })).toBeEnabled();
  } finally { release(); }
  await page.unroute(`**/api/sessions/${a}/model`);
  await dialog.getByLabel('Filter models', { exact: true }).fill(''); await select.selectOption('test/unavailable-model');
  await dialog.getByRole('button', { name: 'Apply model' }).click();
  await expect(dialog.getByRole('alert')).toContainText('unavailable or lacks credentials');
  await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/bootstrap'); await expect(dialog.getByRole('button', { name: 'Apply model' })).toBeEnabled();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('button', { name: 'Open model picker', exact: true })).toHaveText('test/bootstrap');
  await expect(input).toHaveValue('settings draft'); await expect(page.locator('.compose-file-pill[title="settings.txt"]')).toBeVisible();
  await page.reload(); await expect(input).toHaveValue('settings draft');
  await expect(page.locator('.compose-file-pill[title="settings.txt"]')).toBeVisible();
  await open(); await models(); await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/bootstrap');
  expect((await (await request.get(`/api/sessions/${b}/model`)).json()).current).toBe('test/test-model');
  expect((await (await request.get('/api/runtime/config')).json()).current).toBe(runtime.current);
  expect((await (await request.get(`/api/sessions/${a}/turns`)).json()).turns || []).toHaveLength(0);
});

test('@gi-settings-007 @gi-settings-026 A closed model write cannot overwrite another session settings or draft', async ({ page, request }, info) => {
  const { a, b, input, open, models, switchTo, dialog } = await setup(page, request, info);
  await open(); await models(); await dialog.getByLabel('Session model', { exact: true }).selectOption('test/bootstrap');
  let release, entered = false, done; const gate = new Promise(resolve => { release = resolve; }), delivered = new Promise(resolve => { done = resolve; });
  await page.route(`**/api/sessions/${a}/model`, async route => {
    if (route.request().method() !== 'PATCH') return route.continue();
    const response = await route.fetch(); entered = true; await gate; await route.fulfill({ response }); done();
  });
  try {
    await dialog.getByRole('button', { name: 'Apply model' }).click(); await expect.poll(() => entered).toBe(true);
    await page.keyboard.press('Escape'); await switchTo(b); await input.fill('other settings draft');
    await open(); await models();
    let crossReads = 0;
    page.on('request', req => { if (req.method() === 'GET' && new URL(req.url()).pathname === `/api/sessions/${b}/model`) crossReads++; });
    release(); await delivered; await page.waitForTimeout(100);
    expect(crossReads).toBe(0);
    await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/test-model'); await expect(dialog.getByRole('alert')).toHaveCount(0);
    if (process.env.GI_SETTINGS_CAPTURE && info.project.name.startsWith('chromium-')) {
      mkdirSync('test-results/gi-settings-captures', { recursive: true });
      await page.screenshot({ path: `test-results/gi-settings-captures/${info.project.name}.png` });
    }
    await page.keyboard.press('Escape'); await expect(input).toHaveValue('other settings draft');
    expect((await (await request.get(`/api/sessions/${a}/model`)).json()).current).toBe('test/bootstrap');
    await switchTo(a); await expect(input).toHaveValue('settings draft');
  } finally { release(); }
});

test('@gi-settings-004 @gi-settings-007 A held catalogue loads on demand and cannot replace a reopened session', async ({ page, request }, info) => {
  const { a, b, input, open, switchTo, dialog } = await setup(page, request, info);
  // Seed a distinguishable native model in B, not a synthetic catalogue response.
  expect((await request.patch(`/api/sessions/${b}/model`, { data: { model: 'test/bootstrap' } })).status()).toBe(200);
  let release, entered = false, done; const gate = new Promise(resolve => { release = resolve; }), delivered = new Promise(resolve => { done = resolve; });
  await open(); await expect(dialog.locator('.gi-settings-values')).toBeVisible();
  await page.route(`**/api/sessions/${a}/model`, async route => {
    if (route.request().method() !== 'GET' || entered) return route.continue();
    const response = await route.fetch(); entered = true; await gate; await route.fulfill({ response }); done();
  });
  try {
    await dialog.getByRole('button', { name: 'Models', exact: true }).click();
    await expect(dialog.getByRole('status')).toHaveText('Loading models…'); await expect.poll(() => entered).toBe(true);
    await page.keyboard.press('Escape'); await switchTo(b); await input.fill('B catalogue draft');
    await open(); await dialog.getByRole('button', { name: 'Models', exact: true }).click();
    await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/bootstrap');
    release(); await delivered;
    await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/bootstrap');
    await expect(dialog).toContainText(`gi:${b}`); await page.keyboard.press('Escape'); await expect(input).toHaveValue('B catalogue draft');
  } finally { release(); }
});

test('@gi-settings-001 Modal keyboard input cannot activate a background model picker', async ({ page, request }, info) => {
  const { a, input, open, dialog } = await setup(page, request, info);
  await page.getByRole('button', { name: 'Open model picker', exact: true }).click();
  await expect(page.getByRole('listbox', { name: 'Models', exact: true })).toBeVisible();
  await open();
  await dialog.getByRole('button', { name: 'Close settings' }).focus();
  await page.keyboard.press('ArrowDown'); await page.keyboard.press('Enter');
  await expect(dialog).toHaveCount(0);
  expect((await (await request.get(`/api/sessions/${a}/model`)).json()).current).toBe('test/test-model');
  await page.keyboard.press('Escape'); await expect(input).toHaveValue('settings draft');
});

const appearanceKey = 'gi_browser_appearance_v1';
const appearanceSnapshot = page => page.evaluate(() => ({
  theme: document.documentElement.dataset.colorTheme,
  tint: document.documentElement.dataset.tint,
  mode: document.documentElement.dataset.theme,
  background: getComputedStyle(document.documentElement).getPropertyValue('--bg-primary').trim(),
  meta: document.querySelector('#dynamic-theme-color')?.getAttribute('content'),
  saved: localStorage.getItem('gi_browser_appearance_v1'),
}));
async function appearance(page) {
  const dialog = dialogFor(page);
  await dialog.getByRole('button', { name: 'Appearance', exact: true }).click();
  await expect(dialog.getByRole('heading', { name: 'Appearance', exact: true })).toBeVisible();
  // Piclaw 3.2.5 shape: preset radios, a tint colour input and Clear tint; every change applies at once.
  const radio = name => name === 'default' ? dialog.getByRole('radio', { name: /^Default/ }) : dialog.getByRole('radio', { name: new RegExp(`^${name}$`, 'i') });
  const tint = async value => dialog.locator('input[type=color]').first().evaluate((e, v) => { e.value = v; e.dispatchEvent(new Event('input', { bubbles: true })); }, value);
  return { dialog, radio, tint, clearTint: dialog.getByTitle('Clear tint'), padding: dialog.getByLabel('Output padding (px)') };
}

test('@gi-settings-009 Explicit browser appearance survives session/reload without server or legacy writes', async ({ page, request }, info) => {
  const { b, a, input, open, switchTo } = await setup(page, request, info);
  await page.evaluate(() => {
    localStorage.setItem('piclaw_theme', 'tango'); localStorage.setItem('piclaw_tint', '');
    localStorage.setItem('piclaw_chat_themes', JSON.stringify({ 'web:default': { theme: 'ristretto', tint: null } }));
  });
  await page.reload(); await expect(input).toHaveValue('settings draft');
  const legacy = () => page.evaluate(() => ['piclaw_theme', 'piclaw_tint', 'piclaw_chat_themes'].map(k => localStorage.getItem(k)));
  const oldLegacy = await legacy();
  const mutations = []; page.on('request', r => { if (['POST', 'PATCH', 'PUT', 'DELETE'].includes(r.method()) && !r.url().includes('/frontend/log')) mutations.push(r.url()); });
  await open(); const { dialog, radio, tint } = await appearance(page);
  await expect(dialog.getByText(/^Browser settings · this origin, across all sessions/)).toBeVisible();
  await radio('default').check(); await tint('#aabbcc');
  await expect.poll(async () => (await appearanceSnapshot(page)).tint).toBe('#aabbcc');
  let state = await appearanceSnapshot(page); expect(state.theme).toBe('default'); expect(state.meta).toBeTruthy();
  expect(JSON.parse(state.saved)).toEqual({ version: 1, theme: 'default', tint: '#aabbcc', outputPad: 0 });
  const tinted = state.background;
  await radio('monokai').check();
  await expect.poll(async () => (await appearanceSnapshot(page)).theme).toBe('monokai');
  state = await appearanceSnapshot(page); expect(state.tint).toBe(''); expect(state.background).not.toBe(tinted);
  await page.keyboard.press('Escape'); await switchTo(b); expect((await appearanceSnapshot(page)).theme).toBe('monokai');
  await switchTo(a); await expect(input).toHaveValue('settings draft');
  await page.reload(); await expect(input).toHaveValue('settings draft'); expect((await appearanceSnapshot(page)).theme).toBe('monokai');
  expect(await legacy()).toEqual(oldLegacy); expect(mutations).toEqual([]);
  await open(); const controls = await appearance(page); await controls.radio('default').check(); await controls.tint('#aabbcc');
  await expect.poll(async () => (await appearanceSnapshot(page)).background).toBe(tinted);
  await controls.clearTint.click(); await expect.poll(async () => (await appearanceSnapshot(page)).tint).toBe('');
  if (process.env.GI_SETTINGS_CAPTURE && info.project.name.startsWith('chromium-')) {
    mkdirSync('test-results/gi-settings-captures', { recursive: true });
    await page.screenshot({ path: `test-results/gi-settings-captures/appearance-${info.project.name}.png` });
  }
});

test('@gi-settings-010 Denied local storage cannot claim success or change theme', async ({ page, request }, info) => {
  const { input, open } = await setup(page, request, info); await open();
  const { dialog, radio, padding } = await appearance(page); await radio('default').check();
  const before = await appearanceSnapshot(page);
  await expect(padding).toHaveAttribute('min', '0'); await expect(padding).toHaveAttribute('max', '24');
  await page.evaluate(() => {
    const original = Storage.prototype.setItem;
    window.__restoreAppearanceStorage = () => { Storage.prototype.setItem = original; };
    Storage.prototype.setItem = function(key, value) { if (key === 'gi_browser_appearance_v1') throw new DOMException('Appearance storage denied', 'QuotaExceededError'); return original.call(this, key, value); };
  });
  try {
    await radio('monokai').click(); await expect(dialog.getByRole('alert')).toContainText('Appearance storage denied');
    await expect(radio('default')).toBeChecked();
    expect(await appearanceSnapshot(page)).toEqual(before);
  } finally { await page.evaluate(() => window.__restoreAppearanceStorage()); }
  await radio('monokai').check(); await expect.poll(async () => (await appearanceSnapshot(page)).theme).toBe('monokai');
  await expect(dialog.getByRole('alert')).toHaveCount(0);
  await page.keyboard.press('Escape'); await expect(input).toHaveValue('settings draft');
});

test('@gi-settings-011 Default follows system mode and cross-tab updates apply in every tab', async ({ page, request, context }, info) => {
  const { a, input, open } = await setup(page, request, info);
  await page.evaluate(() => localStorage.setItem('gi_browser_appearance_v1', '{invalid json'));
  await page.reload(); await expect(input).toHaveValue('settings draft'); await open();
  const controls = await appearance(page); await controls.radio('monokai').check();
  const other = await context.newPage();
  try {
    await other.goto('/'); await expect(other.getByRole('textbox', { name: inputName, exact: true })).toBeVisible();
    expect((await appearanceSnapshot(other)).theme).toBe('monokai');
    await other.keyboard.press('Control+,'); const otherControls = await appearance(other);
    await otherControls.radio('ristretto').check();
    await expect.poll(async () => (await appearanceSnapshot(page)).theme).toBe('ristretto');
    await expect(controls.radio('ristretto')).toBeChecked();
    await page.emulateMedia({ colorScheme: 'dark' }); await controls.radio('default').check();
    await expect.poll(async () => (await appearanceSnapshot(other)).theme).toBe('default');
    const state = await appearanceSnapshot(page); expect(state.mode).toBe('dark'); expect(state.tint).toBe('');
    expect(JSON.parse(state.saved)).toEqual({ version: 1, theme: 'default', tint: '', outputPad: 0 });
    await page.emulateMedia({ colorScheme: 'light' }); await expect.poll(async () => (await appearanceSnapshot(page)).mode).toBe('light');
    await page.keyboard.press('Escape'); await expect(input).toHaveValue('settings draft');
    expect(await page.evaluate(() => localStorage.getItem('gi_session_id'))).toBe(a);
  } finally { await other.close(); }
});

test('@gi-settings-012 Explicit identity save persists but active names require restart', async ({ page, request }, info) => {
  const { a, input, open, dialog } = await setup(page, request, info);
  const previous = await (await request.get('/api/settings/identity')).json();
  const restore = async () => { const latest = await (await request.get('/api/settings/identity')).json(); expect((await request.patch('/api/settings/identity', { data: { ...previous.saved, revision: latest.saved.revision } })).status()).toBe(200); };
  try {
    await open(); const assistant = dialog.getByLabel('Assistant display name'), user = dialog.getByLabel('User display name');
    await expect(assistant).toHaveValue(previous.saved.assistant_name);
    const name = `Gi ${info.project.name}`, userName = 'Settings Reader';
    await assistant.fill(name); await user.fill(userName);
    expect((await (await request.get('/api/settings/identity')).json()).saved).toEqual(previous.saved);
    let release, held = false; const gate = new Promise(resolve => { release = resolve; });
    await page.route('**/api/settings/identity', async route => {
      if (route.request().method() !== 'PATCH') return route.continue();
      const response = await route.fetch(); held = true; await gate; await route.fulfill({ response });
    });
    try {
      await dialog.getByRole('button', { name: 'Save names', exact: true }).click(); await expect.poll(() => held).toBe(true);
      await expect(dialog.getByRole('button', { name: 'Saving names…' })).toBeDisabled(); await expect(assistant).toBeDisabled();
      await expect(dialog.getByTestId('identity-restart-required')).toHaveCount(previous.restart_required ? 1 : 0);
      release(); await expect(dialog.getByTestId('identity-restart-required')).toBeVisible();
      await expect(dialog.getByRole('status')).toContainText('Restart Gi manually');
    } finally { release(); }
    await page.unroute('**/api/settings/identity');
    const saved = await (await request.get('/api/settings/identity')).json();
    expect(saved.saved.assistant_name).toBe(name); expect(saved.saved.user_name).toBe(userName); expect(saved.active).toEqual(previous.active);
    await expect(dialog.locator('.gi-settings-values')).toContainText(previous.active.assistant_name);
    await page.keyboard.press('Escape'); await expect(input).toHaveValue('settings draft');
    await page.reload(); await expect(input).toHaveValue('settings draft'); await open(); await expect(assistant).toHaveValue(name);
    await expect(dialog.getByTestId('identity-restart-required')).toBeVisible(); expect(await page.evaluate(() => localStorage.getItem('gi_session_id'))).toBe(a);
    if (process.env.GI_SETTINGS_CAPTURE && info.project.name.startsWith('chromium-')) {
      mkdirSync('test-results/gi-settings-captures', { recursive: true }); await page.screenshot({ path: `test-results/gi-settings-captures/identity-${info.project.name}.png` });
    }
  } finally { await restore(); }
});

test('@gi-settings-013 Conflict, validation and failed identity writes retain the draft', async ({ page, request }, info) => {
  const { input, open, dialog } = await setup(page, request, info);
  const previous = await (await request.get('/api/settings/identity')).json();
  try {
    await open(); const assistant = dialog.getByLabel('Assistant display name'); await expect(assistant).toHaveValue(previous.saved.assistant_name);
    await assistant.fill('My unsaved name');
    expect((await request.patch('/api/settings/identity', { data: { ...previous.saved, assistant_name: 'Other writer' } })).status()).toBe(200);
    await dialog.getByRole('button', { name: 'Save names', exact: true }).click(); await expect(dialog.getByRole('alert')).toContainText('configuration changed');
    await expect(assistant).toHaveValue('My unsaved name');
    await dialog.getByRole('button', { name: 'Reload saved names' }).click(); await expect(assistant).toHaveValue('Other writer');
    await assistant.fill(''); await dialog.getByRole('button', { name: 'Save names', exact: true }).click(); await expect(dialog.getByRole('alert')).toContainText('1–128');
    expect((await (await request.get('/api/settings/identity')).json()).saved.assistant_name).toBe('Other writer');
    await assistant.fill('Retry name');
    // Native storage failure: the isolated test workspace's lock path becomes nonregular.
    const config = await (await request.get('/api/runtime/config')).json();
    const { join } = await import('node:path'); const { rmSync } = await import('node:fs');
    const lock = join(config.workspace_root, '.piclaw', '.gi-identity.lock');
    // Never use this fault against an operator workspace.
    expect(config.workspace_root.replaceAll('\\', '/')).toContain('/.gi-ux-parity/workspace');
    rmSync(lock, { force: true }); mkdirSync(lock);
    try {
      await dialog.getByRole('button', { name: 'Save names', exact: true }).click(); await expect(dialog.getByRole('alert')).toContainText('Cannot save identity');
      await expect(assistant).toHaveValue('Retry name'); expect((await (await request.get('/api/settings/identity')).json()).saved.assistant_name).toBe('Other writer');
    } finally { rmSync(lock, { recursive: true, force: true }); }
    await dialog.getByRole('button', { name: 'Save names', exact: true }).click(); await expect(dialog.getByRole('status')).toContainText('Names saved');
    await page.keyboard.press('Escape'); await expect(input).toHaveValue('settings draft');
  } finally {
    const latest = await (await request.get('/api/settings/identity')).json();
    expect((await request.patch('/api/settings/identity', { data: { ...previous.saved, revision: latest.saved.revision } })).status()).toBe(200);
  }
});

test('@gi-settings-012 Closing a pending identity save does not announce success in a new view', async ({ page, request }, info) => {
  const { b, input, open, switchTo, dialog } = await setup(page, request, info);
  const previous = await (await request.get('/api/settings/identity')).json();
  let release, held = false, done; const gate = new Promise(resolve => { release = resolve; }), delivered = new Promise(resolve => { done = resolve; });
  await page.route('**/api/settings/identity', async route => {
    if (route.request().method() !== 'PATCH') return route.continue();
    const response = await route.fetch(); held = true; await gate; await route.fulfill({ response }); done();
  });
  try {
    await open(); const assistant = dialog.getByLabel('Assistant display name'); await expect(assistant).toHaveValue(previous.saved.assistant_name);
    await assistant.fill('Saved while closing'); await dialog.getByRole('button', { name: 'Save names', exact: true }).click(); await expect.poll(() => held).toBe(true);
    await page.keyboard.press('Escape'); await switchTo(b); await input.fill('new view draft'); await open();
    await expect(assistant).toHaveValue('Saved while closing');
    release(); await delivered;
    await expect(dialog.getByRole('status').filter({ hasText: 'Names saved' })).toHaveCount(0);
    await expect(dialog.locator('.gi-settings-values')).toContainText(previous.active.assistant_name);
    await page.keyboard.press('Escape'); await expect(input).toHaveValue('new view draft');
  } finally {
    release();
    const latest = await (await request.get('/api/settings/identity')).json();
    expect((await request.patch('/api/settings/identity', { data: { ...previous.saved, revision: latest.saved.revision } })).status()).toBe(200);
  }
});

const policyBody = saved => ({ revision: saved.revision, enabled: saved.policy.enabled, context_window: saved.policy.context_window, reserve_tokens: saved.policy.reserve_tokens, keep_recent_tokens: saved.policy.keep_recent_tokens, threshold_tokens: saved.policy.threshold_tokens });
async function savedPolicyControls(page) {
  const dialog = dialogFor(page); await dialog.getByRole('button', { name: 'Compaction', exact: true }).click();
  const region = dialog.getByRole('region', { name: 'Saved automatic policy' });
  await expect(region.getByLabel('Saved context window')).toBeVisible(); return { dialog, region };
}

test('@gi-settings-018 Explicit policy save retains active policy until restart and survives reload', async ({ page, request }, info) => {
  const { input, open } = await setup(page, request, info);
  const before = await (await request.get('/api/settings/compaction')).json();
  try {
    await open(); const { dialog, region } = await savedPolicyControls(page);
    await region.getByLabel('Saved automatic compaction').setChecked(!before.saved.policy.enabled);
    await region.getByLabel('Saved context window').fill('128000');
    await region.getByLabel('Saved reserved tokens').fill('10000');
    await region.getByLabel('Saved keep recent tokens').fill('12000');
    await region.getByLabel('Saved trigger threshold').fill('90000');
    expect((await (await request.get('/api/settings/compaction')).json()).saved).toEqual(before.saved);
    let release, held = false; const gate = new Promise(resolve => { release = resolve; });
    await page.route('**/api/settings/compaction', async route => {
      if (route.request().method() !== 'PATCH') return route.continue();
      const response = await route.fetch(); expect(response.status()).toBe(200); held = true; await gate; await route.fulfill({ response });
    });
    try {
      await region.getByRole('button', { name: 'Save policy', exact: true }).click(); await expect.poll(() => held).toBe(true);
      await expect(region.getByRole('button', { name: 'Saving policy…' })).toBeDisabled();
      await expect(region.getByLabel('Saved trigger threshold')).toBeDisabled();
      release(); await expect(region.getByRole('status')).toContainText('Restart Gi manually');
    } finally { release(); }
    await page.unroute('**/api/settings/compaction');
    const saved = await (await request.get('/api/settings/compaction')).json();
    expect(saved.active).toEqual(before.active); expect(saved.saved.policy.threshold_tokens).toBe(90000); expect(saved.restart_required).toBe(true);
    await expect(dialog.getByTestId('compaction-policy')).toContainText(String(before.active.threshold_tokens));
    await page.keyboard.press('Escape'); await expect(input).toHaveValue('settings draft');
    await page.reload(); await expect(input).toHaveValue('settings draft'); await open(); const next = await savedPolicyControls(page);
    await expect(next.region.getByLabel('Saved trigger threshold')).toHaveValue('90000'); await expect(next.region.getByTestId('compaction-policy-restart')).toBeVisible();
    if (process.env.GI_SETTINGS_CAPTURE && info.project.name.startsWith('chromium-')) {
      mkdirSync('test-results/gi-settings-captures', { recursive: true }); await next.region.scrollIntoViewIfNeeded(); await page.screenshot({ path: `test-results/gi-settings-captures/policy-${info.project.name}.png` });
    }
  } finally {
    const latest = await (await request.get('/api/settings/compaction')).json();
    expect((await request.patch('/api/settings/compaction', { data: { ...policyBody(before.saved), revision: latest.saved.revision } })).status()).toBe(200);
  }
});

test('@gi-settings-019 Policy validation, conflict and native write failure preserve unsaved fields', async ({ page, request }, info) => {
  const { input, open } = await setup(page, request, info);
  const before = await (await request.get('/api/settings/compaction')).json();
  try {
    await open(); const { region } = await savedPolicyControls(page);
    const threshold = region.getByLabel('Saved trigger threshold'); await threshold.fill('1.5');
    await region.getByRole('button', { name: 'Save policy', exact: true }).click(); await expect(region.getByRole('alert')).toContainText('whole-number');
    expect((await (await request.get('/api/settings/compaction')).json()).saved).toEqual(before.saved);
    await threshold.fill('80000');
    expect((await request.patch('/api/settings/compaction', { data: { ...policyBody(before.saved), threshold_tokens: 85000 } })).status()).toBe(200);
    await region.getByRole('button', { name: 'Save policy', exact: true }).click(); await expect(region.getByRole('alert')).toContainText('Pi settings changed'); await expect(threshold).toHaveValue('80000');
    await region.getByRole('button', { name: 'Reload saved policy' }).click(); await expect(threshold).toHaveValue('85000'); await threshold.fill('80000');
    const cfg = await (await request.get('/api/runtime/config')).json(); expect(cfg.workspace_root.replaceAll('\\', '/')).toContain('/.gi-ux-parity/workspace');
    const { join } = await import('node:path'), { rmSync } = await import('node:fs'); const lock = join(cfg.workspace_root, '.pi', '.gi-settings.lock');
    rmSync(lock, { force: true }); mkdirSync(lock);
    try {
      await region.getByRole('button', { name: 'Save policy', exact: true }).click(); await expect(region.getByRole('alert')).toContainText('Cannot save compaction policy');
      await expect(threshold).toHaveValue('80000'); expect((await (await request.get('/api/settings/compaction')).json()).saved.policy.threshold_tokens).toBe(85000);
    } finally { rmSync(lock, { recursive: true, force: true }); }
    await region.getByRole('button', { name: 'Save policy', exact: true }).click(); await expect(region.getByRole('status')).toContainText('Policy saved');
    await page.keyboard.press('Escape'); await expect(input).toHaveValue('settings draft');
  } finally {
    const latest = await (await request.get('/api/settings/compaction')).json();
    expect((await request.patch('/api/settings/compaction', { data: { ...policyBody(before.saved), revision: latest.saved.revision } })).status()).toBe(200);
  }
});

test('@gi-settings-019 A closed policy save cannot announce success in a new session view', async ({ page, request }, info) => {
  const { b, input, open, switchTo } = await setup(page, request, info);
  const before = await (await request.get('/api/settings/compaction')).json();
  let release, held = false, done; const gate = new Promise(resolve => { release = resolve; }), delivered = new Promise(resolve => { done = resolve; });
  await page.route('**/api/settings/compaction', async route => {
    if (route.request().method() !== 'PATCH') return route.continue(); const response = await route.fetch(); held = true; await gate; await route.fulfill({ response }); done();
  });
  try {
    await open(); const { region } = await savedPolicyControls(page); await region.getByLabel('Saved trigger threshold').fill('85000');
    await region.getByRole('button', { name: 'Save policy', exact: true }).click(); await expect.poll(() => held).toBe(true);
    await page.keyboard.press('Escape'); await switchTo(b); await input.fill('policy target draft'); await open(); const reopened = await savedPolicyControls(page);
    await expect(reopened.region.getByLabel('Saved trigger threshold')).toHaveValue('85000'); release(); await delivered;
    await expect(reopened.region.getByRole('status').filter({ hasText: 'Policy saved' })).toHaveCount(0);
    await expect(reopened.dialog.getByTestId('compaction-policy')).toContainText(String(before.active.threshold_tokens));
    await page.keyboard.press('Escape'); await expect(input).toHaveValue('policy target draft');
  } finally {
    release(); const latest = await (await request.get('/api/settings/compaction')).json();
    expect((await request.patch('/api/settings/compaction', { data: { ...policyBody(before.saved), revision: latest.saved.revision } })).status()).toBe(200);
  }
});

test('@gi-settings-026 A commit after pane re-entry refreshes native state without replacing newer drafts', async ({ page, request }, info) => {
  const { a, b, input, open, models, dialog } = await setup(page, request, info);
  await open(); await models();
  let release, entered = false; const gate = new Promise(resolve => { release = resolve; }); let reads = 0;
  await page.route(`**/api/sessions/${a}/model`, async route => {
    if (route.request().method() === 'GET') { reads++; return route.continue(); }
    entered = true; await gate; await route.continue(); // Hold before native commit.
  });
  try {
    await dialog.getByLabel('Session model', { exact: true }).selectOption('test/bootstrap');
    await dialog.getByRole('button', { name: 'Apply model', exact: true }).click(); await expect.poll(() => entered).toBe(true);
    await page.keyboard.press('Escape'); await open(); await models();
    await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/test-model');
    const filter = dialog.getByLabel('Filter models', { exact: true }), select = dialog.getByLabel('Session model', { exact: true });
    await filter.fill('test/'); await select.selectOption('test/unavailable-model');
    const before = reads; release();
    await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/bootstrap'); expect(reads).toBeGreaterThan(before);
    await expect(filter).toHaveValue('test/'); await expect(select).toHaveValue('test/unavailable-model');
    await expect(dialog.getByText('Model applied to this session.', { exact: true })).toHaveCount(0);
    await expect(dialog.getByRole('alert')).toHaveCount(0);
    expect((await (await request.get(`/api/sessions/${b}/model`)).json()).current).toBe('test/test-model');
    await page.keyboard.press('Escape'); await expect(input).toHaveValue('settings draft');
  } finally { release(); }
});

test('@gi-settings-026 Settlement fences an older held catalogue while a re-entered pane is loading', async ({ page, request }, info) => {
  const { a, open, models, dialog } = await setup(page, request, info); await open(); await models();
  let releaseWrite, releaseRead, writing = false, held = false, stale = false, done;
  const writeGate = new Promise(r => { releaseWrite = r; }), readGate = new Promise(r => { releaseRead = r; }), delivered = new Promise(r => { done = r; });
  await page.route(`**/api/sessions/${a}/model`, async route => {
    if (route.request().method() === 'PATCH') { writing = true; await writeGate; return route.continue(); }
    if (stale && !held) { const response = await route.fetch(); held = true; await readGate; await route.fulfill({ response }); done(); return; }
    return route.continue();
  });
  try {
    await dialog.getByLabel('Session model', { exact: true }).selectOption('test/bootstrap'); await dialog.getByRole('button', { name: 'Apply model', exact: true }).click(); await expect.poll(() => writing).toBe(true);
    await dialog.getByRole('button', { name: 'General', exact: true }).click(); stale = true;
    await dialog.getByRole('button', { name: 'Models', exact: true }).click(); await expect.poll(() => held).toBe(true);
    await dialog.getByLabel('Filter models', { exact: true }).fill('bootstrap'); releaseWrite();
    await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/bootstrap');
    releaseRead(); await delivered; await page.waitForTimeout(100);
    await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/bootstrap');
    await expect(dialog.getByLabel('Filter models', { exact: true })).toHaveValue('bootstrap');
    await expect(dialog.getByText('Model applied to this session.', { exact: true })).toHaveCount(0);
  } finally { releaseWrite(); releaseRead(); }
});

test('@gi-settings-027 Lost accepted response and failed reread retain action errors until explicit recovery', async ({ page, request }, info) => {
  const { a, open, models, dialog } = await setup(page, request, info); await open(); await models();
  let failRead = false;
  await page.route(`**/api/sessions/${a}/model`, async route => {
    if (route.request().method() === 'PATCH') { const response = await route.fetch(); expect(response.status()).toBe(200); failRead = true; return route.abort(); }
    if (failRead) return route.abort();
    return route.continue();
  });
  await dialog.getByLabel('Session model', { exact: true }).selectOption('test/bootstrap'); await dialog.getByRole('button', { name: 'Apply model', exact: true }).click();
  await expect(dialog.getByRole('alert')).toHaveCount(2);
  await expect(dialog.getByRole('button', { name: 'Apply model', exact: true })).toBeDisabled();
  expect((await (await request.get(`/api/sessions/${a}/model`)).json()).current).toBe('test/bootstrap');
  await dialog.getByLabel('Filter models', { exact: true }).fill('test/');
  await dialog.getByLabel('Session model', { exact: true }).selectOption('test/unavailable-model');
  failRead = false; await dialog.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(dialog.getByTestId('settings-current-model')).toHaveText('test/bootstrap');
  await expect(dialog.getByLabel('Session model', { exact: true })).toHaveValue('test/unavailable-model');
  await expect(dialog.getByLabel('Filter models', { exact: true })).toHaveValue('test/');
  await expect(dialog.getByRole('alert')).toHaveCount(1);
  await expect(dialog.getByText('Model applied to this session.', { exact: true })).toHaveCount(0);
  await expect(dialog.getByRole('button', { name: 'Apply model', exact: true })).toBeEnabled();
});
