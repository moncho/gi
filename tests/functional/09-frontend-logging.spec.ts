/**
 * 09-frontend-logging.spec.ts — Verify the browser-to-backend log bridge.
 *
 * Tests that frontend errors and log entries are captured and forwarded
 * to the backend via the /api/frontend/log endpoint.
 */
import { test, expect } from '@playwright/test';
import { BASE_URL } from './helpers';

test.describe('Frontend logging', () => {

  test('frontend log endpoint accepts log entries', async ({ request }) => {
    const res = await request.post(`${BASE_URL}/api/frontend/log`, {
      data: {
        entries: [{
          ts: new Date().toISOString(),
          level: 'info',
          message: 'functional test log entry',
          detail: { test: true },
        }],
      },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.ok).toBeTruthy();
  });

  test('frontend log endpoint handles empty entries', async ({ request }) => {
    const res = await request.post(`${BASE_URL}/api/frontend/log`, {
      data: { entries: [] },
    });
    expect(res.ok()).toBeTruthy();
  });

  test('frontend log endpoint rejects invalid JSON', async ({ request }) => {
    const res = await request.post(`${BASE_URL}/api/frontend/log`, {
      data: 'not json',
      headers: { 'Content-Type': 'text/plain' },
    });
    expect(res.status()).toBe(400);
  });

  test('an uncaught browser error reaches the native frontend log endpoint', async ({ page }) => {
    await page.goto(BASE_URL);
    const sentinel = `audit-browser-error-${Date.now()}`;
    const delivered = page.waitForResponse(response =>
      response.url().endsWith('/api/frontend/log') &&
      response.request().method() === 'POST' &&
      response.request().postData()?.includes(sentinel) === true,
    );
    await page.evaluate(message => setTimeout(() => { throw new Error(message); }, 0), sentinel);
    const response = await delivered;
    expect(response.ok()).toBe(true);
    const payload = response.request().postDataJSON();
    expect(payload.entries).toHaveLength(1);
    expect(payload.entries[0]).toMatchObject({ level: 'error', message: `Uncaught Error: ${sentinel}` });
  });
});
