import { expect, type APIRequestContext, type APIResponse, type Page } from '@playwright/test';

export const password = 'E2ePassword123!';
const rootAdminEmail = 'e2e-root-admin@example.com';

let userCounter = 0;
let rootAdmin: TestUser | undefined;

export const png1x1 = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=',
  'base64',
);

export type TestUser = {
  email: string;
  password: string;
  userId: string;
  isAdmin: boolean;
  accessToken: string;
  headers: Record<string, string>;
};

export type AuthResponse = {
  accessToken: string;
  isAdmin: boolean;
  userEmail?: string;
  userId?: string;
  [key: string]: unknown;
};

export type LoginSession = {
  accessToken: string;
  headers: Record<string, string>;
  body: AuthResponse;
};

export type AssetResponse = {
  id: string;
  ownerId: string;
  deviceAssetId: string;
  originalFileName: string;
  originalPath: string;
};

export function uniqueId(prefix: string) {
  userCounter += 1;
  return `${prefix}-${Date.now()}-${userCounter}`;
}

export function uniqueEmail(prefix: string) {
  return `${uniqueId(prefix)}@example.com`;
}

export async function expectOk(response: APIResponse) {
  if (!response.ok()) {
    throw new Error(`${response.url()} returned ${response.status()}: ${await response.text()}`);
  }
}

function authHeaders(accessToken: string) {
  return { Authorization: `Bearer ${accessToken}` };
}

function userFromAuthResponse(email: string, body: AuthResponse): TestUser {
  expect(body.accessToken).toBeTruthy();
  expect(body.userId).toBeTruthy();
  expect(typeof body.isAdmin).toBe('boolean');

  return {
    email,
    password,
    userId: String(body.userId),
    isAdmin: body.isAdmin,
    accessToken: body.accessToken,
    headers: authHeaders(body.accessToken),
  };
}

async function tryLoginRootAdmin(request: APIRequestContext): Promise<TestUser | undefined> {
  const response = await request.post('/api/auth/login', {
    data: { email: rootAdminEmail, password },
  });
  if (!response.ok()) {
    return undefined;
  }

  const user = userFromAuthResponse(rootAdminEmail, (await response.json()) as AuthResponse);
  if (!user.isAdmin) {
    throw new Error('e2e root admin account exists but is not an admin');
  }
  rootAdmin = user;
  return user;
}

async function ensureRootAdmin(request: APIRequestContext): Promise<TestUser> {
  if (rootAdmin) {
    return rootAdmin;
  }

  const existing = await tryLoginRootAdmin(request);
  if (existing) {
    return existing;
  }

  const signup = await request.post('/api/auth/admin-sign-up', {
    data: { email: rootAdminEmail, password, name: 'E2E Root Admin' },
  });

  if (!signup.ok()) {
    const retry = await tryLoginRootAdmin(request);
    if (retry) {
      return retry;
    }
    throw new Error(`${signup.url()} returned ${signup.status()}: ${await signup.text()}`);
  }

  const body = (await signup.json()) as AuthResponse;
  expect(body.userEmail).toBe(rootAdminEmail);
  const user = userFromAuthResponse(rootAdminEmail, body);
  if (!user.isAdmin) {
    throw new Error('e2e root admin sign-up returned a non-admin user');
  }

  rootAdmin = user;
  return user;
}

export async function signUpAdmin(
  request: APIRequestContext,
  prefix = 'e2e',
  name = 'E2E User',
): Promise<TestUser> {
  const admin = await ensureRootAdmin(request);
  const email = uniqueEmail(prefix);
  const signup = await request.post('/api/auth/admin-sign-up', {
    data: { email, password, name },
  });
  await expectOk(signup);

  const signupBody = await signup.json();
  expect(signupBody.accessToken).toBeTruthy();
  expect(signupBody.userEmail).toBe(email);
  expect(signupBody.userId).toBeTruthy();
  expect(typeof signupBody.isAdmin).toBe('boolean');

  const user = userFromAuthResponse(email, signupBody);

  if (user.isAdmin) {
    rootAdmin ??= user;
    return user;
  }

  const promoted = await request.put(`/api/admin/users/${user.userId}`, {
    headers: admin.headers,
    data: { isAdmin: true },
  });
  await expectOk(promoted);

  const session = await login(request, user);
  expect(session.body.isAdmin).toBe(true);

  return {
    ...user,
    isAdmin: true,
    accessToken: session.accessToken,
    headers: session.headers,
  };
}

export async function login(
  request: APIRequestContext,
  user: Pick<TestUser, 'email' | 'password'>,
): Promise<LoginSession> {
  const response = await request.post('/api/auth/login', {
    data: { email: user.email, password: user.password },
  });
  await expectOk(response);

  const body = (await response.json()) as AuthResponse;
  expect(body.accessToken).toBeTruthy();

  return {
    accessToken: body.accessToken,
    headers: authHeaders(body.accessToken),
    body,
  };
}

// The backend sets auth cookies with the Secure attribute; over plain http the
// APIRequestContext does not persist them into the browser context, so page
// navigations need the equivalent cookies planted explicitly.
export async function authenticatePage(page: Page, user: Pick<TestUser, 'accessToken'>) {
  const url = new URL(process.env.IMMICH_WEB_URL ?? 'http://127.0.0.1:3000');
  const cookie = { domain: url.hostname, path: '/' } as const;
  await page.context().addCookies([
    { ...cookie, name: 'immich_access_token', value: user.accessToken },
    { ...cookie, name: 'immich_auth_type', value: 'password' },
    { ...cookie, name: 'immich_is_authenticated', value: 'true' },
  ]);
}

export async function uploadAsset(
  request: APIRequestContext,
  user: Pick<TestUser, 'headers'>,
  fileCreatedAt = '2026-06-01T12:00:00Z',
): Promise<AssetResponse> {
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
