//go:build integration
// +build integration

package faces

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/config"
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

// createTestPerson creates a test person and returns the person ID
func createTestPerson(t *testing.T, tdb *testdb.TestDB, ownerID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	ownerUUID := pgtype.UUID{Bytes: ownerID, Valid: true}

	person, err := tdb.Queries.CreatePerson(ctx, sqlc.CreatePersonParams{
		OwnerId: ownerUUID,
		Name:    name,
	})
	require.NoError(t, err)

	return person.ID.Bytes
}

func TestIntegration_CreateFace(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user, asset, and person
	userID := createTestUser(t, tdb, "facetest@test.com")
	assetID := createTestAsset(t, tdb, userID, "faceasset1")
	personID := createTestPerson(t, tdb, userID, "Test Person")

	// Create a face
	response, err := service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  assetID.String(),
		PersonID: personID.String(),
		BoundingBox: BoundingBox{
			X1: 100,
			Y1: 100,
			X2: 200,
			Y2: 200,
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, assetID.String(), response.AssetID)
	assert.Equal(t, personID.String(), response.PersonID)
	assert.Equal(t, int32(100), response.BoundingBox.X1)
	assert.Equal(t, int32(100), response.BoundingBox.Y1)
	assert.Equal(t, int32(200), response.BoundingBox.X2)
	assert.Equal(t, int32(200), response.BoundingBox.Y2)
}

func TestIntegration_CreateFaceInvalidAssetID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try to create a face with invalid asset ID
	response, err := service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  "not-a-valid-uuid",
		PersonID: uuid.New().String(),
		BoundingBox: BoundingBox{
			X1: 100,
			Y1: 100,
			X2: 200,
			Y2: 200,
		},
	})
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid asset ID")
}

func TestIntegration_CreateFaceInvalidPersonID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try to create a face with invalid person ID
	response, err := service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  uuid.New().String(),
		PersonID: "not-a-valid-uuid",
		BoundingBox: BoundingBox{
			X1: 100,
			Y1: 100,
			X2: 200,
			Y2: 200,
		},
	})
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid person ID")
}

func TestIntegration_GetFacesByAsset(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user, asset, and person
	userID := createTestUser(t, tdb, "getfaces@test.com")
	assetID := createTestAsset(t, tdb, userID, "getfacesasset")
	personID := createTestPerson(t, tdb, userID, "Get Faces Person")

	// Create multiple faces for the same asset
	_, err = service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  assetID.String(),
		PersonID: personID.String(),
		BoundingBox: BoundingBox{
			X1: 100, Y1: 100, X2: 200, Y2: 200,
		},
	})
	require.NoError(t, err)

	_, err = service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  assetID.String(),
		PersonID: personID.String(),
		BoundingBox: BoundingBox{
			X1: 300, Y1: 100, X2: 400, Y2: 200,
		},
	})
	require.NoError(t, err)

	// Get faces by asset
	response, err := service.GetFaces(ctx, GetFacesRequest{
		AssetID: assetID.String(),
	})
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Faces, 2)
}

func TestIntegration_GetFacesByPerson(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user, assets, and person
	userID := createTestUser(t, tdb, "getfacesperson@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "personasset1")
	asset2ID := createTestAsset(t, tdb, userID, "personasset2")
	personID := createTestPerson(t, tdb, userID, "Multi-Asset Person")

	// Create faces for the same person across different assets
	_, err = service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  asset1ID.String(),
		PersonID: personID.String(),
		BoundingBox: BoundingBox{
			X1: 100, Y1: 100, X2: 200, Y2: 200,
		},
	})
	require.NoError(t, err)

	_, err = service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  asset2ID.String(),
		PersonID: personID.String(),
		BoundingBox: BoundingBox{
			X1: 150, Y1: 150, X2: 250, Y2: 250,
		},
	})
	require.NoError(t, err)

	// Get faces by person
	response, err := service.GetFaces(ctx, GetFacesRequest{
		PersonID: personID.String(),
	})
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Faces, 2)
}

func TestIntegration_GetFacesNoFilter(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Get faces with no filter returns empty
	response, err := service.GetFaces(ctx, GetFacesRequest{})
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Empty(t, response.Faces)
}

func TestIntegration_GetFacesInvalidAssetID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try to get faces with invalid asset ID
	response, err := service.GetFaces(ctx, GetFacesRequest{
		AssetID: "not-a-valid-uuid",
	})
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid asset ID")
}

func TestIntegration_DeleteFace(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user, asset, and person
	userID := createTestUser(t, tdb, "deleteface@test.com")
	assetID := createTestAsset(t, tdb, userID, "deletefaceasset")
	personID := createTestPerson(t, tdb, userID, "Delete Face Person")

	// Create a face
	createResponse, err := service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  assetID.String(),
		PersonID: personID.String(),
		BoundingBox: BoundingBox{
			X1: 100, Y1: 100, X2: 200, Y2: 200,
		},
	})
	require.NoError(t, err)

	// Delete the face
	err = service.DeleteFace(ctx, createResponse.ID)
	require.NoError(t, err)

	// Verify face is deleted by checking faces for the asset
	getResponse, err := service.GetFaces(ctx, GetFacesRequest{
		AssetID: assetID.String(),
	})
	require.NoError(t, err)
	assert.Empty(t, getResponse.Faces)
}

func TestIntegration_DeleteFaceInvalidID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try to delete with invalid ID
	err = service.DeleteFace(ctx, "not-a-valid-uuid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid face ID")
}

func TestIntegration_ReassignFacesById(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user, asset, and two persons
	userID := createTestUser(t, tdb, "reassign@test.com")
	assetID := createTestAsset(t, tdb, userID, "reassignasset")
	person1ID := createTestPerson(t, tdb, userID, "Person One")
	person2ID := createTestPerson(t, tdb, userID, "Person Two")

	// Create a face assigned to person1
	createResponse, err := service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  assetID.String(),
		PersonID: person1ID.String(),
		BoundingBox: BoundingBox{
			X1: 100, Y1: 100, X2: 200, Y2: 200,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, person1ID.String(), createResponse.PersonID)

	// Reassign the face to person2
	reassignResponse, err := service.ReassignFacesById(ctx, createResponse.ID, person2ID.String())
	require.NoError(t, err)
	assert.NotNil(t, reassignResponse)
	assert.Len(t, reassignResponse.UpdatedFaces, 1)
	assert.Equal(t, person2ID.String(), reassignResponse.UpdatedFaces[0].PersonID)

	// Verify the face now belongs to person2
	getResponse, err := service.GetFaces(ctx, GetFacesRequest{
		PersonID: person2ID.String(),
	})
	require.NoError(t, err)
	assert.Len(t, getResponse.Faces, 1)
	assert.Equal(t, createResponse.ID, getResponse.Faces[0].ID)
}

func TestIntegration_ReassignFacesInvalidFaceID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try to reassign with invalid face ID
	response, err := service.ReassignFacesById(ctx, "not-a-valid-uuid", uuid.New().String())
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid face ID")
}

func TestIntegration_ReassignFacesInvalidPersonID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try to reassign with invalid person ID
	response, err := service.ReassignFacesById(ctx, uuid.New().String(), "not-a-valid-uuid")
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid person ID")
}

func TestIntegration_MultipleFacesPerAsset(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user, asset, and multiple persons
	userID := createTestUser(t, tdb, "multiface@test.com")
	assetID := createTestAsset(t, tdb, userID, "multifaceasset")
	person1ID := createTestPerson(t, tdb, userID, "Alice")
	person2ID := createTestPerson(t, tdb, userID, "Bob")
	person3ID := createTestPerson(t, tdb, userID, "Charlie")

	// Create faces for different people in the same photo
	_, err = service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  assetID.String(),
		PersonID: person1ID.String(),
		BoundingBox: BoundingBox{
			X1: 50, Y1: 50, X2: 150, Y2: 150,
		},
	})
	require.NoError(t, err)

	_, err = service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  assetID.String(),
		PersonID: person2ID.String(),
		BoundingBox: BoundingBox{
			X1: 200, Y1: 50, X2: 300, Y2: 150,
		},
	})
	require.NoError(t, err)

	_, err = service.CreateFace(ctx, CreateFaceRequest{
		AssetID:  assetID.String(),
		PersonID: person3ID.String(),
		BoundingBox: BoundingBox{
			X1: 350, Y1: 50, X2: 450, Y2: 150,
		},
	})
	require.NoError(t, err)

	// Verify all faces are retrieved
	response, err := service.GetFaces(ctx, GetFacesRequest{
		AssetID: assetID.String(),
	})
	require.NoError(t, err)
	assert.Len(t, response.Faces, 3)

	// Verify each person has exactly one face for this asset
	personFaceCount := make(map[string]int)
	for _, face := range response.Faces {
		personFaceCount[face.PersonID]++
	}
	assert.Equal(t, 1, personFaceCount[person1ID.String()])
	assert.Equal(t, 1, personFaceCount[person2ID.String()])
	assert.Equal(t, 1, personFaceCount[person3ID.String()])
}
