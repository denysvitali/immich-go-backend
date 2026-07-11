import { expect, test } from '@playwright/test';

import { authenticatePage, expectOk, jpegWithExifDate, signUpAdmin, uniqueId } from './helpers';

// Drives the real web upload flow (upload button → file chooser → multipart
// POST /api/assets). This is the only test that exercises the uploader's
// client-side extension filter, which is fed by GET /api/server/media-types:
// if that endpoint returns anything other than upstream-shaped extension
// lists, the web UI silently drops every picked file and no upload happens.
test('uploading a picture through the web UI creates the asset', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-upload', 'E2E Upload User');
  await authenticatePage(page, user);

  await page.goto('/photos');
  await expect(page.locator('#stencil')).toBeHidden({ timeout: 15_000 });

  const filename = `${uniqueId('ui-upload')}.jpg`;

  const chooserPromise = page.waitForEvent('filechooser');
  await page
    .getByRole('button', { name: /upload/i })
    .first()
    .click();
  const chooser = await chooserPromise;

  const uploadResponsePromise = page.waitForResponse(
    (response) => new URL(response.url()).pathname === '/api/assets' && response.request().method() === 'POST',
    { timeout: 20_000 },
  );
  await chooser.setFiles({
    name: filename,
    mimeType: 'image/jpeg',
    buffer: jpegWithExifDate('2017:07:22 22:14:34'),
  });

  const uploadResponse = await uploadResponsePromise;
  expect([200, 201]).toContain(uploadResponse.status());
  const uploadBody = await uploadResponse.json();
  expect(uploadBody.id).toBeTruthy();

  const fetched = await request.get(`/api/assets/${uploadBody.id}`, { headers: user.headers });
  await expectOk(fetched);
  const fetchedBody = await fetched.json();
  expect(fetchedBody.originalFileName).toBe(filename);
  expect(fetchedBody.ownerId).toBe(user.userId);

  await expect
    .poll(
      async () => {
        const buckets = await request.get('/api/timeline/buckets', { headers: user.headers });
        await expectOk(buckets);
        return ((await buckets.json()) as Array<{ timeBucket: string }>).some(
          ({ timeBucket }) => timeBucket === '2017-07-22',
        );
      },
      { timeout: 20_000 },
    )
    .toBe(true);

  await page.reload();
  await expect(page.getByText(/Sat, Jul 22/).first()).toBeVisible({ timeout: 15_000 });
});
