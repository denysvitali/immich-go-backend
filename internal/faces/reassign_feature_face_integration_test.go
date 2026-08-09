//go:build integration

package faces

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationReassignFacesByIDRepairsFeatureFaces(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	service, err := NewService(tdb.Queries, &config.Config{})
	require.NoError(t, err)

	ownerID := createTestUser(t, tdb, "single-face-feature-owner@test.com")
	sourceID := createTestPerson(t, tdb, ownerID, "Feature Source")
	targetID := createTestPerson(t, tdb, ownerID, "Feature Target")
	movedAssetID := createTestAsset(t, tdb, ownerID, "single-face-moved")
	remainingAssetID := createTestAsset(t, tdb, ownerID, "single-face-remaining")
	moved, err := service.CreateFace(ctx, ownerID, CreateFaceRequest{
		AssetID:  movedAssetID.String(),
		PersonID: sourceID.String(),
		BoundingBox: BoundingBox{
			X1: 10, Y1: 10, X2: 20, Y2: 20,
		},
	})
	require.NoError(t, err)
	remaining, err := service.CreateFace(ctx, ownerID, CreateFaceRequest{
		AssetID:  remainingAssetID.String(),
		PersonID: sourceID.String(),
		BoundingBox: BoundingBox{
			X1: 30, Y1: 30, X2: 40, Y2: 40,
		},
	})
	require.NoError(t, err)
	movedFaceID := pgutil.UUIDToPgtype(uuid.MustParse(moved.ID))
	remainingFaceID := pgutil.UUIDToPgtype(uuid.MustParse(remaining.ID))
	_, err = tdb.Queries.UpdatePerson(ctx, sqlc.UpdatePersonParams{
		ID:          pgutil.UUIDToPgtype(sourceID),
		FaceAssetID: movedFaceID,
	})
	require.NoError(t, err)

	response, err := service.ReassignFacesById(ctx, ownerID, moved.ID, targetID.String())
	require.NoError(t, err)
	require.Len(t, response.UpdatedFaces, 1)
	assert.Equal(t, targetID.String(), response.UpdatedFaces[0].PersonID)

	source, err := tdb.Queries.GetPerson(ctx, pgutil.UUIDToPgtype(sourceID))
	require.NoError(t, err)
	assert.Equal(t, remainingFaceID, source.FaceAssetId)
	target, err := tdb.Queries.GetPerson(ctx, pgutil.UUIDToPgtype(targetID))
	require.NoError(t, err)
	assert.Equal(t, movedFaceID, target.FaceAssetId)
	updatedFace, err := tdb.Queries.GetAssetFaceByID(ctx, movedFaceID)
	require.NoError(t, err)
	assert.Equal(t, pgutil.UUIDToPgtype(targetID), updatedFace.PersonId)
}
