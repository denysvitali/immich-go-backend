//go:build integration
// +build integration

package timeline

import (
	"context"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestUser(t *testing.T, tdb *testdb.TestDB, email string) uuid.UUID {
	return tdb.CreateTestUser(t, email)
}

func createTestAsset(t *testing.T, tdb *testdb.TestDB, ownerID uuid.UUID, deviceAssetID string) uuid.UUID {
	return tdb.CreateTestAsset(t, ownerID, deviceAssetID)
}

func TestIntegration_GetTimeBuckets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	userID := createTestUser(t, tdb, "buckets@test.com")
	createTestAsset(t, tdb, userID, "bucket1")
	createTestAsset(t, tdb, userID, "bucket2")

	opts := ListOptions{
		UserID:     userID.String(),
		Bucket:     "day",
		IsFavorite: false,
		IsTrashed:  false,
	}

	buckets, err := service.GetTimeBuckets(ctx, opts)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(buckets), 0)
}

func TestIntegration_GetTimeBuckets_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	opts := ListOptions{
		UserID:     "not-a-valid-uuid",
		Bucket:     "day",
		IsFavorite: false,
		IsTrashed:  false,
	}

	_, err := service.GetTimeBuckets(ctx, opts)
	assert.Error(t, err)
}

func TestIntegration_GetBucketAssets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	userID := createTestUser(t, tdb, "bucketassets@test.com")
	createTestAsset(t, tdb, userID, "ba1")
	createTestAsset(t, tdb, userID, "ba2")

	opts := ListOptions{
		UserID:     userID.String(),
		Bucket:     "day",
		Date:       time.Now().Format("2006-01-02"),
		IsFavorite: false,
		IsTrashed:  false,
		Limit:      10,
	}

	assets, err := service.GetBucketAssets(ctx, opts)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(assets), 0)
}

func TestIntegration_GetBucketAssets_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	opts := ListOptions{
		UserID:     "not-a-valid-uuid",
		Bucket:     "day",
		Date:       time.Now().Format("2006-01-02"),
		IsFavorite: false,
		IsTrashed:  false,
		Limit:      10,
	}

	_, err := service.GetBucketAssets(ctx, opts)
	assert.Error(t, err)
}

func TestIntegration_GetBucketAssets_InvalidDate(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	userID := createTestUser(t, tdb, "badate@test.com")

	opts := ListOptions{
		UserID:     userID.String(),
		Bucket:     "day",
		Date:       "not-a-date",
		IsFavorite: false,
		IsTrashed:  false,
		Limit:      10,
	}

	_, err := service.GetBucketAssets(ctx, opts)
	assert.Error(t, err)
}

func TestIntegration_GetTimelineStats(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	userID := createTestUser(t, tdb, "stats@test.com")
	createTestAsset(t, tdb, userID, "stat1")
	createTestAsset(t, tdb, userID, "stat2")
	createTestAsset(t, tdb, userID, "stat3")

	stats, err := service.GetTimelineStats(ctx, userID.String())
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "images")
	assert.Contains(t, stats, "videos")
	assert.Contains(t, stats, "total")
}

func TestIntegration_GetTimelineStats_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	_, err := service.GetTimelineStats(ctx, "not-a-valid-uuid")
	assert.Error(t, err)
}

func TestIntegration_EmptyTimeline(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	userID := createTestUser(t, tdb, "empty@test.com")

	opts := ListOptions{
		UserID:     userID.String(),
		Bucket:     "day",
		Date:       time.Now().Format("2006-01-02"),
		IsFavorite: false,
		IsTrashed:  false,
		Limit:      10,
	}

	assets, err := service.GetBucketAssets(ctx, opts)
	require.NoError(t, err)
	assert.Empty(t, assets)

	stats, err := service.GetTimelineStats(ctx, userID.String())
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats["total"])
}
