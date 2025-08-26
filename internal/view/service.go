package view

import (
	"context"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
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

	// TODO: Implement actual query when SQLC queries are available
	// For now, return empty response
	return &GetAssetsByOriginalPathResponse{
		Assets: []*AssetInfo{},
		Total:  0,
	}, nil
}

// GetUniqueOriginalPaths retrieves all unique original file paths
func (s *Service) GetUniqueOriginalPaths(ctx context.Context) (*GetUniqueOriginalPathsResponse, error) {
	ctx, span := tracer.Start(ctx, "view.get_unique_original_paths")
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_unique_original_paths")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_unique_original_paths")))
	}()

	// TODO: Implement actual query when SQLC queries are available
	// For now, return empty response
	return &GetUniqueOriginalPathsResponse{
		Paths: []string{},
	}, nil
}

// Request/Response types

type GetAssetsByOriginalPathRequest struct {
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
	ID                 string
	DeviceAssetID      string
	DeviceID           string
	Type               AssetType
	OriginalPath       string
	OriginalFileName   string
	IsArchived         bool
	IsFavorite         bool
	IsTrashed          bool
}

type AssetType int32

const (
	AssetType_IMAGE AssetType = 0
	AssetType_VIDEO AssetType = 1
	AssetType_AUDIO AssetType = 2
	AssetType_OTHER AssetType = 3
)