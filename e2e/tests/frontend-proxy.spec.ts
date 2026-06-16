import { expect, test, type APIRequestContext, type APIResponse } from '@playwright/test';

const password = 'E2ePassword123!';
let userCounter = 0;

type TestUser = {
  email: string;
  password: string;
  userId: string;
  headers: Record<string, string>;
};

const png1x1 = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=',
  'base64',
);

function uniqueEmail(prefix: string) {
  userCounter += 1;
  return `${prefix}-${Date.now()}-${userCounter}@example.com`;
}

async function expectOk(response: APIResponse) {
  if (!response.ok()) {
    throw new Error(`${response.url()} returned ${response.status()}: ${await response.text()}`);
  }
}

async function signUp(request: APIRequestContext, prefix = 'e2e'): Promise<TestUser> {
  const email = uniqueEmail(prefix);
  const name = 'E2E User';
  const signup = await request.post('/api/auth/admin-sign-up', {
    data: { email, password, name },
  });
  await expectOk(signup);
  const signupBody = await signup.json();
  expect(signupBody.accessToken).toBeTruthy();
  expect(signupBody.userEmail).toBe(email);
  expect(signupBody.userId).toBeTruthy();

  return {
    email,
    password,
    userId: signupBody.userId,
    headers: {
      Authorization: `Bearer ${signupBody.accessToken}`,
    },
  };
}

test('upstream frontend can reach backend through its proxy', async ({ page, request }) => {
  const response = await page.goto('/');
  expect(response?.ok()).toBeTruthy();
  await expect(page.locator('body')).toBeVisible();

  const about = await request.get('/api/server/about');
  expect(about.ok()).toBeTruthy();
  const aboutBody = await about.json();
  expect(aboutBody.repository).toBe('denysvitali/immich-go-backend');

  const features = await request.get('/api/server/features');
  expect(features.ok()).toBeTruthy();
  const featuresBody = await features.json();
  expect(featuresBody.trash).toBe(true);
  expect(featuresBody.passwordLogin).toBe(true);
});

test('admin can sign up, log in, read profile, and manage license through frontend origin', async ({ request }) => {
  const user = await signUp(request, 'license');

  const login = await request.post('/api/auth/login', {
    data: { email: user.email, password: user.password },
  });
  await expectOk(login);
  const loginBody = await login.json();
  expect(loginBody.accessToken).toBeTruthy();

  const headers = {
    Authorization: `Bearer ${loginBody.accessToken}`,
  };

  const me = await request.get('/api/users/me', { headers });
  expect(me.ok()).toBeTruthy();
  const meBody = await me.json();
  expect(meBody.email).toBe(user.email);
  expect(meBody.isAdmin).toBe(true);

  const license = await request.put('/api/users/me/license', {
    headers,
    data: {
      activationKey: 'e2e-activation-key',
      licenseKey: 'e2e-license-key',
    },
  });
  expect(license.ok()).toBeTruthy();
  const licenseBody = await license.json();
  expect(licenseBody.activationKey).toBe('e2e-activation-key');
  expect(licenseBody.licenseKey).toBe('e2e-license-key');
  expect(licenseBody.activatedAt).toBeTruthy();

  const savedLicense = await request.get('/api/users/me/license', { headers });
  expect(savedLicense.ok()).toBeTruthy();
  const savedLicenseBody = await savedLicense.json();
  expect(savedLicenseBody.licenseKey).toBe('e2e-license-key');

  const deletedLicense = await request.delete('/api/users/me/license', { headers });
  expect(deletedLicense.ok()).toBeTruthy();

  const emptyLicense = await request.get('/api/users/me/license', { headers });
  expect(emptyLicense.ok()).toBeTruthy();
  const emptyLicenseBody = await emptyLicense.json();
  expect(emptyLicenseBody.licenseKey ?? '').toBe('');
  expect(emptyLicenseBody.activationKey ?? '').toBe('');
});

test('user can create, read, and delete a profile image through frontend origin', async ({ request }) => {
  const user = await signUp(request, 'profile-image');

  const created = await request.post('/api/users/profile-image', {
    headers: user.headers,
    data: {
      file: png1x1.toString('base64'),
    },
  });
  await expectOk(created);
  const createdBody = await created.json();
  expect(createdBody.userId).toBe(user.userId);
  expect(createdBody.profileImagePath).toBe(`profile/${user.userId}/avatar.png`);
  expect(createdBody.profileChangedAt).toBeTruthy();

  const me = await request.get('/api/users/me', { headers: user.headers });
  await expectOk(me);
  const meBody = await me.json();
  expect(meBody.profileImagePath).toBe(createdBody.profileImagePath);
  expect(meBody.profileChangedAt).toBeTruthy();

  const image = await request.get(`/api/users/${user.userId}/profile-image`, {
    headers: user.headers,
  });
  await expectOk(image);
  const imageBody = await image.json();
  expect(imageBody.contentType).toBe('image/png');
  expect(Buffer.from(imageBody.imageData, 'base64')).toEqual(png1x1);

  const deleted = await request.delete('/api/users/profile-image', { headers: user.headers });
  await expectOk(deleted);

  const missingImage = await request.get(`/api/users/${user.userId}/profile-image`, {
    headers: user.headers,
  });
  expect(missingImage.status()).toBe(404);
});

test('login failures are rate limited through frontend origin', async ({ request }) => {
  const email = uniqueEmail('rate-limit');
  const badCredentials = { email, password: 'WrongPassword123!' };

  for (let attempt = 0; attempt < 20; attempt += 1) {
    const failedLogin = await request.post('/api/auth/login', { data: badCredentials });
    expect(failedLogin.status()).toBe(401);
  }

  const rateLimitedLogin = await request.post('/api/auth/login', { data: badCredentials });
  expect(rateLimitedLogin.status()).toBe(429);
});
