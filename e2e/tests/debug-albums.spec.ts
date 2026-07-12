import { expect, test } from '@playwright/test';

import { authenticatePage, expectOk, signUpAdmin, uniqueId, uploadAsset } from './helpers';

test('debug: albums page console errors', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'dbg-albums', 'E2E Debug User');
  const asset = await uploadAsset(request, user);

  const albumName = `Debug Album ${uniqueId('album')}`;
  const album = await request.post('/api/albums', {
    headers: user.headers,
    data: { albumName, description: 'debug', assetIds: [asset.id] },
  });
  await expectOk(album);

  page.on('pageerror', (e) => console.log('[pageerror]', String(e).slice(0, 500)));
  page.on('console', (m) => {
    if (m.type() === 'error') console.log('[console.error]', m.text().slice(0, 300));
  });

  await authenticatePage(page, user);
  await page.goto('/albums', { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(8000);
  const body = await page.locator('body').innerText().catch(() => '<no body>');
  console.log('body text (start):', JSON.stringify(body.slice(0, 300)));
  expect(true).toBe(true);
});
