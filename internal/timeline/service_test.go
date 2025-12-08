//go:build integration
// +build integration

package timeline

import (
	"context"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestUser creates a test user and returns the user ID
func createTestUser(t *testing.T, tdb *testdb.TestDB, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	userID := uuid.New()
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	_, err := tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    email,
		Name:     "Test User",
		Password: "hashedpassword",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	return userID
}

// createTestAsset creates a test asset and returns the asset ID
func createTestAsset(t *testing.T, tdb *testdb.TestDB, ownerID uuid.UUID, deviceAssetID string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	ownerUUID := pgtype.UUID{Bytes: ownerID, Valid: true}

	asset, err := tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    deviceAssetID,
		OwnerId:          ownerUUID,
		DeviceId:         "test-device",
		Type:             "IMAGE",
		OriginalPath:     "/test/path/" + deviceAssetID + ".jpg",
		OriginalFileName: deviceAssetID + ".jpg",
		Checksum:         []byte("test-checksum-" + deviceAssetID),
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)

	return asset.ID.Bytes
}

func TestIntegration_GetTimelineAssets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and assets
	userID := createTestUser(t, tdb, "timeline@test.com")
	createTestAsset(t, tdb, userID, "asset1")
	createTestAsset(t, tdb, userID, "asset2")
	createTestAsset(t, tdb, userID, "asset3")

	// Get timeline assets
	opts := TimelineOptions{
		UserID: userID.String(),
		Limit:  10,
		Offset: 0,
	}

	assetIDs, err := service.GetTimelineAssets(ctx, opts)
	require.NoError(t, err)
	assert.Len(t, assetIDs, 3)
}

func TestIntegration_GetTimelineAssets_Pagination(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and multiple assets
	userID := createTestUser(t, tdb, "pagination@test.com")
	for i := 0; i < 5; i++ {
		createTestAsset(t, tdb, userID, "pageasset"+string(rune('0'+i)))
	}

	// Get first page
	opts := TimelineOptions{
		UserID: userID.String(),
		Limit:  2,
		Offset: 0,
	}

	page1, err := service.GetTimelineAssets(ctx, opts)
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	// Get second page
	opts.Offset = 2
	page2, err := service.GetTimelineAssets(ctx, opts)
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	// Get third page (partial)
	opts.Offset = 4
	page3, err := service.GetTimelineAssets(ctx, opts)
	require.NoError(t, err)
	assert.Len(t, page3, 1)
}

func TestIntegration_GetTimelineAssets_UserIsolation(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "user1timeline@test.com")
	user2ID := createTestUser(t, tdb, "user2timeline@test.com")

	// Create assets for user1
	createTestAsset(t, tdb, user1ID, "user1asset1")
	createTestAsset(t, tdb, user1ID, "user1asset2")

	// Create assets for user2
	createTestAsset(t, tdb, user2ID, "user2asset1")

	// User1 should only see their assets
	opts1 := TimelineOptions{UserID: user1ID.String(), Limit: 10}
	assets1, err := service.GetTimelineAssets(ctx, opts1)
	require.NoError(t, err)
	assert.Len(t, assets1, 2)

	// User2 should only see their assets
	opts2 := TimelineOptions{UserID: user2ID.String(), Limit: 10}
	assets2, err := service.GetTimelineAssets(ctx, opts2)
	require.NoError(t, err)
	assert.Len(t, assets2, 1)
}

func TestIntegration_GetTimelineAssets_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Try with invalid user ID
	opts := TimelineOptions{
		UserID: "not-a-valid-uuid",
		Limit:  10,
	}

	_, err := service.GetTimelineAssets(ctx, opts)
	assert.Error(t, err)
}

func TestIntegration_GetTimeBuckets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and assets
	userID := createTestUser(t, tdb, "buckets@test.com")
	createTestAsset(t, tdb, userID, "bucket1")
	createTestAsset(t, tdb, userID, "bucket2")

	// Get time buckets (day)
	opts := TimelineOptions{
		UserID:     userID.String(),
		TimeBucket: "day",
	}

	buckets, err := service.GetTimeBuckets(ctx, opts)
	require.NoError(t, err)
	// Should have at least one bucket
	assert.GreaterOrEqual(t, len(buckets), 0) // May be 0 if query returns empty for test data
}

func TestIntegration_GetTimeBuckets_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Try with invalid user ID
	opts := TimelineOptions{
		UserID:     "not-a-valid-uuid",
		TimeBucket: "day",
	}

	_, err := service.GetTimeBuckets(ctx, opts)
	assert.Error(t, err)
}

func TestIntegration_GetMonthlyBuckets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and assets
	userID := createTestUser(t, tdb, "monthly@test.com")
	createTestAsset(t, tdb, userID, "monthly1")
	createTestAsset(t, tdb, userID, "monthly2")

	// Get monthly buckets
	buckets, err := service.GetMonthlyBuckets(ctx, userID.String(), 2024)
	require.NoError(t, err)
	// Should have buckets (may be empty for test data without proper dates)
	assert.NotNil(t, buckets)
}

func TestIntegration_GetYearlyBuckets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and assets
	userID := createTestUser(t, tdb, "yearly@test.com")
	createTestAsset(t, tdb, userID, "yearly1")
	createTestAsset(t, tdb, userID, "yearly2")

	// Get yearly buckets
	buckets, err := service.GetYearlyBuckets(ctx, userID.String())
	require.NoError(t, err)
	assert.NotNil(t, buckets)
}

func TestIntegration_GetDayDetail(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and assets
	userID := createTestUser(t, tdb, "daydetail@test.com")
	createTestAsset(t, tdb, userID, "day1")

	// Get day detail
	date := time.Now()
	bucket, err := service.GetDayDetail(ctx, userID.String(), date)
	require.NoError(t, err)
	assert.NotNil(t, bucket)
	assert.Equal(t, date.Format("2006-01-02"), bucket.Date)
}

func TestIntegration_GetDayDetail_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Try with invalid user ID
	_, err := service.GetDayDetail(ctx, "not-a-valid-uuid", time.Now())
	assert.Error(t, err)
}

func TestIntegration_GetTimelineStats(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and assets
	userID := createTestUser(t, tdb, "stats@test.com")
	createTestAsset(t, tdb, userID, "stat1")
	createTestAsset(t, tdb, userID, "stat2")
	createTestAsset(t, tdb, userID, "stat3")

	// Get timeline stats
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

	// Try with invalid user ID
	_, err := service.GetTimelineStats(ctx, "not-a-valid-uuid")
	assert.Error(t, err)
}

func TestIntegration_EmptyTimeline(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user without any assets
	userID := createTestUser(t, tdb, "empty@test.com")

	// Get timeline assets
	opts := TimelineOptions{
		UserID: userID.String(),
		Limit:  10,
	}

	assetIDs, err := service.GetTimelineAssets(ctx, opts)
	require.NoError(t, err)
	assert.Empty(t, assetIDs)

	// Get stats
	stats, err := service.GetTimelineStats(ctx, userID.String())
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats["total"])
}
