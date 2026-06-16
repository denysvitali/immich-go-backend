import { expect, test } from '@playwright/test';

const email = `e2e-${Date.now()}@example.com`;
const password = 'E2ePassword123!';
const name = 'E2E Admin';

test('upstream frontend can reach backend through its proxy', async ({ page, request }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/Immich/i);

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
  const signup = await request.post('/api/auth/admin-sign-up', {
    data: { email, password, name },
  });
  expect(signup.ok()).toBeTruthy();
  const signupBody = await signup.json();
  expect(signupBody.accessToken).toBeTruthy();
  expect(signupBody.userEmail).toBe(email);

  const login = await request.post('/api/auth/login', {
    data: { email, password },
  });
  expect(login.ok()).toBeTruthy();
  const loginBody = await login.json();
  expect(loginBody.accessToken).toBeTruthy();

  const headers = {
    Authorization: `Bearer ${loginBody.accessToken}`,
  };

  const me = await request.get('/api/users/me', { headers });
  expect(me.ok()).toBeTruthy();
  const meBody = await me.json();
  expect(meBody.email).toBe(email);
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
  expect(await emptyLicense.json()).toEqual({});
});
