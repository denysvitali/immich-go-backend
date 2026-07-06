//go:build integration
// +build integration

package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func TestCheckBulkUploadReturnsExistingAssetsForAuthenticatedUser(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	ctx := context.Background()
	tdb := testdb.SetupTestDB(t)
	conn, err := db.New(ctx, tdb.ConnStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	srv := &Server{db: conn}
	ownerID := tdb.CreateTestUser(t, "bulk-upload-owner@example.com")
	otherID := tdb.CreateTestUser(t, "bulk-upload-other@example.com")

	existingA := tdb.CreateTestAsset(t, ownerID, "existing-a")
	existingB := tdb.CreateTestAsset(t, ownerID, "existing-b")
	tdb.CreateTestAsset(t, otherID, "foreign")

	resp, err := srv.CheckBulkUpload(auth.WithClaims(ctx, &auth.Claims{UserID: ownerID.String()}), &immichv1.CheckBulkUploadRequest{
		Assets: []*immichv1.CreateAssetRequest{
			{DeviceId: "test-device", DeviceAssetId: "new"},
			{DeviceId: "test-device", DeviceAssetId: "existing-b"},
			{DeviceId: "test-device", DeviceAssetId: "foreign"},
			{DeviceId: "test-device", DeviceAssetId: "existing-a"},
			{DeviceId: "test-device", DeviceAssetId: "existing-b"},
			{DeviceId: "other-device", DeviceAssetId: "existing-a"},
			nil,
			{DeviceId: "test-device"},
			{DeviceAssetId: "existing-a"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 2)

	assert.Equal(t, existingB.String(), resp.Results[0].Id)
	assert.Equal(t, "existing-b", resp.Results[0].DeviceAssetId)
	assert.Equal(t, existingA.String(), resp.Results[1].Id)
	assert.Equal(t, "existing-a", resp.Results[1].DeviceAssetId)
}

func TestCheckBulkUploadRequiresAuthentication(t *testing.T) {
	resp, err := (&Server{}).CheckBulkUpload(context.Background(), &immichv1.CheckBulkUploadRequest{})

	require.Error(t, err)
	assert.Nil(t, resp)
}
