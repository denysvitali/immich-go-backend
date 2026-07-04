import { expect, test } from '@playwright/test';

import { expectOk, signUpAdmin, uploadAsset } from './helpers';

const timelineColumns = [
  'id',
  'city',
  'country',
  'duration',
  'fileCreatedAt',
  'isFavorite',
  'isImage',
  'isTrashed',
  'latitude',
  'livePhotoVideoId',
  'localOffsetHours',
  'longitude',
  'ownerId',
  'projectionType',
  'ratio',
  'stack',
  'thumbhash',
  'type',
  'width',
  'height',
  'originalFileName',
  'originalPath',
  'exifImageWidth',
  'exifImageHeight',
  'exifInfo',
];

type TimelineBucket = {
  timeBucket: string;
  count: number;
};

type CalendarHeatmap = {
  from: string;
  to: string;
  totalCount: number;
  series: Array<{ date: string; count: number }>;
};

function countForDate(body: CalendarHeatmap, date: string) {
  return body.series.find((item) => item.date === date)?.count ?? 0;
}

test('timeline frontend endpoints expose bucket summaries and columnar assets', async ({ request }) => {
  const user = await signUpAdmin(request, 'timeline-api', 'E2E Timeline User');
  const asset = await uploadAsset(request, user, '2026-06-15T08:30:00Z');

  const buckets = await request.get('/api/timeline/buckets', { headers: user.headers });
  await expectOk(buckets);
  const bucketsBody = (await buckets.json()) as TimelineBucket[];

  expect(Array.isArray(bucketsBody)).toBe(true);
  const dayBucket = bucketsBody.find((bucket) => bucket.timeBucket === '2026-06-15');
  expect(dayBucket).toMatchObject({ timeBucket: '2026-06-15', count: expect.any(Number) });
  expect(dayBucket?.count).toBeGreaterThanOrEqual(1);

  const bucket = await request.get('/api/timeline/bucket?timeBucket=2026-06-15T00%3A00%3A00.000Z', {
    headers: user.headers,
  });
  await expectOk(bucket);
  const bucketBody = await bucket.json();

  for (const column of timelineColumns) {
    expect(Array.isArray(bucketBody[column]), `${column} is not an array`).toBe(true);
    expect(bucketBody[column], `${column} length`).toHaveLength(bucketBody.id.length);
  }

  const assetIndex = bucketBody.id.indexOf(asset.id);
  expect(assetIndex).toBeGreaterThanOrEqual(0);
  expect(bucketBody.ownerId[assetIndex]).toBe(user.userId);
  expect(bucketBody.fileCreatedAt[assetIndex]).toContain('2026-06-15T08:30:00');
  expect(bucketBody.isFavorite[assetIndex]).toBe(false);
  expect(bucketBody.isImage[assetIndex]).toBe(true);
  expect(bucketBody.isTrashed[assetIndex]).toBe(false);
  expect(bucketBody.originalFileName[assetIndex]).toBe(asset.originalFileName);
  expect(bucketBody.originalPath[assetIndex]).toBe(asset.originalPath);
  expect(bucketBody.ratio[assetIndex]).toBeGreaterThan(0);
  expect(bucketBody.thumbhash[assetIndex]).toBeNull();
  expect(bucketBody.type[assetIndex]).toBe('IMAGE');
});

test('calendar heatmap endpoints expose taken-date activity for users and admins', async ({ request }) => {
  const user = await signUpAdmin(request, 'calendar-heatmap', 'E2E Calendar User');
  await uploadAsset(request, user, '2026-06-15T08:30:00Z');
  await uploadAsset(request, user, '2026-06-16T12:00:00Z');

  const query = 'from=2026-06-15&to=2026-06-17&type=Taken';
  const mine = await request.get(`/api/users/me/calendar-heatmap?${query}`, {
    headers: user.headers,
  });
  await expectOk(mine);
  const mineBody = (await mine.json()) as CalendarHeatmap;

  expect(mineBody.from).toBe('2026-06-15');
  expect(mineBody.to).toBe('2026-06-17');
  expect(mineBody.series).toHaveLength(3);
  expect(mineBody.totalCount).toBe(2);
  expect(countForDate(mineBody, '2026-06-15')).toBe(1);
  expect(countForDate(mineBody, '2026-06-16')).toBe(1);
  expect(countForDate(mineBody, '2026-06-17')).toBe(0);

  const admin = await request.get(`/api/admin/users/${user.userId}/calendar-heatmap?${query}`, {
    headers: user.headers,
  });
  await expectOk(admin);
  expect(await admin.json()).toEqual(mineBody);
});
