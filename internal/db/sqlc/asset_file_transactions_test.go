package sqlc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type encodedVideoQueryStub struct {
	updateErr error
	createErr error
	calls     []string
}

func (s *encodedVideoQueryStub) UpdateAssetEncodedVideoPath(_ context.Context, params UpdateAssetEncodedVideoPathParams) (Asset, error) {
	s.calls = append(s.calls, "update")
	return Asset{ID: params.ID, EncodedVideoPath: params.EncodedVideoPath}, s.updateErr
}

func (s *encodedVideoQueryStub) CreateAssetFile(_ context.Context, params CreateAssetFileParams) (AssetFile, error) {
	s.calls = append(s.calls, "record")
	return AssetFile{AssetId: params.AssetId, Type: params.Type, Path: params.Path}, s.createErr
}

func TestFinalizeEncodedVideoInTransactionPublishesPathAndCleanupRecord(t *testing.T) {
	assetID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	stub := &encodedVideoQueryStub{}

	asset, err := finalizeEncodedVideoInTransaction(context.Background(), stub, FinalizeEncodedVideoParams{
		AssetID: assetID,
		Path:    "encoded/video.mp4",
	})

	require.NoError(t, err)
	assert.Equal(t, assetID, asset.ID)
	assert.Equal(t, "encoded/video.mp4", asset.EncodedVideoPath.String)
	assert.Equal(t, []string{"update", "record"}, stub.calls)
}

func TestFinalizeEncodedVideoInTransactionReturnsRecordFailureForRollback(t *testing.T) {
	stub := &encodedVideoQueryStub{createErr: errors.New("injected record failure")}

	_, err := finalizeEncodedVideoInTransaction(context.Background(), stub, FinalizeEncodedVideoParams{
		AssetID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Path:    "encoded/video.mp4",
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "record encoded video file")
	assert.Equal(t, []string{"update", "record"}, stub.calls)
}
