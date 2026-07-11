import { expect, test } from '@playwright/test';

import { authenticatePage, expectOk, jpegWithExifDate, png1x1, signUpAdmin, uniqueId } from './helpers';

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

  const filename = `${uniqueId('ui-upload')}.png`;

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
  await chooser.setFiles({ name: filename, mimeType: 'image/png', buffer: png1x1 });

  const uploadResponse = await uploadResponsePromise;
  expect([200, 201]).toContain(uploadResponse.status());
  const uploadBody = await uploadResponse.json();
  expect(uploadBody.id).toBeTruthy();

  const fetched = await request.get(`/api/assets/${uploadBody.id}`, { headers: user.headers });
  await expectOk(fetched);
  const fetchedBody = await fetched.json();
  expect(fetchedBody.originalFileName).toBe(filename);
  expect(fetchedBody.ownerId).toBe(user.userId);
});

test('EXIF date moves an uploaded picture into the matching timeline year', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'exif-timeline', 'E2E EXIF Timeline User');
  const filename = `${uniqueId('exif-timeline')}.jpg`;
  const uploaded = await request.post('/api/assets', {
    headers: user.headers,
    multipart: {
      assetData: {
        name: filename,
        mimeType: 'image/jpeg',
        buffer: jpegWithExifDate('2017:07:22 22:14:34'),
      },
      deviceAssetId: uniqueId('exif-asset'),
      deviceId: 'playwright-exif-e2e',
      fileCreatedAt: '2024-10-01T12:00:00.000Z',
      fileModifiedAt: '2024-10-01T12:00:00.000Z',
    },
  });
  expect(uploaded.status()).toBe(201);

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

  await authenticatePage(page, user);
  await page.goto('/photos');
  await expect(page.getByText('2017', { exact: true }).first()).toBeVisible({ timeout: 15_000 });
});
