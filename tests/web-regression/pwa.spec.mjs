import { test, expect } from '@playwright/test';

test('Gi static Apple touch and favicon fallback routes return their declared assets', async ({ request }) => {
  for (const path of ['/apple-touch-icon-180x180.png', '/apple-touch-icon-167x167.png', '/apple-touch-icon-152x152.png', '/apple-touch-icon.png']) {
    const response = await request.get(path);
    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('image/png');
    expect((await response.body()).subarray(0, 8)).toEqual(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]));
  }
  const favicon = await request.get('/favicon.ico');
  expect(favicon.status()).toBe(200);
  expect((await favicon.body()).length).toBeGreaterThan(0);
});
