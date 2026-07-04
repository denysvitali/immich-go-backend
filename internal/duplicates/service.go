package duplicates

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var tracer = telemetry.GetTracer("duplicates")

// Service handles duplicate detection operations
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// Metrics
	operationCounter  metric.Int64Counter
	operationDuration metric.Float64Histogram
	duplicatesFound   metric.Int64UpDownCounter
}

// NewService creates a new duplicates service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	meter := telemetry.GetMeter()

	operationCounter, err := meter.Int64Counter(
		"duplicates_operations_total",
		metric.WithDescription("Total number of duplicates operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation counter: %w", err)
	}

	operationDuration, err := meter.Float64Histogram(
		"duplicates_operation_duration_seconds",
		metric.WithDescription("Time spent on duplicates operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation duration histogram: %w", err)
	}

	duplicatesFound, err := meter.Int64UpDownCounter(
		"duplicates_found_total",
		metric.WithDescription("Total number of duplicate assets found"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create duplicates found counter: %w", err)
	}

	return &Service{
		db:                queries,
		config:            cfg,
		operationCounter:  operationCounter,
		operationDuration: operationDuration,
		duplicatesFound:   duplicatesFound,
	}, nil
}

// GetAssetDuplicates retrieves duplicate assets for the user
func (s *Service) GetAssetDuplicates(ctx context.Context, userID string) (*GetAssetDuplicatesResponse, error) {
	ctx, span := tracer.Start(ctx, "duplicates.get_asset_duplicates",
		trace.WithAttributes(attribute.String("user_id", userID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_asset_duplicates")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_asset_duplicates")))
	}()

	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get duplicate assets for the user
	duplicateAssets, err := s.db.GetDuplicateAssets(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get duplicate assets: %w", err)
	}

	// Group duplicates by checksum
	duplicateMap := make(map[string][]*DuplicateAsset)
	for _, asset := range duplicateAssets {
		// Convert checksum bytes to string
		checksumStr := hex.EncodeToString(asset.Checksum)

		// Get exif data for file size
		exif, err := s.db.GetExifByAssetId(ctx, asset.ID)
		var fileSize int64
		if err == nil && exif.FileSizeInByte.Valid {
			fileSize = exif.FileSizeInByte.Int64
		}

		// Create duplicate asset
		dupAsset := &DuplicateAsset{
			AssetID:        uuid.UUID(asset.ID.Bytes).String(),
			DeviceAssetID:  asset.DeviceAssetId,
			DeviceID:       asset.DeviceId,
			Checksum:       checksumStr,
			Type:           s.convertAssetType(asset.Type),
			OriginalPath:   asset.OriginalPath,
			FileSizeInByte: fileSize,
		}

		duplicateMap[checksumStr] = append(duplicateMap[checksumStr], dupAsset)
	}

	// Build duplicate groups
	var duplicateGroups []*DuplicateGroup
	for checksum, assets := range duplicateMap {
		if len(assets) > 1 { // Only include groups with actual duplicates
			group := &DuplicateGroup{
				DuplicateID: checksum,
				Assets:      assets,
			}
			duplicateGroups = append(duplicateGroups, group)
		}
	}

	// Update metrics
	s.duplicatesFound.Add(ctx, int64(len(duplicateGroups)))

	return &GetAssetDuplicatesResponse{
		Duplicates: duplicateGroups,
	}, nil
}

// DeleteDuplicate clears a duplicate group for the user.
func (s *Service) DeleteDuplicate(ctx context.Context, userID string, duplicateID string) error {
	userUUID, err := parseUserUUID(userID)
	if err != nil {
		return err
	}
	return s.clearDuplicateGroup(ctx, userUUID, duplicateID)
}

// DeleteDuplicates clears the supplied duplicate groups. If no IDs are supplied,
// all currently detected duplicate groups for the user are cleared.
func (s *Service) DeleteDuplicates(ctx context.Context, userID string, duplicateIDs []string) error {
	if len(duplicateIDs) == 0 {
		groups, err := s.GetAssetDuplicates(ctx, userID)
		if err != nil {
			return err
		}
		for _, group := range groups.Duplicates {
			duplicateIDs = append(duplicateIDs, group.DuplicateID)
		}
	}

	userUUID, err := parseUserUUID(userID)
	if err != nil {
		return err
	}
	for _, duplicateID := range duplicateIDs {
		if err := s.clearDuplicateGroup(ctx, userUUID, duplicateID); err != nil {
			return err
		}
	}
	return nil
}

// ResolveDuplicates trashes selected assets and clears the associated duplicate groups.
func (s *Service) ResolveDuplicates(ctx context.Context, userID string, groups []*ResolveDuplicateGroup) ([]*DuplicateBulkIDResponse, error) {
	userUUID, err := parseUserUUID(userID)
	if err != nil {
		return nil, err
	}

	results := make([]*DuplicateBulkIDResponse, 0, len(groups))
	for _, group := range groups {
		result := &DuplicateBulkIDResponse{ID: group.DuplicateID, Success: true}

		assetIDs, err := parseAssetUUIDs(group.TrashAssetIDs)
		if err != nil {
			result.Success = false
			result.Error = "invalid_asset_id"
			result.ErrorMessage = err.Error()
			results = append(results, result)
			continue
		}

		if len(assetIDs) > 0 {
			if err := s.db.TrashAssetsByIDsAndOwner(ctx, sqlc.TrashAssetsByIDsAndOwnerParams{
				OwnerId: userUUID,
				Column2: assetIDs,
			}); err != nil {
				result.Success = false
				result.Error = "trash_failed"
				result.ErrorMessage = "failed to trash duplicate assets"
				results = append(results, result)
				continue
			}
		}

		if err := s.clearDuplicateGroup(ctx, userUUID, group.DuplicateID); err != nil {
			result.Success = false
			result.Error = "clear_failed"
			result.ErrorMessage = "failed to clear duplicate group"
		}
		results = append(results, result)
	}

	return results, nil
}

func (s *Service) clearDuplicateGroup(ctx context.Context, userUUID pgtype.UUID, duplicateID string) error {
	if parsed, err := uuid.Parse(duplicateID); err == nil {
		return s.db.ClearDuplicateGroupByID(ctx, sqlc.ClearDuplicateGroupByIDParams{
			OwnerId:     userUUID,
			DuplicateId: pgtype.UUID{Bytes: parsed, Valid: true},
		})
	}

	checksum, err := hex.DecodeString(duplicateID)
	if err != nil {
		return fmt.Errorf("invalid duplicate ID: %w", err)
	}
	return s.db.ClearDuplicateGroupByChecksum(ctx, sqlc.ClearDuplicateGroupByChecksumParams{
		OwnerId:  userUUID,
		Checksum: checksum,
	})
}

// FindDuplicatesByChecksum finds assets with identical checksums
func (s *Service) FindDuplicatesByChecksum(ctx context.Context, userID string, checksum string) ([]*DuplicateAsset, error) {
	ctx, span := tracer.Start(ctx, "duplicates.find_duplicates_by_checksum",
		trace.WithAttributes(
			attribute.String("user_id", userID),
			attribute.String("checksum", checksum),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "find_duplicates_by_checksum")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "find_duplicates_by_checksum")))
	}()

	// Parse user ID for ownership verification
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Decode checksum from hex string to bytes
	checksumBytes, err := hex.DecodeString(checksum)
	if err != nil {
		return nil, fmt.Errorf("invalid checksum format: %w", err)
	}

	// Get assets by checksum
	assets, err := s.db.GetAssetsByChecksum(ctx, checksumBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to get assets by checksum: %w", err)
	}

	// Filter by user ownership and convert to DuplicateAsset
	var duplicates []*DuplicateAsset
	for _, asset := range assets {
		// Only include assets owned by the user
		if asset.OwnerId.Valid && asset.OwnerId.Bytes == uid {
			// Get exif data for file size
			exif, err := s.db.GetExifByAssetId(ctx, asset.ID)
			var fileSize int64
			if err == nil && exif.FileSizeInByte.Valid {
				fileSize = exif.FileSizeInByte.Int64
			}

			dupAsset := &DuplicateAsset{
				AssetID:        uuid.UUID(asset.ID.Bytes).String(),
				DeviceAssetID:  asset.DeviceAssetId,
				DeviceID:       asset.DeviceId,
				Checksum:       checksum,
				Type:           s.convertAssetType(asset.Type),
				OriginalPath:   asset.OriginalPath,
				FileSizeInByte: fileSize,
			}
			duplicates = append(duplicates, dupAsset)
		}
	}

	return duplicates, nil
}

// FindDuplicatesBySize finds assets with identical file sizes
func (s *Service) FindDuplicatesBySize(ctx context.Context, userID string, sizeBytes int64) ([]*DuplicateAsset, error) {
	ctx, span := tracer.Start(ctx, "duplicates.find_duplicates_by_size",
		trace.WithAttributes(
			attribute.String("user_id", userID),
			attribute.Int64("size_bytes", sizeBytes),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "find_duplicates_by_size")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "find_duplicates_by_size")))
	}()

	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Server-side filter by ownerId + file size in a single query so we
	// don't cap at 1000 assets or N+1-fetch every exif row.
	assets, err := s.db.GetAssetsByFileSizeAndUser(ctx, sqlc.GetAssetsByFileSizeAndUserParams{
		OwnerId:        userUUID,
		FileSizeInByte: pgtype.Int8{Int64: sizeBytes, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get assets by file size: %w", err)
	}

	duplicates := make([]*DuplicateAsset, 0, len(assets))
	for _, asset := range assets {
		duplicates = append(duplicates, &DuplicateAsset{
			AssetID:        uuid.UUID(asset.ID.Bytes).String(),
			DeviceAssetID:  asset.DeviceAssetId,
			DeviceID:       asset.DeviceId,
			Checksum:       hex.EncodeToString(asset.Checksum),
			Type:           s.convertAssetType(asset.Type),
			OriginalPath:   asset.OriginalPath,
			FileSizeInByte: sizeBytes,
		})
	}

	return duplicates, nil
}

// Request/Response types

type GetAssetDuplicatesResponse struct {
	Duplicates []*DuplicateGroup
}

type ResolveDuplicateGroup struct {
	DuplicateID   string
	KeepAssetIDs  []string
	TrashAssetIDs []string
}

type DuplicateBulkIDResponse struct {
	ID           string
	Success      bool
	Error        string
	ErrorMessage string
}

type DuplicateGroup struct {
	DuplicateID string
	Assets      []*DuplicateAsset
}

type DuplicateAsset struct {
	AssetID        string
	DeviceAssetID  string
	DeviceID       string
	Checksum       string
	Type           AssetType
	OriginalPath   string
	FileSizeInByte int64
}

type AssetType int32

const (
	AssetType_UNSPECIFIED AssetType = 0
	AssetType_IMAGE       AssetType = 1
	AssetType_VIDEO       AssetType = 2
	AssetType_AUDIO       AssetType = 3
	AssetType_OTHER       AssetType = 4
)

// convertAssetType converts database asset type string to AssetType enum
func (s *Service) convertAssetType(assetType string) AssetType {
	switch assetType {
	case "IMAGE":
		return AssetType_IMAGE
	case "VIDEO":
		return AssetType_VIDEO
	case "AUDIO":
		return AssetType_AUDIO
	default:
		return AssetType_OTHER
	}
}

func parseUserUUID(userID string) (pgtype.UUID, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid user ID: %w", err)
	}
	return pgtype.UUID{Bytes: uid, Valid: true}, nil
}

func parseAssetUUIDs(assetIDs []string) ([]pgtype.UUID, error) {
	ids := make([]pgtype.UUID, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		parsed, err := uuid.Parse(assetID)
		if err != nil {
			return nil, fmt.Errorf("invalid asset ID %q: %w", assetID, err)
		}
		ids = append(ids, pgtype.UUID{Bytes: parsed, Valid: true})
	}
	return ids, nil
}
