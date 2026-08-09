//go:build integration

package people

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIntegrationReassignFacesMovesAllRowsAndRepairsFeatureFaces(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	ownerID := tdb.CreateTestUser(t, "people-reassign-owner@example.com")
	target := createIntegrationPerson(t, tdb.Queries, ownerID, "target")
	source := createIntegrationPerson(t, tdb.Queries, ownerID, "source")
	assetOneID := tdb.CreateTestAsset(t, ownerID, "people-reassign-asset-one")
	assetTwoID := tdb.CreateTestAsset(t, ownerID, "people-reassign-asset-two")
	faceOne := createIntegrationFace(t, tdb.Queries, assetOneID, uuid.UUID(source.ID.Bytes))
	faceTwo := createIntegrationFace(t, tdb.Queries, assetTwoID, uuid.UUID(source.ID.Bytes))
	_, err := tdb.Queries.UpdatePerson(ctx, sqlc.UpdatePersonParams{
		ID:          source.ID,
		FaceAssetID: faceOne.ID,
	})
	require.NoError(t, err)

	server := NewServer(tdb.Queries, nil)
	response, err := server.ReassignFaces(
		auth.WithClaims(ctx, &auth.Claims{UserID: ownerID.String()}),
		&immichv1.ReassignFacesRequest{
			Id: uuid.UUID(target.ID.Bytes).String(),
			FaceUpdates: []*immichv1.FaceUpdate{
				{AssetId: assetOneID.String(), PersonId: uuid.UUID(source.ID.Bytes).String()},
				{AssetId: assetTwoID.String(), PersonId: uuid.UUID(source.ID.Bytes).String()},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, response.GetPeople(), 2)
	assert.Equal(t, int32(2), response.GetPeople()[0].GetFaces())

	updatedFaceOne, err := tdb.Queries.GetAssetFaceByID(ctx, faceOne.ID)
	require.NoError(t, err)
	assert.Equal(t, target.ID, updatedFaceOne.PersonId)
	updatedFaceTwo, err := tdb.Queries.GetAssetFaceByID(ctx, faceTwo.ID)
	require.NoError(t, err)
	assert.Equal(t, target.ID, updatedFaceTwo.PersonId)

	updatedTarget, err := tdb.Queries.GetPerson(ctx, target.ID)
	require.NoError(t, err)
	assert.True(t, updatedTarget.FaceAssetId.Valid)
	assert.Contains(t, []uuid.UUID{uuid.UUID(faceOne.ID.Bytes), uuid.UUID(faceTwo.ID.Bytes)}, uuid.UUID(updatedTarget.FaceAssetId.Bytes))
	updatedSource, err := tdb.Queries.GetPerson(ctx, source.ID)
	require.NoError(t, err)
	assert.False(t, updatedSource.FaceAssetId.Valid)
}

func TestIntegrationReassignFacesForeignAssetRollsBackOwnedFace(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	ownerID := tdb.CreateTestUser(t, "people-reassign-safe-owner@example.com")
	otherOwnerID := tdb.CreateTestUser(t, "people-reassign-other-owner@example.com")
	target := createIntegrationPerson(t, tdb.Queries, ownerID, "target")
	source := createIntegrationPerson(t, tdb.Queries, ownerID, "source")
	ownedAssetID := tdb.CreateTestAsset(t, ownerID, "people-reassign-owned-asset")
	foreignAssetID := tdb.CreateTestAsset(t, otherOwnerID, "people-reassign-foreign-asset")
	ownedFace := createIntegrationFace(t, tdb.Queries, ownedAssetID, uuid.UUID(source.ID.Bytes))
	createIntegrationFace(t, tdb.Queries, foreignAssetID, uuid.UUID(source.ID.Bytes))

	server := NewServer(tdb.Queries, nil)
	response, err := server.ReassignFaces(
		auth.WithClaims(ctx, &auth.Claims{UserID: ownerID.String()}),
		&immichv1.ReassignFacesRequest{
			Id: uuid.UUID(target.ID.Bytes).String(),
			FaceUpdates: []*immichv1.FaceUpdate{
				{AssetId: ownedAssetID.String(), PersonId: uuid.UUID(source.ID.Bytes).String()},
				{AssetId: foreignAssetID.String(), PersonId: uuid.UUID(source.ID.Bytes).String()},
			},
		},
	)
	assert.Nil(t, response)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))

	unchangedFace, err := tdb.Queries.GetAssetFaceByID(ctx, ownedFace.ID)
	require.NoError(t, err)
	assert.Equal(t, source.ID, unchangedFace.PersonId)
}
