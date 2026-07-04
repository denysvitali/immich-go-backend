import { expect, test } from '@playwright/test';

import { authenticatePage, expectOk, signUpAdmin, uniqueId, uploadAsset } from './helpers';

const assetMonthLabel = 'Jun 2026';
const assetTimeBucket = '2026-06-01';
const assetTimestamp = '2026-06-01T12:00:00Z';

test('user can log in through the login form and reach the timeline', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-login', 'E2E UI User');

  await page.goto('/auth/login');
  await page.locator('#email').fill(user.email);
  await page.locator('#password').fill(user.password);
  await page.getByRole('button', { name: 'Login' }).click();

  await expect(page).toHaveURL(/\/photos/, { timeout: 15_000 });
  await expect(page.locator('#stencil')).toBeHidden();
});

test('uploaded asset appears on the timeline', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-timeline', 'E2E UI User');
  await uploadAsset(request, user, assetTimestamp);

  await authenticatePage(page, user);
  await page.goto('/photos');
  await expect(page.getByText(assetMonthLabel).first()).toBeVisible({ timeout: 15_000 });
});

test('album created via API shows up on the albums page', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-albums', 'E2E UI User');
  const asset = await uploadAsset(request, user);

  const albumName = `UI Album ${uniqueId('album')}`;
  const album = await request.post('/api/albums', {
    headers: user.headers,
    data: { albumName, description: 'UI E2E album', assetIds: [asset.id] },
  });
  await expectOk(album);
  const albumBody = await album.json();

  await authenticatePage(page, user);
  await page.goto('/albums');
  await expect(page.getByText(albumName).first()).toBeVisible({ timeout: 15_000 });

  await page.goto(`/albums/${albumBody.id}`);
  await expect(page.getByRole('textbox', { name: 'Edit Title' })).toHaveValue(albumName, { timeout: 15_000 });
});

test('favorited asset appears on the favorites page', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-favorites', 'E2E UI User');
  const asset = await uploadAsset(request, user, assetTimestamp);

  const favorite = await request.put(`/api/assets/${asset.id}`, {
    headers: user.headers,
    data: { isFavorite: true },
  });
  await expectOk(favorite);

  await authenticatePage(page, user);
  await page.goto('/favorites');
  await expect(page.getByText(assetMonthLabel).first()).toBeVisible({ timeout: 15_000 });
});

test('trashed asset is exposed through the trash timeline and page renders', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-trash', 'E2E UI User');
  const asset = await uploadAsset(request, user, assetTimestamp);

  const deleted = await request.delete('/api/assets', {
    headers: user.headers,
    data: { ids: [asset.id] },
  });
  await expectOk(deleted);

  const buckets = await request.get('/api/timeline/buckets?isTrashed=true', { headers: user.headers });
  await expectOk(buckets);
  const bucketsBody = (await buckets.json()) as Array<{ timeBucket: string; count: number }>;
  const trashedBucket = bucketsBody.find((bucket) => bucket.timeBucket === assetTimeBucket);
  expect(trashedBucket).toMatchObject({ timeBucket: assetTimeBucket, count: expect.any(Number) });
  expect(trashedBucket?.count).toBeGreaterThanOrEqual(1);

  const bucket = await request.get(`/api/timeline/bucket?timeBucket=${assetTimeBucket}&isTrashed=true`, {
    headers: user.headers,
  });
  await expectOk(bucket);
  const bucketBody = await bucket.json();
  expect(Array.isArray(bucketBody.id)).toBe(true);
  expect(bucketBody.id).toContain(asset.id);

  await authenticatePage(page, user);
  await page.goto('/trash');
  await expect(page.getByText('Trashed items will be permanently deleted after 30 days.')).toBeVisible({
    timeout: 15_000,
  });
  await expect(page.getByRole('button', { name: 'Restore all' })).toBeVisible();
});

test('authenticated navigation pages render without redirecting to login', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-nav', 'E2E UI User');
  await uploadAsset(request, user);
  await authenticatePage(page, user);

  for (const path of ['/explore', '/people', '/sharing', '/archive', '/utilities', '/user-settings']) {
    await page.goto(path);
    await expect(page.locator('#stencil'), `stencil stuck on ${path}`).toBeHidden({ timeout: 15_000 });
    expect(page.url(), `redirected to login from ${path}`).not.toContain('/auth/login');
  }
});
