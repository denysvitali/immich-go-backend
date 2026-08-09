package sqlc

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// FinalizeEncodedVideoParams identifies a generated encoded video that should
// become visible on an active asset.
type FinalizeEncodedVideoParams struct {
	AssetID pgtype.UUID
	Path    string
}

type encodedVideoQueries interface {
	UpdateAssetEncodedVideoPath(context.Context, UpdateAssetEncodedVideoPathParams) (Asset, error)
	CreateAssetFile(context.Context, CreateAssetFileParams) (AssetFile, error)
}

// FinalizeEncodedVideo publishes the asset field and cleanup record in one
// transaction. The generated queries reject deletion-intent assets and lock
// the parent row, serializing this operation with permanent deletion.
func (q *Queries) FinalizeEncodedVideo(ctx context.Context, params FinalizeEncodedVideoParams) (Asset, error) {
	if !params.AssetID.Valid || params.Path == "" {
		return Asset{}, errors.New("encoded video finalization requires an asset and path")
	}

	beginner, ok := q.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return Asset{}, errors.New("encoded video finalization requires a transaction-capable database")
	}

	var updated Asset
	err := pgx.BeginFunc(ctx, beginner, func(tx pgx.Tx) error {
		var err error
		updated, err = finalizeEncodedVideoInTransaction(ctx, q.WithTx(tx), params)
		return err
	})
	if err != nil {
		return Asset{}, err
	}
	return updated, nil
}

func finalizeEncodedVideoInTransaction(
	ctx context.Context,
	queries encodedVideoQueries,
	params FinalizeEncodedVideoParams,
) (Asset, error) {
	updated, err := queries.UpdateAssetEncodedVideoPath(ctx, UpdateAssetEncodedVideoPathParams{
		ID:               params.AssetID,
		EncodedVideoPath: pgtype.Text{String: params.Path, Valid: true},
	})
	if err != nil {
		return Asset{}, fmt.Errorf("publish encoded video path: %w", err)
	}

	if _, err := queries.CreateAssetFile(ctx, CreateAssetFileParams{
		AssetId: params.AssetID,
		Type:    "encoded-video",
		Path:    params.Path,
	}); err != nil {
		return Asset{}, fmt.Errorf("record encoded video file: %w", err)
	}
	return updated, nil
}
