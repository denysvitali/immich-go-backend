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

type fakeOwnedAlbumUpdateQueries struct {
	albumErr     error
	thumbnailErr error
	updateErr    error
	updated      Album
	albumCalls   []GetOwnedAlbumForUpdateParams
	thumbCalls   []GetOwnedAlbumThumbnailForUpdateParams
	updateCalls  []UpdateAlbumParams
}

func (f *fakeOwnedAlbumUpdateQueries) GetOwnedAlbumForUpdate(_ context.Context, params GetOwnedAlbumForUpdateParams) (Album, error) {
	f.albumCalls = append(f.albumCalls, params)
	return Album{ID: params.AlbumID, OwnerId: params.OwnerID}, f.albumErr
}

func (f *fakeOwnedAlbumUpdateQueries) GetOwnedAlbumThumbnailForUpdate(_ context.Context, params GetOwnedAlbumThumbnailForUpdateParams) (Asset, error) {
	f.thumbCalls = append(f.thumbCalls, params)
	return Asset{ID: params.AssetID, OwnerId: params.OwnerID}, f.thumbnailErr
}

func (f *fakeOwnedAlbumUpdateQueries) UpdateAlbum(_ context.Context, params UpdateAlbumParams) (Album, error) {
	f.updateCalls = append(f.updateCalls, params)
	return f.updated, f.updateErr
}

func albumUpdateUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func TestUpdateOwnedAlbumInTransactionValidatesThumbnailBeforeUpdate(t *testing.T) {
	ownerID := albumUpdateUUID(uuid.New())
	albumID := albumUpdateUUID(uuid.New())
	thumbnailID := albumUpdateUUID(uuid.New())
	want := Album{ID: albumID, OwnerId: ownerID, AlbumThumbnailAssetId: thumbnailID}
	store := &fakeOwnedAlbumUpdateQueries{updated: want}
	params := UpdateOwnedAlbumParams{
		OwnerID: ownerID,
		Update: UpdateAlbumParams{
			ID:                    albumID,
			AlbumName:             pgtype.Text{String: "safe", Valid: true},
			AlbumThumbnailAssetID: thumbnailID,
		},
	}

	got, err := updateOwnedAlbumInTransaction(context.Background(), store, params)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	require.Len(t, store.albumCalls, 1)
	assert.Equal(t, ownerID, store.albumCalls[0].OwnerID)
	require.Len(t, store.thumbCalls, 1)
	assert.Equal(t, thumbnailID, store.thumbCalls[0].AssetID)
	assert.Equal(t, albumID, store.thumbCalls[0].AlbumID)
	assert.Equal(t, ownerID, store.thumbCalls[0].OwnerID)
	require.Len(t, store.updateCalls, 1)
	assert.Equal(t, params.Update, store.updateCalls[0])
}

func TestUpdateOwnedAlbumInTransactionRejectsBeforeUpdate(t *testing.T) {
	ownerID := albumUpdateUUID(uuid.New())
	albumID := albumUpdateUUID(uuid.New())
	thumbnailID := albumUpdateUUID(uuid.New())
	params := UpdateOwnedAlbumParams{
		OwnerID: ownerID,
		Update: UpdateAlbumParams{
			ID:                    albumID,
			AlbumThumbnailAssetID: thumbnailID,
		},
	}

	tests := []struct {
		name         string
		albumErr     error
		thumbnailErr error
		wantErr      error
	}{
		{name: "album not owned", albumErr: pgx.ErrNoRows, wantErr: ErrAlbumUpdateAccessDenied},
		{name: "thumbnail not owned member", thumbnailErr: pgx.ErrNoRows, wantErr: ErrAlbumThumbnailInvalid},
		{name: "album lookup failure", albumErr: errors.New("database unavailable"), wantErr: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeOwnedAlbumUpdateQueries{albumErr: tt.albumErr, thumbnailErr: tt.thumbnailErr}
			_, err := updateOwnedAlbumInTransaction(context.Background(), store, params)
			require.Error(t, err)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
			assert.Empty(t, store.updateCalls)
		})
	}
}

func TestUpdateOwnedAlbumInTransactionSkipsThumbnailLookupWhenUnchanged(t *testing.T) {
	ownerID := albumUpdateUUID(uuid.New())
	albumID := albumUpdateUUID(uuid.New())
	store := &fakeOwnedAlbumUpdateQueries{updated: Album{ID: albumID, OwnerId: ownerID}}

	_, err := updateOwnedAlbumInTransaction(context.Background(), store, UpdateOwnedAlbumParams{
		OwnerID: ownerID,
		Update:  UpdateAlbumParams{ID: albumID},
	})
	require.NoError(t, err)
	assert.Empty(t, store.thumbCalls)
	require.Len(t, store.updateCalls, 1)
}
