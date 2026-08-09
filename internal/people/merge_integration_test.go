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
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func createIntegrationPerson(t *testing.T, queries *sqlc.Queries, ownerID uuid.UUID, name string) sqlc.Person {
	t.Helper()
	person, err := queries.CreatePerson(context.Background(), sqlc.CreatePersonParams{
		OwnerId: pgUUID(ownerID),
		Name:    name,
	})
	require.NoError(t, err)
	return person
}

func createIntegrationFace(t *testing.T, queries *sqlc.Queries, assetID, personID uuid.UUID) sqlc.AssetFace {
	t.Helper()
	face, err := queries.CreateFace(context.Background(), sqlc.CreateFaceParams{
		AssetId:     pgUUID(assetID),
		PersonId:    pgUUID(personID),
		ImageWidth:  100,
		ImageHeight: 100,
	})
	require.NoError(t, err)
	return face
}

func TestIntegrationMergePersonMovesFacesBeforeDeletingSources(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	ownerID := tdb.CreateTestUser(t, "people-merge-owner@example.com")
	target := createIntegrationPerson(t, tdb.Queries, ownerID, "target")
	sourceOne := createIntegrationPerson(t, tdb.Queries, ownerID, "source one")
	sourceTwo := createIntegrationPerson(t, tdb.Queries, ownerID, "source two")
	assetOneID := tdb.CreateTestAsset(t, ownerID, "people-merge-asset-one")
	assetTwoID := tdb.CreateTestAsset(t, ownerID, "people-merge-asset-two")
	faceOne := createIntegrationFace(t, tdb.Queries, assetOneID, uuid.UUID(sourceOne.ID.Bytes))
	faceTwo := createIntegrationFace(t, tdb.Queries, assetTwoID, uuid.UUID(sourceTwo.ID.Bytes))

	server := NewServer(tdb.Queries, nil)
	response, err := server.MergePerson(
		auth.WithClaims(ctx, &auth.Claims{UserID: ownerID.String()}),
		&immichv1.MergePersonRequest{
			Id:  uuid.UUID(target.ID.Bytes).String(),
			Ids: []string{uuid.UUID(sourceOne.ID.Bytes).String(), uuid.UUID(sourceTwo.ID.Bytes).String()},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, int32(2), response.GetPerson().GetFaces())

	targetFaces, err := tdb.Queries.GetFacesByPerson(ctx, target.ID)
	require.NoError(t, err)
	require.Len(t, targetFaces, 2)
	assert.ElementsMatch(t, []interface{}{faceOne.ID, faceTwo.ID}, []interface{}{
		targetFaces[0].ID, targetFaces[1].ID,
	})
	_, err = tdb.Queries.GetPerson(ctx, sourceOne.ID)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	_, err = tdb.Queries.GetPerson(ctx, sourceTwo.ID)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestIntegrationMergePersonCrossUserFailureDoesNotPartiallyMerge(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	ownerID := tdb.CreateTestUser(t, "people-merge-safe-owner@example.com")
	otherOwnerID := tdb.CreateTestUser(t, "people-merge-other-owner@example.com")
	target := createIntegrationPerson(t, tdb.Queries, ownerID, "target")
	ownedSource := createIntegrationPerson(t, tdb.Queries, ownerID, "owned source")
	foreignSource := createIntegrationPerson(t, tdb.Queries, otherOwnerID, "foreign source")
	assetID := tdb.CreateTestAsset(t, ownerID, "people-merge-atomic-asset")
	face := createIntegrationFace(t, tdb.Queries, assetID, uuid.UUID(ownedSource.ID.Bytes))

	server := NewServer(tdb.Queries, nil)
	response, err := server.MergePerson(
		auth.WithClaims(ctx, &auth.Claims{UserID: ownerID.String()}),
		&immichv1.MergePersonRequest{
			Id: uuid.UUID(target.ID.Bytes).String(),
			Ids: []string{
				uuid.UUID(ownedSource.ID.Bytes).String(),
				uuid.UUID(foreignSource.ID.Bytes).String(),
			},
		},
	)
	assert.Nil(t, response)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = tdb.Queries.GetPerson(ctx, ownedSource.ID)
	require.NoError(t, err)
	unchangedFace, err := tdb.Queries.GetAssetFaceByID(ctx, face.ID)
	require.NoError(t, err)
	assert.Equal(t, ownedSource.ID, unchangedFace.PersonId)
}
