import { expect, test } from '@playwright/test';
import { expectOk, png1x1, signUpAdmin, uniqueId } from './helpers';

// Covers the upstream Immich v2.4.0 API surface added for the web UI:
// system-config CRUD, multipart asset upload, download info/archive and the
// jobs status endpoint.

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
