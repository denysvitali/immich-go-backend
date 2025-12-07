package trash

import (
	"context"
	"fmt"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service handles trash operations
type Service struct {
	db *sqlc.Queries
}

// NewService creates a new trash service
func NewService(queries *sqlc.Queries) *Service {
	return &Service{db: queries}
}

// TrashedAsset represents an asset in trash
type TrashedAsset struct {
	ID           string
	DeviceID     string
	OriginalPath string
	Type         string
}

// GetTrashedAssets retrieves all trashed assets for a user
func (s *Service) GetTrashedAssets(ctx context.Context, userID string) ([]*TrashedAsset, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	assets, err := s.db.GetTrashedAssetsByUser(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trashed assets: %w", err)
	}

	result := make([]*TrashedAsset, len(assets))
	for i, asset := range assets {
		result[i] = &TrashedAsset{
			ID:           uuid.UUID(asset.ID.Bytes).String(),
			DeviceID:     asset.DeviceId,
			OriginalPath: asset.OriginalPath,
			Type:         asset.Type,
		}
	}

	return result, nil
}

// TrashAsset moves an asset to trash (soft delete)
func (s *Service) TrashAsset(ctx context.Context, userID, assetID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	aid, err := uuid.Parse(assetID)
	if err != nil {
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	assetUUID := pgtype.UUID{Bytes: aid, Valid: true}

	// Verify ownership first
	asset, err := s.db.GetAsset(ctx, assetUUID)
	if err != nil {
		return fmt.Errorf("asset not found: %w", err)
	}

	if asset.OwnerId.Bytes != uid {
		return fmt.Errorf("access denied: asset does not belong to user")
	}

	// Soft delete the asset using MoveAssetToTrash
	err = s.db.MoveAssetToTrash(ctx, assetUUID)
	if err != nil {
		return fmt.Errorf("failed to trash asset: %w", err)
	}

	return nil
}

// TrashAssets moves multiple assets to trash
func (s *Service) TrashAssets(ctx context.Context, userID string, assetIDs []string) (int, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	var assetUUIDs []pgtype.UUID
	for _, assetIDStr := range assetIDs {
		assetID, err := uuid.Parse(assetIDStr)
		if err != nil {
			continue // Skip invalid IDs
		}
		assetUUIDs = append(assetUUIDs, pgtype.UUID{Bytes: assetID, Valid: true})
	}

	if len(assetUUIDs) == 0 {
		return 0, nil
	}

	// Soft delete assets owned by the user
	err = s.db.MoveAssetsToTrash(ctx, sqlc.MoveAssetsToTrashParams{
		Column1: assetUUIDs,
		OwnerId: userUUID,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to trash assets: %w", err)
	}

	return len(assetUUIDs), nil
}

// RestoreAsset restores an asset from trash
func (s *Service) RestoreAsset(ctx context.Context, userID, assetID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	aid, err := uuid.Parse(assetID)
	if err != nil {
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	assetUUID := pgtype.UUID{Bytes: aid, Valid: true}

	// Verify ownership first
	asset, err := s.db.GetAsset(ctx, assetUUID)
	if err != nil {
		return fmt.Errorf("asset not found: %w", err)
	}

	if asset.OwnerId.Bytes != uid {
		return fmt.Errorf("access denied: asset does not belong to user")
	}

	// Restore the asset
	err = s.db.RestoreAssetFromTrash(ctx, assetUUID)
	if err != nil {
		return fmt.Errorf("failed to restore asset: %w", err)
	}

	return nil
}

// RestoreAssets restores multiple assets from trash
func (s *Service) RestoreAssets(ctx context.Context, userID string, assetIDs []string) (int, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	var assetUUIDs []pgtype.UUID
	for _, assetIDStr := range assetIDs {
		assetID, err := uuid.Parse(assetIDStr)
		if err != nil {
			continue // Skip invalid IDs
		}
		assetUUIDs = append(assetUUIDs, pgtype.UUID{Bytes: assetID, Valid: true})
	}

	if len(assetUUIDs) == 0 {
		return 0, nil
	}

	// Restore assets owned by the user
	err = s.db.RestoreAssets(ctx, sqlc.RestoreAssetsParams{
		Column1: assetUUIDs,
		OwnerId: userUUID,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to restore assets: %w", err)
	}

	return len(assetUUIDs), nil
}

// RestoreAllAssets restores all trashed assets for a user
func (s *Service) RestoreAllAssets(ctx context.Context, userID string) (int, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get all trashed assets
	assets, err := s.db.GetTrashedAssetsByUser(ctx, userUUID)
	if err != nil {
		return 0, fmt.Errorf("failed to get trashed assets: %w", err)
	}

	// Restore each asset
	restored := 0
	for _, asset := range assets {
		err = s.db.RestoreAssetFromTrash(ctx, asset.ID)
		if err == nil {
			restored++
		}
	}

	return restored, nil
}

// EmptyTrash permanently deletes all trashed assets for a user
func (s *Service) EmptyTrash(ctx context.Context, userID string) (int, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get all trashed assets
	assets, err := s.db.GetTrashedAssetsByUser(ctx, userUUID)
	if err != nil {
		return 0, fmt.Errorf("failed to get trashed assets: %w", err)
	}

	// Permanently delete each asset
	deleted := 0
	for _, asset := range assets {
		err = s.db.PermanentlyDeleteAsset(ctx, asset.ID)
		if err == nil {
			deleted++
		}
	}

	return deleted, nil
}

// PermanentlyDeleteAsset permanently deletes a single asset
func (s *Service) PermanentlyDeleteAsset(ctx context.Context, userID, assetID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	aid, err := uuid.Parse(assetID)
	if err != nil {
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	assetUUID := pgtype.UUID{Bytes: aid, Valid: true}

	// Verify ownership first
	asset, err := s.db.GetAsset(ctx, assetUUID)
	if err != nil {
		return fmt.Errorf("asset not found: %w", err)
	}

	if asset.OwnerId.Bytes != uid {
		return fmt.Errorf("access denied: asset does not belong to user")
	}

	// Permanently delete the asset
	err = s.db.PermanentlyDeleteAsset(ctx, assetUUID)
	if err != nil {
		return fmt.Errorf("failed to permanently delete asset: %w", err)
	}

	return nil
}
