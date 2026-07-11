//go:build integration
// +build integration

package server

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func TestCheckBulkUploadFlagsDuplicateChecksums(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	ctx := context.Background()
	tdb := testdb.SetupTestDB(t)
	conn, err := db.New(ctx, tdb.ConnStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	srv := &Server{db: conn}
	ownerID := tdb.CreateTestUser(t, "bulk-upload-owner@example.com")
	otherID := tdb.CreateTestUser(t, "bulk-upload-other@example.com")

	// Checksums are stored as the bytes of the 40-char hex string.
	hexActive := strings.Repeat("a1", 20)
	hexTrashed := strings.Repeat("b2", 20)
	hexForeign := strings.Repeat("c3", 20)
	hexNew := strings.Repeat("d4", 20)

	activeAsset := tdb.CreateTestAssetWithChecksum(t, ownerID, "bulk-active", []byte(hexActive))
	trashedAsset := tdb.CreateTestAssetWithChecksum(t, ownerID, "bulk-trashed", []byte(hexTrashed))
	tdb.CreateTestAssetWithChecksum(t, otherID, "bulk-foreign", []byte(hexForeign))

	require.NoError(t, tdb.Queries.TrashAssetsByIDsAndOwner(ctx, sqlc.TrashAssetsByIDsAndOwnerParams{
		OwnerId: pgtype.UUID{Bytes: ownerID, Valid: true},
		Column2: []pgtype.UUID{{Bytes: trashedAsset, Valid: true}},
	}))

	rawActive, err := hex.DecodeString(hexActive)
	require.NoError(t, err)

	resp, err := srv.CheckBulkUpload(auth.WithClaims(ctx, &auth.Claims{UserID: ownerID.String()}), &immichv1.CheckBulkUploadRequest{
		Assets: []*immichv1.AssetBulkUploadCheckItem{
			{Id: "active.png", Checksum: hexActive},
			{Id: "trashed.png", Checksum: hexTrashed},
			{Id: "foreign.png", Checksum: hexForeign},
			{Id: "new.png", Checksum: hexNew},
			{Id: "base64.png", Checksum: base64.StdEncoding.EncodeToString(rawActive)},
			{Id: "junk.png", Checksum: "not-a-checksum"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 6)

	byID := make(map[string]*immichv1.AssetBulkUploadCheckResult, len(resp.Results))
	for _, result := range resp.Results {
		byID[result.Id] = result
	}

	active := byID["active.png"]
	require.NotNil(t, active)
	assert.Equal(t, "reject", active.Action)
	assert.Equal(t, "duplicate", active.GetReason())
	assert.Equal(t, activeAsset.String(), active.GetAssetId())
	assert.False(t, active.GetIsTrashed())

	trashed := byID["trashed.png"]
	require.NotNil(t, trashed)
	assert.Equal(t, "reject", trashed.Action)
	assert.Equal(t, "duplicate", trashed.GetReason())
	assert.Equal(t, trashedAsset.String(), trashed.GetAssetId())
	assert.True(t, trashed.GetIsTrashed())

	// Another user's asset with the same checksum is not a duplicate.
	assert.Equal(t, "accept", byID["foreign.png"].Action)
	assert.Equal(t, "accept", byID["new.png"].Action)

	// Base64-encoded checksums match their hex-stored equivalent.
	base64Result := byID["base64.png"]
	require.NotNil(t, base64Result)
	assert.Equal(t, "reject", base64Result.Action)
	assert.Equal(t, activeAsset.String(), base64Result.GetAssetId())

	// Unparseable checksums fall through to accept.
	assert.Equal(t, "accept", byID["junk.png"].Action)
}

func TestCheckBulkUploadRequiresAuthentication(t *testing.T) {
	resp, err := (&Server{}).CheckBulkUpload(context.Background(), &immichv1.CheckBulkUploadRequest{})

	require.Error(t, err)
	assert.Nil(t, resp)
}
