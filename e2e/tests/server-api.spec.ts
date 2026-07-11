import { expect, test } from '@playwright/test';

import { expectOk, signUpAdmin } from './helpers';

test('server bootstrap endpoints expose upstream-compatible response shapes', async ({ request }) => {
  const ping = await request.get('/api/server/ping');
  await expectOk(ping);
  await expect(ping.json()).resolves.toEqual({ res: 'pong' });

  const about = await request.get('/api/server/about');
  await expectOk(about);
  const aboutBody = await about.json();
  expect(aboutBody).toMatchObject({
    repository: 'denysvitali/immich-go-backend',
    repositoryUrl: 'https://github.com/denysvitali/immich-go-backend',
    licensed: false,
  });
  expect(aboutBody.version).toEqual(expect.any(String));

  const config = await request.get('/api/server/config');
  await expectOk(config);
  const configBody = await config.json();
  expect(configBody).toMatchObject({
    loginPageMessage: expect.any(String),
    mapDarkStyleUrl: expect.stringContaining('dark.json'),
    mapLightStyleUrl: expect.stringContaining('light.json'),
    oauthButtonText: expect.any(String),
    trashDays: expect.any(Number),
    userDeleteDelay: expect.any(Number),
  });
  for (const key of ['isInitialized', 'isOnboarded', 'maintenanceMode', 'publicUsers']) {
    expect(typeof configBody[key], key).toBe('boolean');
  }

  const features = await request.get('/api/server/features');
  await expectOk(features);
  const featuresBody = await features.json();
  for (const key of [
    'configFile',
    'duplicateDetection',
    'email',
    'facialRecognition',
    'importFaces',
    'map',
    'oauth',
    'oauthAutoLaunch',
    'passwordLogin',
    'reverseGeocoding',
    'search',
    'sidecar',
    'smartSearch',
    'trash',
  ]) {
    expect(typeof featuresBody[key], key).toBe('boolean');
  }
  expect(featuresBody.passwordLogin).toBe(true);
  expect(featuresBody.trash).toBe(true);
});

test('server media, version, theme, and storage endpoints stay JSON compatible', async ({ request }) => {
  const mediaTypes = await request.get('/api/server/media-types');
  await expectOk(mediaTypes);
  const mediaTypesBody = await mediaTypes.json();
  // Extensions, not MIME types: the web uploader matches file names against
  // these entries, so `image/jpeg`-style values reject every upload.
  expect(mediaTypesBody.image).toContain('.jpg');
  expect(mediaTypesBody.image).toContain('.png');
  expect(mediaTypesBody.video).toContain('.mp4');
  expect(mediaTypesBody.sidecar).toContain('.xmp');

  const version = await request.get('/api/server/version');
  await expectOk(version);
  expect(await version.json()).toMatchObject({
    major: expect.any(Number),
    minor: expect.any(Number),
    patch: expect.any(Number),
  });

  const versionHistory = await request.get('/api/server/version-history');
  await expectOk(versionHistory);
  const versionHistoryBody = await versionHistory.json();
  expect(Array.isArray(versionHistoryBody)).toBe(true);
  expect(versionHistoryBody.length).toBeGreaterThan(0);
  expect(versionHistoryBody[0]).toMatchObject({
    id: expect.any(String),
    version: expect.any(String),
    createdAt: expect.any(String),
  });

  const theme = await request.get('/api/server/theme');
  await expectOk(theme);
  expect(await theme.json()).toMatchObject({ customCss: expect.any(String) });

  const storage = await request.get('/api/server/storage');
  await expectOk(storage);
  expect(await storage.json()).toMatchObject({
    diskAvailable: expect.any(String),
    diskAvailableRaw: expect.stringMatching(/^\d+$/),
    diskSize: expect.any(String),
    diskSizeRaw: expect.stringMatching(/^\d+$/),
    diskUsagePercentage: expect.any(Number),
    diskUse: expect.any(String),
    diskUseRaw: expect.stringMatching(/^\d+$/),
  });
});

test('authenticated server statistics expose numeric usage fields', async ({ request }) => {
  const user = await signUpAdmin(request, 'server-statistics');

  const statistics = await request.get('/api/server/statistics', { headers: user.headers });
  await expectOk(statistics);
  const statisticsBody = await statistics.json();

  expect(statisticsBody).toMatchObject({
    photos: expect.any(Number),
    videos: expect.any(Number),
    usage: expect.stringMatching(/^\d+$/),
    usagePhotos: expect.stringMatching(/^\d+$/),
    usageVideos: expect.stringMatching(/^\d+$/),
  });
  expect(Array.isArray(statisticsBody.usageByUser)).toBe(true);
});
