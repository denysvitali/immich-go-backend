package sqlc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type originalFileQueryStub struct {
	asset       Asset
	lockErr     error
	finalizeErr error
	calls       []string
}

func (s *originalFileQueryStub) GetAssetForOriginalFileWrite(_ context.Context, _ pgtype.UUID) (Asset, error) {
	s.calls = append(s.calls, "lock")
	return s.asset, s.lockErr
}

func (s *originalFileQueryStub) CompleteAssetOriginalFileWrite(_ context.Context, _ pgtype.UUID) (Asset, error) {
	s.calls = append(s.calls, "finalize")
	return s.asset, s.finalizeErr
}

func (s *originalFileQueryStub) GetOwnedAssetForOriginalFileWrite(_ context.Context, _ GetOwnedAssetForOriginalFileWriteParams) (Asset, error) {
	s.calls = append(s.calls, "lock-owned")
	return s.asset, s.lockErr
}

func (s *originalFileQueryStub) ReplaceAssetFile(_ context.Context, params ReplaceAssetFileParams) (Asset, error) {
	s.calls = append(s.calls, "finalize-replacement")
	asset := s.asset
	asset.Checksum = params.Checksum
	asset.FileModifiedAt = params.FileModifiedAt
	return asset, s.finalizeErr
}

func TestCompleteAssetUploadInTransactionLocksWritesThenFinalizes(t *testing.T) {
	assetID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	stub := &originalFileQueryStub{asset: Asset{ID: assetID, OriginalPath: "original.jpg"}}

	asset, err := completeAssetUploadInTransaction(context.Background(), stub, CompleteAssetUploadParams{
		AssetID: assetID,
		WriteOriginalFile: func(_ context.Context, locked Asset) error {
			stub.calls = append(stub.calls, "write:"+locked.OriginalPath)
			return nil
		},
	})

	require.NoError(t, err)
	assert.Equal(t, assetID, asset.ID)
	assert.Equal(t, []string{"lock", "write:original.jpg", "finalize"}, stub.calls)
}

func TestReplaceAssetOriginalFileInTransactionRetainsOwnershipLockThroughFinalization(t *testing.T) {
	assetID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	ownerID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	stub := &originalFileQueryStub{asset: Asset{ID: assetID, OwnerId: ownerID, OriginalPath: "original.jpg"}}

	asset, err := replaceAssetOriginalFileInTransaction(context.Background(), stub, ReplaceAssetOriginalFileParams{
		AssetID:  assetID,
		OwnerID:  ownerID,
		Checksum: []byte("replacement-checksum"),
		WriteOriginalFile: func(_ context.Context, locked Asset) error {
			stub.calls = append(stub.calls, "write:"+locked.OriginalPath)
			return nil
		},
	})

	require.NoError(t, err)
	assert.Equal(t, []byte("replacement-checksum"), asset.Checksum)
	assert.Equal(t, []string{"lock-owned", "write:original.jpg", "finalize-replacement"}, stub.calls)
}

func TestOriginalFileWriteRejectedBeforeStorageCallback(t *testing.T) {
	assetID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	stub := &originalFileQueryStub{lockErr: pgx.ErrNoRows}
	writeCalled := false

	_, err := completeAssetUploadInTransaction(context.Background(), stub, CompleteAssetUploadParams{
		AssetID: assetID,
		WriteOriginalFile: func(context.Context, Asset) error {
			writeCalled = true
			return nil
		},
	})

	require.ErrorIs(t, err, ErrAssetOriginalFileWriteRejected)
	assert.False(t, writeCalled)
	assert.Equal(t, []string{"lock"}, stub.calls)
}

func TestOriginalFileWriteFailureSkipsDatabaseFinalization(t *testing.T) {
	assetID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	writeErr := errors.New("storage unavailable")
	stub := &originalFileQueryStub{asset: Asset{ID: assetID}}

	_, err := completeAssetUploadInTransaction(context.Background(), stub, CompleteAssetUploadParams{
		AssetID: assetID,
		WriteOriginalFile: func(context.Context, Asset) error {
			stub.calls = append(stub.calls, "write")
			return writeErr
		},
	})

	require.ErrorIs(t, err, writeErr)
	assert.Equal(t, []string{"lock", "write"}, stub.calls)
}
