package sqlc

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	// ErrAlbumUpdateAccessDenied means the requested album is not a live album
	// owned by the caller.
	ErrAlbumUpdateAccessDenied = errors.New("album update access denied")
	// ErrAlbumThumbnailInvalid means the requested thumbnail is not a live,
	// caller-owned member of the album.
	ErrAlbumThumbnailInvalid = errors.New("album thumbnail must be an owned album asset")
)

// UpdateOwnedAlbumParams carries an album update and the owner who is
// authorized to apply it.
type UpdateOwnedAlbumParams struct {
	OwnerID pgtype.UUID
	Update  UpdateAlbumParams
}

type ownedAlbumUpdateQueries interface {
	GetOwnedAlbumForUpdate(context.Context, GetOwnedAlbumForUpdateParams) (Album, error)
	GetOwnedAlbumThumbnailForUpdate(context.Context, GetOwnedAlbumThumbnailForUpdateParams) (Asset, error)
	UpdateAlbum(context.Context, UpdateAlbumParams) (Album, error)
}

// UpdateOwnedAlbum verifies album ownership and, when present, thumbnail
// ownership and membership in the same transaction as the update.
func (q *Queries) UpdateOwnedAlbum(ctx context.Context, params UpdateOwnedAlbumParams) (Album, error) {
	if !params.OwnerID.Valid || !params.Update.ID.Valid {
		return Album{}, errors.New("owned album update requires valid album and owner IDs")
	}

	beginner, ok := q.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return Album{}, errors.New("owned album update requires a transaction-capable database")
	}

	var updated Album
	err := pgx.BeginFunc(ctx, beginner, func(tx pgx.Tx) error {
		var err error
		updated, err = updateOwnedAlbumInTransaction(ctx, q.WithTx(tx), params)
		return err
	})
	if err != nil {
		return Album{}, err
	}
	return updated, nil
}

func updateOwnedAlbumInTransaction(ctx context.Context, queries ownedAlbumUpdateQueries, params UpdateOwnedAlbumParams) (Album, error) {
	_, err := queries.GetOwnedAlbumForUpdate(ctx, GetOwnedAlbumForUpdateParams{
		AlbumID: params.Update.ID,
		OwnerID: params.OwnerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Album{}, ErrAlbumUpdateAccessDenied
	}
	if err != nil {
		return Album{}, fmt.Errorf("lock owned album for update: %w", err)
	}

	if params.Update.AlbumThumbnailAssetID.Valid {
		_, err = queries.GetOwnedAlbumThumbnailForUpdate(ctx, GetOwnedAlbumThumbnailForUpdateParams{
			AlbumID: params.Update.ID,
			AssetID: params.Update.AlbumThumbnailAssetID,
			OwnerID: params.OwnerID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return Album{}, ErrAlbumThumbnailInvalid
		}
		if err != nil {
			return Album{}, fmt.Errorf("lock album thumbnail for update: %w", err)
		}
	}

	updated, err := queries.UpdateAlbum(ctx, params.Update)
	if err != nil {
		return Album{}, fmt.Errorf("update owned album: %w", err)
	}
	return updated, nil
}
