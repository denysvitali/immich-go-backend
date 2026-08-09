package sqlc

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrAssetOriginalFileWriteRejected = errors.New("asset original-file write rejected")

// AssetOriginalFileWriter writes an asset's original object while the caller
// holds a database row lock that serializes the write with permanent deletion.
type AssetOriginalFileWriter func(context.Context, Asset) error

type CompleteAssetUploadParams struct {
	AssetID           pgtype.UUID
	WriteOriginalFile AssetOriginalFileWriter
}

type ReplaceAssetOriginalFileParams struct {
	AssetID           pgtype.UUID
	OwnerID           pgtype.UUID
	Checksum          []byte
	FileModifiedAt    pgtype.Timestamptz
	WriteOriginalFile AssetOriginalFileWriter
}

type completeAssetUploadQueries interface {
	GetAssetForOriginalFileWrite(context.Context, pgtype.UUID) (Asset, error)
	CompleteAssetOriginalFileWrite(context.Context, pgtype.UUID) (Asset, error)
}

type replaceAssetOriginalFileQueries interface {
	GetOwnedAssetForOriginalFileWrite(context.Context, GetOwnedAssetForOriginalFileWriteParams) (Asset, error)
	ReplaceAssetFile(context.Context, ReplaceAssetFileParams) (Asset, error)
}

// CompleteAssetUpload locks a live asset through its original-file write and
// status finalization. A concurrent permanent deletion either waits for this
// transaction or establishes deletion intent before the writer is invoked.
func (q *Queries) CompleteAssetUpload(ctx context.Context, params CompleteAssetUploadParams) (Asset, error) {
	if !params.AssetID.Valid || params.WriteOriginalFile == nil {
		return Asset{}, errors.New("complete asset upload requires an asset and original-file writer")
	}

	beginner, ok := q.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return Asset{}, errors.New("complete asset upload requires a transaction-capable database")
	}

	var updated Asset
	err := pgx.BeginFunc(ctx, beginner, func(tx pgx.Tx) error {
		var err error
		updated, err = completeAssetUploadInTransaction(ctx, q.WithTx(tx), params)
		return err
	})
	if err != nil {
		return Asset{}, err
	}
	return updated, nil
}

func completeAssetUploadInTransaction(
	ctx context.Context,
	queries completeAssetUploadQueries,
	params CompleteAssetUploadParams,
) (Asset, error) {
	asset, err := queries.GetAssetForOriginalFileWrite(ctx, params.AssetID)
	if err != nil {
		return Asset{}, originalFileLockError(err)
	}

	if err := params.WriteOriginalFile(ctx, asset); err != nil {
		return Asset{}, fmt.Errorf("write asset original file: %w", err)
	}

	updated, err := queries.CompleteAssetOriginalFileWrite(ctx, params.AssetID)
	if err != nil {
		return Asset{}, originalFileFinalizeError(err)
	}
	return updated, nil
}

// ReplaceAssetOriginalFile locks a caller-owned live asset through its storage
// replacement and metadata update, retaining the ownership check in the same
// transaction as the guarded write.
func (q *Queries) ReplaceAssetOriginalFile(ctx context.Context, params ReplaceAssetOriginalFileParams) (Asset, error) {
	if !params.AssetID.Valid || !params.OwnerID.Valid || params.WriteOriginalFile == nil {
		return Asset{}, errors.New("replace asset original file requires an asset, owner, and original-file writer")
	}

	beginner, ok := q.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return Asset{}, errors.New("replace asset original file requires a transaction-capable database")
	}

	var updated Asset
	err := pgx.BeginFunc(ctx, beginner, func(tx pgx.Tx) error {
		var err error
		updated, err = replaceAssetOriginalFileInTransaction(ctx, q.WithTx(tx), params)
		return err
	})
	if err != nil {
		return Asset{}, err
	}
	return updated, nil
}

func replaceAssetOriginalFileInTransaction(
	ctx context.Context,
	queries replaceAssetOriginalFileQueries,
	params ReplaceAssetOriginalFileParams,
) (Asset, error) {
	asset, err := queries.GetOwnedAssetForOriginalFileWrite(ctx, GetOwnedAssetForOriginalFileWriteParams{
		ID:      params.AssetID,
		OwnerId: params.OwnerID,
	})
	if err != nil {
		return Asset{}, originalFileLockError(err)
	}

	if err := params.WriteOriginalFile(ctx, asset); err != nil {
		return Asset{}, fmt.Errorf("write asset original file: %w", err)
	}

	updated, err := queries.ReplaceAssetFile(ctx, ReplaceAssetFileParams{
		Checksum:       params.Checksum,
		FileModifiedAt: params.FileModifiedAt,
		ID:             params.AssetID,
		OwnerID:        params.OwnerID,
	})
	if err != nil {
		return Asset{}, originalFileFinalizeError(err)
	}
	return updated, nil
}

func originalFileLockError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: asset is deleted or unavailable", ErrAssetOriginalFileWriteRejected)
	}
	return fmt.Errorf("lock asset for original-file write: %w", err)
}

func originalFileFinalizeError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: asset became unavailable", ErrAssetOriginalFileWriteRejected)
	}
	return fmt.Errorf("finalize asset original-file write: %w", err)
}
