import { expect, test } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expectOk, gpsJpeg, png1x1, signUpAdmin, uniqueId } from './helpers';

// Covers the upstream Immich v2.4.0 API surface added for the web UI:
// system-config CRUD, multipart asset upload, download info/archive and the
// jobs status endpoint.

type AlbumMapMarker = {
  id: string;
  lat: number;
  lon: number;
  city: string;
  state: string;
  country: string;
};

function delay(ms: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, ms));
}

function hasFFmpeg() {
  try {
    execFileSync('ffmpeg', ['-version'], { stdio: 'ignore' });
    execFileSync('ffprobe', ['-version'], { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

function generateTestMp4() {
  const dir = mkdtempSync(join(tmpdir(), 'immich-e2e-video-'));
  const output = join(dir, 'test.mp4');
  try {
    execFileSync(
      'ffmpeg',
      [
        '-hide_banner',
        '-loglevel',
        'error',
        '-f',
        'lavfi',
        '-i',
        'testsrc=duration=2:size=64x64:rate=10',
        '-f',
        'lavfi',
        '-i',
        'sine=frequency=1000:duration=2',
        '-shortest',
        '-c:v',
        'libx264',
        '-c:a',
        'aac',
        '-pix_fmt',
        'yuv420p',
        '-movflags',
        '+faststart',
        '-y',
        output,
      ],
      { stdio: 'pipe' },
    );
    return readFileSync(output);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

test.describe('system config', () => {
  test('GET /api/system-config returns the full upstream shape', async ({ request }) => {
    const admin = await signUpAdmin(request, 'sysconf');

    const response = await request.get('/api/system-config', { headers: admin.headers });
    await expectOk(response);
    const config = await response.json();

    for (const key of [
      'backup',
      'ffmpeg',
      'image',
      'job',
      'library',
      'logging',
      'machineLearning',
      'map',
      'metadata',
      'newVersionCheck',
      'nightlyTasks',
      'notifications',
      'oauth',
      'passwordLogin',
      'reverseGeocoding',
      'server',
      'storageTemplate',
      'templates',
      'theme',
      'trash',
      'user',
    ]) {
      expect(config, `missing config section ${key}`).toHaveProperty(key);
    }

    expect(config.ffmpeg.crf).toBe(23);
    expect(config.backup.database.keepLastAmount).toBe(14);
    expect(config.trash.days).toBe(30);
    expect(Array.isArray(config.machineLearning.urls)).toBe(true);
  });

  test('PUT /api/system-config persists changes', async ({ request }) => {
    const admin = await signUpAdmin(request, 'sysconf-put');

    const current = await request.get('/api/system-config', { headers: admin.headers });
    await expectOk(current);
    const config = await current.json();

    config.trash.days = 45;
    config.server.loginPageMessage = 'e2e message';

    const update = await request.put('/api/system-config', {
      headers: admin.headers,
      data: config,
    });
    await expectOk(update);
    const updated = await update.json();
    expect(updated.trash.days).toBe(45);

    const reread = await request.get('/api/system-config', { headers: admin.headers });
    await expectOk(reread);
    const persisted = await reread.json();
    expect(persisted.trash.days).toBe(45);
    expect(persisted.server.loginPageMessage).toBe('e2e message');
  });

  test('defaults and storage template options', async ({ request }) => {
    const admin = await signUpAdmin(request, 'sysconf-defaults');

    const defaults = await request.get('/api/system-config/defaults', { headers: admin.headers });
    await expectOk(defaults);
    expect((await defaults.json()).trash.days).toBe(30);

    const options = await request.get('/api/system-config/storage-template-options', {
      headers: admin.headers,
    });
    await expectOk(options);
    const body = await options.json();
    expect(body.yearOptions).toEqual(['y', 'yy']);
    expect(body.presetOptions.length).toBeGreaterThan(10);
    expect(body.weekOptions).toEqual(['W', 'WW']);
  });

  test('system config requires admin', async ({ request }) => {
    const response = await request.get('/api/system-config');
    expect(response.status()).toBe(401);
  });
});

test.describe('system metadata', () => {
  test('version check state is admin-only and keeps upstream nullable shape', async ({
    playwright,
    request,
  }) => {
    const admin = await signUpAdmin(request, 'version-check-state');
    const backend = await playwright.request.newContext({
      baseURL: process.env.IMMICH_SERVER_URL ?? 'http://127.0.0.1:3001',
    });
    try {
      const unauthenticated = await backend.get('/system-metadata/version-check-state');
      expect(unauthenticated.status()).toBe(401);

      const response = await backend.get('/system-metadata/version-check-state', {
        headers: admin.headers,
      });
      await expectOk(response);

      const body = await response.json();
      expect(body).toHaveProperty('checkedAt');
      expect(body).toHaveProperty('releaseVersion');
      expect(body.checkedAt === null || typeof body.checkedAt === 'string').toBe(true);
      expect(body.releaseVersion === null || typeof body.releaseVersion === 'string').toBe(true);
    } finally {
      await backend.dispose();
    }
  });
});

test.describe('oauth', () => {
  test('backchannel logout accepts upstream form posts without auth', async ({ request }) => {
    const missingToken = await request.post('/api/oauth/backchannel-logout', { form: {} });
    expect(missingToken.status()).toBe(400);

    const disabledOAuth = await request.post('/api/oauth/backchannel-logout', {
      form: { logout_token: 'invalid-token' },
    });
    expect(disabledOAuth.status()).toBe(400);
    const body = await disabledOAuth.json();
    expect(String(body.message ?? body.error)).toContain('OAuth is not enabled');
  });
});

test.describe('notifications', () => {
  test('admin messages round-trip through notification list, read, and delete APIs', async ({
    request,
  }) => {
    const admin = await signUpAdmin(request, 'notifications-admin');
    const user = await signUpAdmin(request, 'notifications-user');
    const subject = `E2E notification ${uniqueId('subject')}`;
    const message = `Message ${uniqueId('body')}`;

    const create = await request.post('/api/admin/notifications', {
      headers: admin.headers,
      data: {
        subject,
        message,
        userIds: [user.userId],
      },
    });
    await expectOk(create);

    const list = await request.get('/api/notifications', { headers: user.headers });
    await expectOk(list);
    const notifications = await list.json();
    expect(Array.isArray(notifications)).toBe(true);

    const notification = notifications.find(
      (item: { title?: string }) => item.title === subject,
    );
    expect(notification).toBeTruthy();
    expect(notification).toMatchObject({
      title: subject,
      description: message,
      level: 'NOTIFICATION_LEVEL_INFO',
      type: 'NOTIFICATION_TYPE_SYSTEM_MESSAGE',
    });
    expect(notification.id).toBeTruthy();
    expect(notification.createdAt).toBeTruthy();
    expect(notification.readAt).toBeUndefined();

    const one = await request.get(`/api/notifications/${notification.id}`, {
      headers: user.headers,
    });
    await expectOk(one);
    expect(await one.json()).toMatchObject({
      id: notification.id,
      title: subject,
      description: message,
    });

    const readAt = '2026-07-05T00:00:00Z';
    const update = await request.put(`/api/notifications/${notification.id}`, {
      headers: user.headers,
      data: { readAt },
    });
    await expectOk(update);
    const updated = await update.json();
    expect(updated.id).toBe(notification.id);
    expect(new Date(updated.readAt).toISOString()).toBe(new Date(readAt).toISOString());

    const unread = await request.get('/api/notifications?unread=true', { headers: user.headers });
    await expectOk(unread);
    const unreadNotifications = await unread.json();
    expect(Array.isArray(unreadNotifications)).toBe(true);
    expect(
      unreadNotifications.some((item: { id?: string }) => item.id === notification.id),
    ).toBe(false);

    const remove = await request.delete(`/api/notifications/${notification.id}`, {
      headers: user.headers,
    });
    expect(remove.status()).toBe(200);

    const missing = await request.get(`/api/notifications/${notification.id}`, {
      headers: user.headers,
    });
    expect(missing.status()).toBe(404);
  });
});

test.describe('asset HLS streaming', () => {
  test('streams playlists and generated fMP4 segments', async ({ request }) => {
    test.skip(!hasFFmpeg(), 'ffmpeg/ffprobe are required for HLS E2E coverage');

    const admin = await signUpAdmin(request, 'hls');
    const mediaId = uniqueId('hls-video');
    const filename = `${mediaId}.mp4`;
    const video = generateTestMp4();

    const uploaded = await request.post('/api/assets', {
      headers: admin.headers,
      data: {
        assetData: {
          deviceAssetId: mediaId,
          deviceId: 'playwright-e2e',
          type: 'ASSET_TYPE_VIDEO',
          originalPath: `fallback/${filename}`,
          originalFileName: filename,
          fileCreatedAt: '2026-01-01T00:00:00Z',
          fileModifiedAt: '2026-01-01T00:00:00Z',
        },
        checksum: `checksum-${mediaId}`,
        fileContent: video.toString('base64'),
      },
    });
    await expectOk(uploaded);
    const asset = await uploaded.json();

    const main = await request.get(`/api/assets/${asset.id}/video/stream/main.m3u8`, {
      headers: admin.headers,
    });
    await expectOk(main);
    expect(main.headers()['content-type']).toContain('application/vnd.apple.mpegurl');

    const mainText = await main.text();
    expect(mainText).toContain('#EXTM3U');
    const sessionMatch = mainText.match(/([0-9a-f-]{36})\/0\/playlist\.m3u8/);
    expect(sessionMatch).not.toBeNull();
    const sessionId = sessionMatch![1];

    const media = await request.get(
      `/api/assets/${asset.id}/video/stream/${sessionId}/0/playlist.m3u8`,
      { headers: admin.headers },
    );
    await expectOk(media);
    expect(media.headers()['content-type']).toContain('application/vnd.apple.mpegurl');

    const mediaText = await media.text();
    expect(mediaText).toContain('#EXT-X-MAP:URI="init.mp4"');
    const segmentName = mediaText.match(/(seg_\d+\.m4s)/)?.[1];
    expect(segmentName).toBeTruthy();

    const init = await request.get(
      `/api/assets/${asset.id}/video/stream/${sessionId}/0/init.mp4`,
      { headers: admin.headers },
    );
    await expectOk(init);
    expect(init.headers()['content-type']).toContain('application/octet-stream');
    expect((await init.body()).byteLength).toBeGreaterThan(0);

    const segment = await request.get(
      `/api/assets/${asset.id}/video/stream/${sessionId}/0/${segmentName}`,
      { headers: admin.headers },
    );
    await expectOk(segment);
    expect(segment.headers()['content-type']).toContain('application/octet-stream');
    expect((await segment.body()).byteLength).toBeGreaterThan(0);

    const end = await request.delete(`/api/assets/${asset.id}/video/stream/${sessionId}`, {
      headers: admin.headers,
    });
    expect(end.status()).toBe(204);
  });
});

test.describe('admin integrity', () => {
  test('integrity reports expose the upstream empty-report shape', async ({ request }) => {
    const unauthenticated = await request.get('/api/admin/integrity/summary');
    expect(unauthenticated.status()).toBe(401);

    const admin = await signUpAdmin(request, 'integrity');

    const summary = await request.get('/api/admin/integrity/summary', { headers: admin.headers });
    await expectOk(summary);
    expect(await summary.json()).toEqual({
      checksumMismatch: 0,
      missingFile: 0,
      untrackedFile: 0,
    });

    const report = await request.get('/api/admin/integrity/report?type=missing_file', {
      headers: admin.headers,
    });
    await expectOk(report);
    const reportBody = await report.json();
    expect(reportBody.items).toEqual([]);
    expect(reportBody.nextCursor).toBeUndefined();

    const csv = await request.get('/api/admin/integrity/report/missing_file/csv', {
      headers: admin.headers,
    });
    await expectOk(csv);
    expect(csv.headers()['content-type']).toContain('application/octet-stream');
    expect(csv.headers()['content-disposition']).toContain('missing_file.csv');
    expect(await csv.text()).toBe('id,type,path\n');

    const invalid = await request.get('/api/admin/integrity/report?type=unknown', {
      headers: admin.headers,
    });
    expect(invalid.status()).toBe(400);

    const missingItem = await request.delete(
      '/api/admin/integrity/report/00000000-0000-4000-8000-000000000000',
      { headers: admin.headers },
    );
    expect(missingItem.status()).toBe(404);
  });
});

test.describe('plugins', () => {
  test('plugin triggers require auth and expose upstream enum values', async ({ request }) => {
    const unauthenticated = await request.get('/api/plugins/triggers');
    expect(unauthenticated.status()).toBe(401);

    const admin = await signUpAdmin(request, 'plugin-triggers');
    const response = await request.get('/api/plugins/triggers', { headers: admin.headers });
    await expectOk(response);

    expect(await response.json()).toEqual([
      { contextType: 'asset', type: 'AssetCreate' },
      { contextType: 'person', type: 'PersonRecognized' },
    ]);
  });

  test('plugin methods and templates are searchable arrays', async ({ request }) => {
    expect((await request.get('/api/plugins/methods')).status()).toBe(401);
    expect((await request.get('/api/plugins/templates')).status()).toBe(401);

    const admin = await signUpAdmin(request, 'plugin-methods');
    const methods = await request.get(
      '/api/plugins/methods?pluginName=thumbnail-processor&type=AssetV1&trigger=AssetCreate&enabled=true',
      { headers: admin.headers },
    );
    await expectOk(methods);

    const methodBody = await methods.json();
    expect(Array.isArray(methodBody)).toBe(true);
    expect(methodBody).toHaveLength(1);
    expect(methodBody[0]).toMatchObject({
      key: 'thumbnail-processor#generate-thumbnail',
      name: 'generate-thumbnail',
      title: 'Generate thumbnail',
      description: 'Generate thumbnails for an asset',
      hostFunctions: false,
      types: ['AssetV1'],
      uiHints: ['asset'],
    });
    expect(methodBody[0].schema.type).toBe('object');

    const noMatch = await request.get('/api/plugins/methods?pluginName=missing', {
      headers: admin.headers,
    });
    await expectOk(noMatch);
    expect(await noMatch.json()).toEqual([]);

    const templates = await request.get('/api/plugins/templates', { headers: admin.headers });
    await expectOk(templates);
    const templateBody = await templates.json();
    expect(Array.isArray(templateBody)).toBe(true);
    expect(templateBody).toHaveLength(1);
    expect(templateBody[0]).toMatchObject({
      key: 'thumbnail-processor#generate-thumbnail-on-upload',
      title: 'Generate thumbnail on upload',
      description: 'Generate thumbnails when a new asset is uploaded',
      trigger: 'AssetCreate',
      uiHints: ['asset'],
      steps: [
        {
          method: 'thumbnail-processor#generate-thumbnail',
          config: { force: false },
          enabled: true,
        },
      ],
    });
  });
});

test.describe('workflows', () => {
  test('workflow triggers and share export use upstream v3 shapes', async ({ request }) => {
    expect((await request.get('/api/workflows/triggers')).status()).toBe(401);

    const admin = await signUpAdmin(request, 'workflow-share');
    const triggers = await request.get('/api/workflows/triggers', { headers: admin.headers });
    await expectOk(triggers);
    expect(await triggers.json()).toEqual([
      { trigger: 'AssetCreate', types: ['AssetV1'] },
      { trigger: 'AssetMetadataExtraction', types: ['AssetV1'] },
    ]);

    const create = await request.post('/api/workflows', {
      headers: admin.headers,
      data: {
        name: 'E2E thumbnail workflow',
        description: 'Share export coverage',
        enabled: true,
        trigger: { type: 'WORKFLOW_TRIGGER_TYPE_ASSET_UPLOADED' },
        actions: [
          {
            type: 'WORKFLOW_ACTION_TYPE_RUN_PLUGIN',
            params: {
              method: 'thumbnail-processor#generate-thumbnail',
              force: false,
            },
            order: 1,
          },
        ],
      },
    });
    await expectOk(create);
    const workflow = await create.json();
    expect(workflow.id).toBeTruthy();

    const share = await request.get(`/api/workflows/${workflow.id}/share`, {
      headers: admin.headers,
    });
    await expectOk(share);
    const shareBody = await share.json();
    expect(shareBody).toMatchObject({
      name: 'E2E thumbnail workflow',
      description: 'Share export coverage',
      trigger: 'AssetCreate',
      steps: [
        {
          method: 'thumbnail-processor#generate-thumbnail',
          config: { force: false },
        },
      ],
    });
    expect(shareBody.id).toBeUndefined();
    expect(shareBody.createdAt).toBeUndefined();
    expect(shareBody.updatedAt).toBeUndefined();
  });
});

test.describe('multipart upload', () => {
  test('POST /api/assets multipart creates an asset and detects duplicates', async ({
    request,
  }) => {
    const admin = await signUpAdmin(request, 'upload');
    const deviceAssetId = uniqueId('multipart-asset');

    const upload = await request.post('/api/assets', {
      headers: admin.headers,
      multipart: {
        assetData: {
          name: 'e2e-multipart.png',
          mimeType: 'image/png',
          buffer: png1x1,
        },
        deviceAssetId,
        deviceId: 'e2e-device',
        fileCreatedAt: new Date('2024-03-01T12:00:00Z').toISOString(),
        fileModifiedAt: new Date('2024-03-01T12:00:00Z').toISOString(),
      },
    });
    expect(upload.status()).toBe(201);
    const created = await upload.json();
    expect(created.id).toBeTruthy();
    expect(created.status).toBe('created');

    // Same bytes again → duplicate, not a second asset.
    const duplicate = await request.post('/api/assets', {
      headers: admin.headers,
      multipart: {
        assetData: {
          name: 'e2e-multipart-again.png',
          mimeType: 'image/png',
          buffer: png1x1,
        },
        deviceAssetId: uniqueId('multipart-dup'),
        deviceId: 'e2e-device',
      },
    });
    expect(duplicate.status()).toBe(200);
    const dupBody = await duplicate.json();
    expect(dupBody.status).toBe('duplicate');
    expect(dupBody.id).toBe(created.id);
  });
});

test.describe('albums', () => {
  test('album map markers expose GPS assets as an upstream array', async ({ request }) => {
    const unauthenticated = await request.get(
      '/api/albums/00000000-0000-4000-8000-000000000000/map-markers',
    );
    expect(unauthenticated.status()).toBe(401);

    const admin = await signUpAdmin(request, 'album-markers');
    const upload = await request.post('/api/assets', {
      headers: admin.headers,
      multipart: {
        assetData: {
          name: 'e2e-gps.jpg',
          mimeType: 'image/jpeg',
          buffer: gpsJpeg,
        },
        deviceAssetId: uniqueId('gps-asset'),
        deviceId: 'e2e-device',
        fileCreatedAt: '2026-07-01T12:00:00.000Z',
        fileModifiedAt: '2026-07-01T12:00:00.000Z',
      },
    });
    expect(upload.status()).toBe(201);
    const asset = await upload.json();

    const createAlbum = await request.post('/api/albums', {
      headers: admin.headers,
      data: {
        albumName: `GPS Album ${uniqueId('album')}`,
        description: 'Album map marker E2E coverage',
        assetIds: [asset.id],
      },
    });
    await expectOk(createAlbum);
    const album = await createAlbum.json();

    let markers: AlbumMapMarker[] = [];
    for (let attempt = 0; attempt < 40; attempt += 1) {
      const response = await request.get(`/api/albums/${album.id}/map-markers`, {
        headers: admin.headers,
      });
      await expectOk(response);
      markers = (await response.json()) as AlbumMapMarker[];
      if (markers.some((marker) => marker.id === asset.id)) {
        break;
      }
      await delay(250);
    }

    expect(Array.isArray(markers)).toBe(true);
    const marker = markers.find((item) => item.id === asset.id);
    if (!marker) {
      throw new Error(`album map marker for ${asset.id} was not returned: ${JSON.stringify(markers)}`);
    }
    expect(marker).toMatchObject({
      id: asset.id,
      city: expect.any(String),
      state: expect.any(String),
      country: expect.any(String),
    });
    expect(marker.lat).toBeCloseTo(37.7749, 4);
    expect(marker.lon).toBeCloseTo(-122.4194, 4);
  });
});

test.describe('download', () => {
  test('download info and archive for an uploaded asset', async ({ request }) => {
    const admin = await signUpAdmin(request, 'download');

    const upload = await request.post('/api/assets', {
      headers: admin.headers,
      multipart: {
        assetData: {
          name: 'e2e-download.png',
          mimeType: 'image/png',
          buffer: png1x1,
        },
        deviceAssetId: uniqueId('download-asset'),
        deviceId: 'e2e-device',
      },
    });
    expect(upload.status()).toBe(201);
    const asset = await upload.json();

    const info = await request.post('/api/download/info', {
      headers: admin.headers,
      data: { assetIds: [asset.id] },
    });
    await expectOk(info);
    const infoBody = await info.json();
    expect(infoBody.totalSize).toBeGreaterThan(0);
    expect(Array.isArray(infoBody.archives)).toBe(true);
    expect(infoBody.archives[0].assetIds).toContain(asset.id);

    const archive = await request.post('/api/download/archive', {
      headers: admin.headers,
      data: { assetIds: [asset.id] },
    });
    await expectOk(archive);
    expect(archive.headers()['content-type']).toContain('application/zip');
    const zip = await archive.body();
    // Zip local file header magic.
    expect(zip.subarray(0, 2).toString()).toBe('PK');
  });
});

test.describe('jobs', () => {
  test('GET /api/jobs returns a status for every queue', async ({ request }) => {
    const admin = await signUpAdmin(request, 'jobs');

    const response = await request.get('/api/jobs', { headers: admin.headers });
    await expectOk(response);
    const body = await response.json();

    for (const key of ['backgroundTask', 'thumbnailGeneration', 'metadataExtraction', 'videoConversion']) {
      expect(body, `missing job queue ${key}`).toHaveProperty(key);
      expect(body[key].queueStatus).toBeDefined();
    }
  });
});

test.describe('users', () => {
  test('delete current user onboarding status', async ({ request }) => {
    const user = await signUpAdmin(request, 'user-onboarding');

    const update = await request.put('/api/users/me/onboarding', {
      headers: user.headers,
      data: { isOnboarded: true },
    });
    await expectOk(update);
    expect((await update.json()).isOnboarded).toBe(true);

    const deleted = await request.delete('/api/users/me/onboarding', { headers: user.headers });
    expect(deleted.status()).toBe(204);
    expect(await deleted.text()).toBe('');

    const current = await request.get('/api/users/me/onboarding', { headers: user.headers });
    await expectOk(current);
    expect((await current.json()).isOnboarded).toBe(false);
  });
});

test.describe('partners', () => {
  test('create partners with stable and deprecated upstream routes', async ({ request }) => {
    const owner = await signUpAdmin(request, 'partners-owner');
    const stablePartner = await signUpAdmin(request, 'partners-stable');
    const deprecatedPartner = await signUpAdmin(request, 'partners-deprecated');

    const stable = await request.post('/api/partners', {
      headers: owner.headers,
      data: { sharedWithId: stablePartner.userId },
    });
    expect(stable.status()).toBe(201);
    const stableBody = await stable.json();
    expect(stableBody).toMatchObject({
      id: stablePartner.userId,
      email: stablePartner.email,
      name: expect.any(String),
      inTimeline: expect.any(Boolean),
      avatarColor: expect.any(String),
      profileChangedAt: expect.any(String),
      profileImagePath: expect.any(String),
    });
    expect(stableBody.user).toBeUndefined();

    const deprecated = await request.post(`/api/partners/${deprecatedPartner.userId}`, {
      headers: owner.headers,
    });
    expect(deprecated.status()).toBe(201);
    const deprecatedBody = await deprecated.json();
    expect(deprecatedBody.id).toBe(deprecatedPartner.userId);
    expect(deprecatedBody.user).toBeUndefined();
  });
});

test.describe('shared links', () => {
  test('create, list and delete an individual shared link', async ({ request }) => {
    const admin = await signUpAdmin(request, 'sharedlinks');

    const upload = await request.post('/api/assets', {
      headers: admin.headers,
      multipart: {
        assetData: {
          name: 'e2e-shared.png',
          mimeType: 'image/png',
          buffer: png1x1,
        },
        deviceAssetId: uniqueId('shared-asset'),
        deviceId: 'e2e-device',
      },
    });
    expect(upload.status()).toBe(201);
    const asset = await upload.json();

    const create = await request.post('/api/shared-links', {
      headers: admin.headers,
      data: {
        type: 'SHARED_LINK_TYPE_INDIVIDUAL',
        assetIds: [asset.id],
        allowDownload: true,
        showMetadata: true,
      },
    });
    await expectOk(create);
    const link = await create.json();
    expect(link.id).toBeTruthy();
    expect(link.key).toBeTruthy();

    const list = await request.get('/api/shared-links', { headers: admin.headers });
    await expectOk(list);
    const listBody = await list.json();
    const links = Array.isArray(listBody) ? listBody : (listBody.sharedLinks ?? listBody.links ?? []);
    expect(JSON.stringify(links)).toContain(link.id);

    const remove = await request.delete(`/api/shared-links/${link.id}`, {
      headers: admin.headers,
    });
    await expectOk(remove);
  });
});
