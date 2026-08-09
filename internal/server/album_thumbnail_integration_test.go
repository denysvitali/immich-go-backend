//go:build integration

package server

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIntegrationAlbumThumbnailMustBeOwnedAlbumAsset(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	ctx := context.Background()
	tdb := testdb.SetupTestDB(t)
	conn, err := db.New(ctx, tdb.ConnStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	srv := &Server{db: conn}

	ownerID := tdb.CreateTestUser(t, "thumbnail-owner@example.com")
	otherOwnerID := tdb.CreateTestUser(t, "thumbnail-other@example.com")
	memberID := tdb.CreateTestAsset(t, ownerID, "thumbnail-member")
	nonMemberID := tdb.CreateTestAsset(t, ownerID, "thumbnail-non-member")
	foreignMemberID := tdb.CreateTestAsset(t, otherOwnerID, "thumbnail-foreign-member")
	album, err := tdb.Queries.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:   pgutil.UUIDToPgtype(ownerID),
		AlbumName: "thumbnail album",
	})
	require.NoError(t, err)
	for _, assetID := range []pgtype.UUID{pgutil.UUIDToPgtype(memberID), pgutil.UUIDToPgtype(foreignMemberID)} {
		require.NoError(t, tdb.Queries.AddAssetToAlbum(ctx, sqlc.AddAssetToAlbumParams{
			AlbumsId: album.ID,
			AssetsId: assetID,
		}))
	}
	authCtx := auth.WithClaims(ctx, &auth.Claims{UserID: ownerID.String()})

	member := memberID.String()
	updated, err := srv.UpdateAlbumInfo(authCtx, &immichv1.UpdateAlbumInfoRequest{
		Id:                    pgutil.UUIDToString(album.ID),
		AlbumThumbnailAssetId: &member,
	})
	require.NoError(t, err)
	assert.Equal(t, member, updated.GetAlbumThumbnailAssetId())

	for name, invalidID := range map[string]string{
		"owned asset outside album":  nonMemberID.String(),
		"foreign asset inside album": foreignMemberID.String(),
	} {
		t.Run(name, func(t *testing.T) {
			response, err := srv.UpdateAlbumInfo(authCtx, &immichv1.UpdateAlbumInfoRequest{
				Id:                    pgutil.UUIDToString(album.ID),
				AlbumThumbnailAssetId: &invalidID,
			})
			assert.Nil(t, response)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))

			unchanged, getErr := tdb.Queries.GetAlbum(ctx, album.ID)
			require.NoError(t, getErr)
			assert.Equal(t, pgutil.UUIDToPgtype(memberID), unchanged.AlbumThumbnailAssetId)
		})
	}
}
