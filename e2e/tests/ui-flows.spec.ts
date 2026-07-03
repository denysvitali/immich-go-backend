import { expect, test, type APIRequestContext, type APIResponse, type Page } from '@playwright/test';

const password = 'E2ePassword123!';
let userCounter = 0;

const png1x1 = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=',
  'base64',
);

type TestUser = {
  email: string;
  password: string;
  userId: string;
  accessToken: string;
  headers: Record<string, string>;
};

function uniqueId(prefix: string) {
  userCounter += 1;
  return `${prefix}-${Date.now()}-${userCounter}`;
}

async function expectOk(response: APIResponse) {
  if (!response.ok()) {
    throw new Error(`${response.url()} returned ${response.status()}: ${await response.text()}`);
  }
}

async function signUpAdmin(request: APIRequestContext, prefix: string): Promise<TestUser> {
  const email = `${uniqueId(prefix)}@example.com`;
  const signup = await request.post('/api/auth/admin-sign-up', {
    data: { email, password, name: 'E2E UI User' },
  });
  await expectOk(signup);
  const signupBody = await signup.json();

  const login = await request.post('/api/auth/login', {
    data: { email, password },
  });
  await expectOk(login);
  const loginBody = await login.json();

  return {
    email,
    password,
    userId: signupBody.userId,
    accessToken: loginBody.accessToken,
    headers: { Authorization: `Bearer ${loginBody.accessToken}` },
  };
}

// The backend sets auth cookies with the Secure attribute; over plain http the
// APIRequestContext does not persist them into the browser context, so we plant
// the same cookies explicitly to authenticate page navigations.
async function authenticatePage(page: Page, user: TestUser) {
  const url = new URL(process.env.IMMICH_WEB_URL ?? 'http://127.0.0.1:3000');
  const cookie = { domain: url.hostname, path: '/' } as const;
  await page.context().addCookies([
    { ...cookie, name: 'immich_access_token', value: user.accessToken },
    { ...cookie, name: 'immich_auth_type', value: 'password' },
    { ...cookie, name: 'immich_is_authenticated', value: 'true' },
  ]);
}

async function uploadAsset(request: APIRequestContext, user: TestUser, fileCreatedAt = '2026-06-01T12:00:00Z') {
  const mediaId = uniqueId('ui-asset');
  const filename = `${mediaId}.png`;
  const uploaded = await request.post('/api/assets', {
    headers: user.headers,
    data: {
      assetData: {
        deviceAssetId: mediaId,
        deviceId: 'playwright-ui-e2e',
        type: 'ASSET_TYPE_IMAGE',
        originalPath: `fallback/${filename}`,
        originalFileName: filename,
        fileCreatedAt,
        fileModifiedAt: fileCreatedAt,
      },
      checksum: `checksum-${mediaId}`,
      fileContent: png1x1.toString('base64'),
    },
  });
  await expectOk(uploaded);
  return uploaded.json();
}

test('user can log in through the login form and reach the timeline', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-login');

  await page.goto('/auth/login');
  await page.locator('#email').fill(user.email);
  await page.locator('#password').fill(user.password);
  await page.getByRole('button', { name: 'Login' }).click();

  await expect(page).toHaveURL(/\/photos/, { timeout: 15_000 });
  await expect(page.locator('#stencil')).toBeHidden();
});

test('uploaded asset appears on the timeline', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-timeline');
  const asset = await uploadAsset(request, user);

  await authenticatePage(page, user);
  await page.goto('/photos');
  await expect(page.locator(`[data-asset="${asset.id}"]`)).toBeVisible({ timeout: 15_000 });
});

test('album created via API shows up on the albums page', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-albums');
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
  await expect(page.getByText(albumName)).toBeVisible({ timeout: 15_000 });

  await page.goto(`/albums/${albumBody.id}`);
  await expect(page.getByText(albumName).first()).toBeVisible({ timeout: 15_000 });
});

test('favorited asset appears on the favorites page', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-favorites');
  const asset = await uploadAsset(request, user);

  const favorite = await request.put(`/api/assets/${asset.id}`, {
    headers: user.headers,
    data: { isFavorite: true },
  });
  await expectOk(favorite);

  await authenticatePage(page, user);
  await page.goto('/favorites');
  await expect(page.locator(`[data-asset="${asset.id}"]`)).toBeVisible({ timeout: 15_000 });
});

test('trashed asset appears on the trash page', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-trash');
  const asset = await uploadAsset(request, user);

  const deleted = await request.delete('/api/assets', {
    headers: user.headers,
    data: { ids: [asset.id] },
  });
  await expectOk(deleted);

  await authenticatePage(page, user);
  await page.goto('/trash');
  await expect(page.locator(`[data-asset="${asset.id}"]`)).toBeVisible({ timeout: 15_000 });
});

test('authenticated navigation pages render without redirecting to login', async ({ page, request }) => {
  const user = await signUpAdmin(request, 'ui-nav');
  await uploadAsset(request, user);
  await authenticatePage(page, user);

  for (const path of ['/explore', '/people', '/sharing', '/archive', '/utilities', '/user-settings']) {
    await page.goto(path);
    await expect(page.locator('#stencil'), `stencil stuck on ${path}`).toBeHidden({ timeout: 15_000 });
    expect(page.url(), `redirected to login from ${path}`).not.toContain('/auth/login');
  }
});
