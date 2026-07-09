//go:build integration

package activity

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func activityContext(userID uuid.UUID) context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String(), Email: "activity@example.com"})
}

func TestIntegrationActivityLifecycleFiltersAndStatistics(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	ownerID := tdb.CreateTestUser(t, "activity-owner@example.com")
	otherID := tdb.CreateTestUser(t, "activity-other@example.com")
	assetID := tdb.CreateTestAsset(t, ownerID, "activity-asset")

	album, err := tdb.Queries.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:     pgtype.UUID{Bytes: ownerID, Valid: true},
		AlbumName:   "Activity test album",
		Description: "",
	})
	require.NoError(t, err)

	server := NewServer(tdb.Queries)
	assetIDString := assetID.String()
	comment, err := server.CreateActivity(activityContext(ownerID), &immichv1.CreateActivityRequest{
		AlbumId: album.ID.String(),
		AssetId: &assetIDString,
		Comment: "A comment on an asset",
		Type:    immichv1.ReactionType_REACTION_TYPE_COMMENT,
	})
	require.NoError(t, err)
	assert.Equal(t, "A comment on an asset", comment.Comment)
	assert.Equal(t, immichv1.ReactionType_REACTION_TYPE_COMMENT, comment.Type)

	like, err := server.CreateActivity(activityContext(ownerID), &immichv1.CreateActivityRequest{
		AlbumId: album.ID.String(),
		Type:    immichv1.ReactionType_REACTION_TYPE_LIKE,
	})
	require.NoError(t, err)
	assert.Equal(t, immichv1.ReactionType_REACTION_TYPE_LIKE, like.Type)
	assert.Empty(t, like.AssetId)

	duplicateLike, err := server.CreateActivity(activityContext(ownerID), &immichv1.CreateActivityRequest{
		AlbumId: album.ID.String(),
		Type:    immichv1.ReactionType_REACTION_TYPE_LIKE,
	})
	require.NoError(t, err)
	assert.Equal(t, like.Id, duplicateLike.Id)

	all, err := server.GetActivities(activityContext(ownerID), &immichv1.GetActivitiesRequest{AlbumId: album.ID.String()})
	require.NoError(t, err)
	require.Len(t, all.Activities, 2)
	assert.Equal(t, comment.Id, all.Activities[0].Id, "activities are returned oldest first")
	assert.Equal(t, like.Id, all.Activities[1].Id)

	assetOnly := immichv1.ReactionLevel_REACTION_LEVEL_ASSET
	assetActivities, err := server.GetActivities(activityContext(ownerID), &immichv1.GetActivitiesRequest{
		AlbumId: album.ID.String(), AssetId: &assetIDString, Level: &assetOnly,
	})
	require.NoError(t, err)
	require.Len(t, assetActivities.Activities, 1)
	assert.Equal(t, comment.Id, assetActivities.Activities[0].Id)

	albumOnly := immichv1.ReactionLevel_REACTION_LEVEL_ALBUM
	likeType := immichv1.ReactionType_REACTION_TYPE_LIKE
	albumLikes, err := server.GetActivities(activityContext(ownerID), &immichv1.GetActivitiesRequest{
		AlbumId: album.ID.String(), Level: &albumOnly, Type: &likeType,
	})
	require.NoError(t, err)
	require.Len(t, albumLikes.Activities, 1)
	assert.Equal(t, like.Id, albumLikes.Activities[0].Id)

	statistics, err := server.GetActivityStatistics(activityContext(ownerID), &immichv1.GetActivityStatisticsRequest{AlbumId: album.ID.String()})
	require.NoError(t, err)
	assert.Equal(t, int32(1), statistics.Comments)
	assert.Equal(t, int32(1), statistics.Likes)

	assetStatistics, err := server.GetActivityStatistics(activityContext(ownerID), &immichv1.GetActivityStatisticsRequest{
		AlbumId: album.ID.String(), AssetId: &assetIDString,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), assetStatistics.Comments)
	assert.Equal(t, int32(0), assetStatistics.Likes)

	_, err = server.GetActivities(activityContext(otherID), &immichv1.GetActivitiesRequest{AlbumId: album.ID.String()})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = server.DeleteActivity(activityContext(otherID), &immichv1.DeleteActivityRequest{Id: comment.Id})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = server.DeleteActivity(activityContext(ownerID), &immichv1.DeleteActivityRequest{Id: comment.Id})
	require.NoError(t, err)
	remaining, err := server.GetActivities(activityContext(ownerID), &immichv1.GetActivitiesRequest{AlbumId: album.ID.String()})
	require.NoError(t, err)
	require.Len(t, remaining.Activities, 1)
	assert.Equal(t, like.Id, remaining.Activities[0].Id)
}
