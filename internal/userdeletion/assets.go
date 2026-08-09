// Package userdeletion coordinates cleanup that must happen before a user row
// is permanently deleted.
package userdeletion

import (
	"context"
	"fmt"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
)

const assetPageSize = 1000

// AssetLister is the subset of generated queries needed to enumerate a user's
// all storage-bearing asset records before the user row is removed by a
// cascading delete, including tombstones left by older releases.
type AssetLister interface {
	GetAllUserAssetsForDeletion(ctx context.Context, arg sqlc.GetAllUserAssetsForDeletionParams) ([]sqlc.Asset, error)
}

// PermanentAssetDeleter owns the storage-first permanent asset deletion
// workflow. assets.Service implements this interface.
type PermanentAssetDeleter interface {
	HardDeleteAsset(ctx context.Context, assetID, userID uuid.UUID) error
}

// DeleteOwnedAssets removes every storage object tracked by a user's assets
// before their database records can be removed by a user-row cascade.
//
// The caller first fences the user in status=removing, so no new CreateAsset
// statement can pass its active-owner lock. Fixed-offset pages are therefore
// stable even though HardDeleteAsset intentionally retains DB tombstones.
func DeleteOwnedAssets(
	ctx context.Context,
	lister AssetLister,
	deleter PermanentAssetDeleter,
	userID uuid.UUID,
) error {
	if lister == nil {
		return fmt.Errorf("asset listing is not configured")
	}
	if deleter == nil {
		return fmt.Errorf("permanent asset deletion is not configured")
	}

	ownerID := pgutil.UUIDToPgtype(userID)
	var pageOffset int32
	for {
		page, err := lister.GetAllUserAssetsForDeletion(ctx, sqlc.GetAllUserAssetsForDeletionParams{
			OwnerID:    ownerID,
			PageOffset: pageOffset,
			PageSize:   assetPageSize,
		})
		if err != nil {
			return fmt.Errorf("list owned assets: %w", err)
		}
		if len(page) == 0 {
			return nil
		}

		for _, asset := range page {
			assetID := pgutil.PgtypeToUUID(asset.ID)
			if err := deleter.HardDeleteAsset(ctx, assetID, userID); err != nil {
				return fmt.Errorf("delete owned asset %s: %w", assetID, err)
			}
		}
		pageOffset += int32(len(page))
	}
}
