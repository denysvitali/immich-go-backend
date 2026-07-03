import { expect, test } from '@playwright/test';

import { expectOk, login, png1x1, signUpAdmin, uniqueId } from './helpers';

test('admin can upload an image, create an album, and retrieve the image', async ({ request }) => {
  const admin = await signUpAdmin(request, 'media-flow', 'E2E Media Admin');
  expect(admin.isAdmin).toBe(true);

  const { body: loginBody, headers } = await login(request, admin);
  expect(loginBody.accessToken).toBeTruthy();
  expect(loginBody.isAdmin).toBe(true);

  const mediaId = uniqueId('dummy-picture');
  const filename = `${mediaId}.png`;

  const uploaded = await request.post('/api/assets', {
    headers,
    data: {
      assetData: {
        deviceAssetId: mediaId,
        deviceId: 'playwright-e2e',
        type: 'ASSET_TYPE_IMAGE',
        originalPath: `fallback/${filename}`,
        originalFileName: filename,
        fileCreatedAt: '2026-01-01T00:00:00Z',
        fileModifiedAt: '2026-01-01T00:00:00Z',
      },
      checksum: `checksum-${mediaId}`,
      fileContent: png1x1.toString('base64'),
    },
  });
  await expectOk(uploaded);

  const asset = await uploaded.json();
  expect(asset.id).toBeTruthy();
  expect(asset.ownerId).toBe(admin.userId);
  expect(asset.deviceAssetId).toBe(mediaId);
  expect(asset.originalFileName).toBe(filename);
  expect(asset.originalPath).toContain(`users/${admin.userId}/`);

  const albumName = `E2E Album ${mediaId}`;
  const album = await request.post('/api/albums', {
    headers,
    data: {
      albumName,
      description: 'Album created by Playwright media flow',
      assetIds: [asset.id],
    },
  });
  await expectOk(album);

  const albumBody = await album.json();
  expect(albumBody.id).toBeTruthy();
  expect(albumBody.albumName).toBe(albumName);

  const addAsset = await request.put(`/api/albums/${albumBody.id}/assets`, {
    headers,
    data: {
      assetIds: {
        ids: [asset.id],
      },
    },
  });
  await expectOk(addAsset);

  const addAssetBody = await addAsset.json();
  expect(addAssetBody.results).toHaveLength(1);
  expect(addAssetBody.results[0]).toMatchObject({ id: asset.id, success: true });

  const albums = await request.get('/api/albums', { headers });
  await expectOk(albums);
  const albumsBody = await albums.json();
  const albumItems = Array.isArray(albumsBody) ? albumsBody : albumsBody.albums;
  expect(Array.isArray(albumItems)).toBe(true);
  expect(albumItems.some((item: { id: string }) => item.id === albumBody.id)).toBe(true);

  const albumDetail = await request.get(`/api/albums/${albumBody.id}?withoutAssets=true`, { headers });
  await expectOk(albumDetail);
  const albumDetailBody = await albumDetail.json();
  expect(albumDetailBody.id).toBe(albumBody.id);
  expect(albumDetailBody.albumName).toBe(albumName);
  expect(Array.isArray(albumDetailBody.albumUsers)).toBe(true);

  const assetList = await request.get('/api/assets?page=1&size=10', { headers });
  await expectOk(assetList);
  const assetListBody = await assetList.json();
  expect(assetListBody.assets.some((item: { id: string }) => item.id === asset.id)).toBe(true);

  const fetchedAsset = await request.get(`/api/assets/${asset.id}`, { headers });
  await expectOk(fetchedAsset);
  const fetchedAssetBody = await fetchedAsset.json();
  expect(fetchedAssetBody.id).toBe(asset.id);
  expect(fetchedAssetBody.originalFileName).toBe(filename);

  const downloaded = await request.get(`/api/assets/${asset.id}/original`, { headers });
  await expectOk(downloaded);
  const downloadedBody = await downloaded.json();
  expect(downloadedBody.contentType).toBe('image/png');
  expect(downloadedBody.filename).toBe(filename);
  expect(Buffer.from(downloadedBody.data, 'base64')).toEqual(png1x1);
});
