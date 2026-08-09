package userdeletion

import (
	"context"
	"errors"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type deletionHarness struct {
	assets       []sqlc.Asset
	attempted    []uuid.UUID
	failAsset    uuid.UUID
	failureArmed bool
}

func (h *deletionHarness) GetAllUserAssetsForDeletion(_ context.Context, arg sqlc.GetAllUserAssetsForDeletionParams) ([]sqlc.Asset, error) {
	start := int(arg.PageOffset)
	if start >= len(h.assets) {
		return nil, nil
	}
	end := start + int(arg.PageSize)
	if end > len(h.assets) {
		end = len(h.assets)
	}
	return append([]sqlc.Asset(nil), h.assets[start:end]...), nil
}

func (h *deletionHarness) HardDeleteAsset(_ context.Context, assetID, _ uuid.UUID) error {
	h.attempted = append(h.attempted, assetID)
	if h.failureArmed && assetID == h.failAsset {
		h.failureArmed = false
		return errors.New("storage unavailable")
	}
	return nil
}

func TestDeleteOwnedAssetsIsRetryableAfterAssetFailure(t *testing.T) {
	ownerID := uuid.New()
	firstID := uuid.New()
	failingID := uuid.New()
	lastID := uuid.New()
	harness := &deletionHarness{
		assets: []sqlc.Asset{
			{ID: pgutil.UUIDToPgtype(firstID)},
			{ID: pgutil.UUIDToPgtype(failingID)},
			{ID: pgutil.UUIDToPgtype(lastID)},
		},
		failAsset:    failingID,
		failureArmed: true,
	}

	err := DeleteOwnedAssets(context.Background(), harness, harness, ownerID)
	require.Error(t, err)
	assert.ErrorContains(t, err, failingID.String())
	assert.Equal(t, []uuid.UUID{firstID, failingID}, harness.attempted)

	harness.attempted = nil
	require.NoError(t, DeleteOwnedAssets(context.Background(), harness, harness, ownerID))
	assert.Equal(t, []uuid.UUID{firstID, failingID, lastID}, harness.attempted)
}

func TestDeleteOwnedAssetsRequiresDeletionWorkflow(t *testing.T) {
	err := DeleteOwnedAssets(context.Background(), &deletionHarness{}, nil, uuid.New())
	require.Error(t, err)
	assert.ErrorContains(t, err, "not configured")
}

func TestDeleteOwnedAssetsDeletesMoreThanOneBatch(t *testing.T) {
	ownerID := uuid.New()
	harness := &deletionHarness{assets: make([]sqlc.Asset, assetPageSize+1)}
	for i := range harness.assets {
		harness.assets[i].ID = pgutil.UUIDToPgtype(uuid.New())
	}

	require.NoError(t, DeleteOwnedAssets(context.Background(), harness, harness, ownerID))
	assert.Len(t, harness.attempted, assetPageSize+1)
}
