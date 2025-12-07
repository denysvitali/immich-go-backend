package view

import (
	"context"
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

var tracer = telemetry.GetTracer("view")

// Service handles view operations for folder-based asset browsing
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// Metrics
	operationCounter  metric.Int64Counter
	operationDuration metric.Float64Histogram
}

// NewService creates a new view service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	meter := telemetry.GetMeter()

	operationCounter, err := meter.Int64Counter(
		"view_operations_total",
		metric.WithDescription("Total number of view operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation counter: %w", err)
	}

	operationDuration, err := meter.Float64Histogram(
		"view_operation_duration_seconds",
		metric.WithDescription("Time spent on view operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation duration histogram: %w", err)
	}

	return &Service{
		db:                queries,
		config:            cfg,
		operationCounter:  operationCounter,
		operationDuration: operationDuration,
	}, nil
}

// GetAssetsByOriginalPath retrieves assets by their original file path
func (s *Service) GetAssetsByOriginalPath(ctx context.Context, req GetAssetsByOriginalPathRequest) (*GetAssetsByOriginalPathResponse, error) {
	ctx, span := tracer.Start(ctx, "view.get_assets_by_original_path",
		trace.WithAttributes(attribute.String("path", req.Path)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_assets_by_original_path")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_assets_by_original_path")))
	}()

	// Convert UUID to pgtype.UUID
	ownerUUID := pgtype.UUID{Bytes: req.UserID, Valid: true}

	// Set default pagination
	limit := int32(100)
	offset := int32(0)
	if req.Take != nil {
		limit = *req.Take
	}
	if req.Skip != nil {
		offset = *req.Skip
	}

	// Build query params
	params := sqlc.GetAssetsByOriginalPathPrefixParams{
		OwnerId: ownerUUID,
		Column2: pgtype.Text{String: req.Path, Valid: true},
		Limit:   limit,
		Offset:  offset,
	}

	// Add optional filters
	if req.IsArchived != nil {
		params.IsArchived = pgtype.Bool{Bool: *req.IsArchived, Valid: true}
	}
	if req.IsFavorite != nil {
		params.IsFavorite = pgtype.Bool{Bool: *req.IsFavorite, Valid: true}
	}

	// Get assets from database
	assets, err := s.db.GetAssetsByOriginalPathPrefix(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get assets by path: %w", err)
	}

	// Get total count
	countParams := sqlc.CountAssetsByOriginalPathPrefixParams{
		OwnerId:    ownerUUID,
		Column2:    pgtype.Text{String: req.Path, Valid: true},
		IsArchived: params.IsArchived,
		IsFavorite: params.IsFavorite,
	}
	totalCount, err := s.db.CountAssetsByOriginalPathPrefix(ctx, countParams)
	if err != nil {
		return nil, fmt.Errorf("failed to count assets by path: %w", err)
	}

	// Convert to response format
	assetInfos := make([]*AssetInfo, len(assets))
	for i, asset := range assets {
		assetInfos[i] = convertAssetToAssetInfo(&asset)
	}

	return &GetAssetsByOriginalPathResponse{
		Assets: assetInfos,
		Total:  int32(totalCount),
	}, nil
}

// GetUniqueOriginalPaths retrieves all unique original file paths for a user
func (s *Service) GetUniqueOriginalPaths(ctx context.Context, userID uuid.UUID) (*GetUniqueOriginalPathsResponse, error) {
	ctx, span := tracer.Start(ctx, "view.get_unique_original_paths")
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_unique_original_paths")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_unique_original_paths")))
	}()

	// Convert UUID to pgtype.UUID
	ownerUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get unique paths from database
	paths, err := s.db.GetUniqueOriginalPathPrefixes(ctx, ownerUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique paths: %w", err)
	}

	return &GetUniqueOriginalPathsResponse{
		Paths: paths,
	}, nil
}

// Request/Response types

type GetAssetsByOriginalPathRequest struct {
	UserID     uuid.UUID
	Path       string
	IsArchived *bool
	IsFavorite *bool
	Skip       *int32
	Take       *int32
}

type GetAssetsByOriginalPathResponse struct {
	Assets []*AssetInfo
	Total  int32
}

type GetUniqueOriginalPathsResponse struct {
	Paths []string
}

type AssetInfo struct {
	ID               string
	DeviceAssetID    string
	DeviceID         string
	Type             AssetType
	OriginalPath     string
	OriginalFileName string
	IsArchived       bool
	IsFavorite       bool
	IsTrashed        bool
}

type AssetType int32

const (
	AssetType_IMAGE AssetType = 0
	AssetType_VIDEO AssetType = 1
	AssetType_AUDIO AssetType = 2
	AssetType_OTHER AssetType = 3
)

// convertAssetToAssetInfo converts a database asset to an AssetInfo struct
func convertAssetToAssetInfo(asset *sqlc.Asset) *AssetInfo {
	// Convert UUID to string
	assetID := uuid.UUID(asset.ID.Bytes).String()

	// Determine asset type from string
	var assetType AssetType
	switch asset.Type {
	case "IMAGE":
		assetType = AssetType_IMAGE
	case "VIDEO":
		assetType = AssetType_VIDEO
	case "AUDIO":
		assetType = AssetType_AUDIO
	default:
		assetType = AssetType_OTHER
	}

	// Check if archived (visibility == 'archive')
	isArchived := asset.Visibility == sqlc.AssetVisibilityEnumArchive

	// Check if trashed (status == 'trashed')
	isTrashed := asset.Status == sqlc.AssetsStatusEnumTrashed

	return &AssetInfo{
		ID:               assetID,
		DeviceAssetID:    asset.DeviceAssetId,
		DeviceID:         asset.DeviceId,
		Type:             assetType,
		OriginalPath:     asset.OriginalPath,
		OriginalFileName: asset.OriginalFileName,
		IsArchived:       isArchived,
		IsFavorite:       asset.IsFavorite,
		IsTrashed:        isTrashed,
	}
}
