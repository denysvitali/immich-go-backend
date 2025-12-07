package stacks

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

var tracer = telemetry.GetTracer("stacks")

// Service handles stack operations for grouping burst photos
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// Metrics
	stackCounter      metric.Int64UpDownCounter
	operationCounter  metric.Int64Counter
	operationDuration metric.Float64Histogram
}

// NewService creates a new stack service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	meter := telemetry.GetMeter()

	stackCounter, err := meter.Int64UpDownCounter(
		"stacks_total",
		metric.WithDescription("Total number of stacks in the system"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stack counter: %w", err)
	}

	operationCounter, err := meter.Int64Counter(
		"stack_operations_total",
		metric.WithDescription("Total number of stack operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation counter: %w", err)
	}

	operationDuration, err := meter.Float64Histogram(
		"stack_operation_duration_seconds",
		metric.WithDescription("Time spent on stack operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation duration histogram: %w", err)
	}

	return &Service{
		db:                queries,
		config:            cfg,
		stackCounter:      stackCounter,
		operationCounter:  operationCounter,
		operationDuration: operationDuration,
	}, nil
}

// Helper functions for UUID conversion
func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func pgtypeToUUID(pg pgtype.UUID) uuid.UUID {
	if !pg.Valid {
		return uuid.Nil
	}
	return pg.Bytes
}

func stringToUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

func stringsToUUIDs(strs []string) ([]uuid.UUID, error) {
	uuids := make([]uuid.UUID, len(strs))
	for i, s := range strs {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID at index %d: %w", i, err)
		}
		uuids[i] = id
	}
	return uuids, nil
}

func uuidsToPgtype(ids []uuid.UUID) []pgtype.UUID {
	pgUUIDs := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		pgUUIDs[i] = uuidToPgtype(id)
	}
	return pgUUIDs
}

// CreateStack creates a new asset stack
func (s *Service) CreateStack(ctx context.Context, req CreateStackRequest) (*StackResponse, error) {
	ctx, span := tracer.Start(ctx, "stacks.create_stack",
		trace.WithAttributes(attribute.Int("asset_count", len(req.AssetIDs))))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "create_stack")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "create_stack")))
	}()

	if len(req.AssetIDs) == 0 {
		return nil, fmt.Errorf("at least one asset ID is required")
	}

	// Parse asset IDs
	assetUUIDs, err := stringsToUUIDs(req.AssetIDs)
	if err != nil {
		return nil, fmt.Errorf("invalid asset IDs: %w", err)
	}

	// The first asset becomes the primary asset
	primaryAssetID := assetUUIDs[0]

	// Get the owner from the primary asset
	asset, err := s.db.GetAsset(ctx, uuidToPgtype(primaryAssetID))
	if err != nil {
		return nil, fmt.Errorf("failed to get primary asset: %w", err)
	}

	// Create the stack
	stack, err := s.db.CreateStack(ctx, sqlc.CreateStackParams{
		PrimaryAssetId: uuidToPgtype(primaryAssetID),
		OwnerId:        asset.OwnerId,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stack: %w", err)
	}

	// Add all assets to the stack
	err = s.db.AddAssetsToStack(ctx, sqlc.AddAssetsToStackParams{
		StackId: stack.ID,
		Column2: uuidsToPgtype(assetUUIDs),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add assets to stack: %w", err)
	}

	// Update metrics
	s.stackCounter.Add(ctx, 1)

	// Return the stack response
	return &StackResponse{
		ID:             pgtypeToUUID(stack.ID).String(),
		PrimaryAssetID: pgtypeToUUID(stack.PrimaryAssetId).String(),
		AssetIDs:       req.AssetIDs,
		AssetCount:     int32(len(req.AssetIDs)),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}, nil
}

// GetStack retrieves a stack by ID
func (s *Service) GetStack(ctx context.Context, stackID string) (*StackResponse, error) {
	ctx, span := tracer.Start(ctx, "stacks.get_stack",
		trace.WithAttributes(attribute.String("stack_id", stackID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_stack")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_stack")))
	}()

	// Parse stack ID
	stackUUID, err := stringToUUID(stackID)
	if err != nil {
		return nil, fmt.Errorf("invalid stack ID: %w", err)
	}

	// Get stack with asset count
	stack, err := s.db.GetStackWithAssets(ctx, uuidToPgtype(stackUUID))
	if err != nil {
		return nil, fmt.Errorf("failed to get stack: %w", err)
	}

	// Get assets in the stack
	assets, err := s.db.GetStackAssets(ctx, uuidToPgtype(stackUUID))
	if err != nil {
		return nil, fmt.Errorf("failed to get stack assets: %w", err)
	}

	// Convert asset IDs to strings
	assetIDs := make([]string, len(assets))
	for i, asset := range assets {
		assetIDs[i] = pgtypeToUUID(asset.ID).String()
	}

	return &StackResponse{
		ID:             pgtypeToUUID(stack.ID).String(),
		PrimaryAssetID: pgtypeToUUID(stack.PrimaryAssetId).String(),
		AssetIDs:       assetIDs,
		AssetCount:     int32(stack.AssetCount),
		CreatedAt:      time.Now(), // Stack table doesn't have timestamps
		UpdatedAt:      time.Now(),
	}, nil
}

// UpdateStack updates an existing stack
func (s *Service) UpdateStack(ctx context.Context, stackID string, req UpdateStackRequest) (*StackResponse, error) {
	ctx, span := tracer.Start(ctx, "stacks.update_stack",
		trace.WithAttributes(attribute.String("stack_id", stackID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "update_stack")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "update_stack")))
	}()

	// Parse stack ID
	stackUUID, err := stringToUUID(stackID)
	if err != nil {
		return nil, fmt.Errorf("invalid stack ID: %w", err)
	}

	// Update primary asset if specified
	if req.PrimaryAssetID != nil {
		primaryUUID, err := stringToUUID(*req.PrimaryAssetID)
		if err != nil {
			return nil, fmt.Errorf("invalid primary asset ID: %w", err)
		}

		_, err = s.db.UpdateStackPrimaryAsset(ctx, sqlc.UpdateStackPrimaryAssetParams{
			ID:             uuidToPgtype(stackUUID),
			PrimaryAssetId: uuidToPgtype(primaryUUID),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update stack primary asset: %w", err)
		}
	}

	// Return updated stack
	return s.GetStack(ctx, stackID)
}

// DeleteStack removes a stack
func (s *Service) DeleteStack(ctx context.Context, stackID string) error {
	ctx, span := tracer.Start(ctx, "stacks.delete_stack",
		trace.WithAttributes(attribute.String("stack_id", stackID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "delete_stack")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "delete_stack")))
	}()

	// Parse stack ID
	stackUUID, err := stringToUUID(stackID)
	if err != nil {
		return fmt.Errorf("invalid stack ID: %w", err)
	}

	pgStackID := uuidToPgtype(stackUUID)

	// Clear stack references from assets first
	err = s.db.ClearStackAssets(ctx, pgStackID)
	if err != nil {
		return fmt.Errorf("failed to clear stack assets: %w", err)
	}

	// Delete the stack
	err = s.db.DeleteStack(ctx, pgStackID)
	if err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	// Update metrics
	s.stackCounter.Add(ctx, -1)

	return nil
}

// DeleteStacks removes multiple stacks
func (s *Service) DeleteStacks(ctx context.Context, stackIDs []string) error {
	ctx, span := tracer.Start(ctx, "stacks.delete_stacks",
		trace.WithAttributes(attribute.Int("stack_count", len(stackIDs))))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "delete_stacks")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "delete_stacks")))
	}()

	if len(stackIDs) == 0 {
		return nil
	}

	// Parse stack IDs
	stackUUIDs, err := stringsToUUIDs(stackIDs)
	if err != nil {
		return fmt.Errorf("invalid stack IDs: %w", err)
	}

	// Clear stack references from assets for each stack
	for _, stackUUID := range stackUUIDs {
		err = s.db.ClearStackAssets(ctx, uuidToPgtype(stackUUID))
		if err != nil {
			return fmt.Errorf("failed to clear stack assets: %w", err)
		}
	}

	// Delete all stacks
	err = s.db.DeleteStacksByIds(ctx, uuidsToPgtype(stackUUIDs))
	if err != nil {
		return fmt.Errorf("failed to delete stacks: %w", err)
	}

	// Update metrics
	s.stackCounter.Add(ctx, int64(-len(stackIDs)))

	return nil
}

// SearchStacks searches for stacks based on criteria
func (s *Service) SearchStacks(ctx context.Context, req SearchStacksRequest) (*SearchStacksResponse, error) {
	ctx, span := tracer.Start(ctx, "stacks.search_stacks")
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "search_stacks")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "search_stacks")))
	}()

	if req.UserID == nil {
		return nil, fmt.Errorf("user ID is required for search")
	}

	// Parse user ID
	userUUID, err := stringToUUID(*req.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Build search params
	params := sqlc.SearchStacksParams{
		OwnerId: uuidToPgtype(userUUID),
		Limit:   100, // Default limit
		Offset:  0,
	}

	// Add primary asset filter if specified
	if req.PrimaryAssetID != nil {
		primaryUUID, err := stringToUUID(*req.PrimaryAssetID)
		if err != nil {
			return nil, fmt.Errorf("invalid primary asset ID: %w", err)
		}
		params.PrimaryAssetID = uuidToPgtype(primaryUUID)
	}

	// Execute search
	stacks, err := s.db.SearchStacks(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to search stacks: %w", err)
	}

	// Convert to response
	results := make([]*StackResponse, len(stacks))
	for i, stack := range stacks {
		// Get assets for this stack
		assets, err := s.db.GetStackAssets(ctx, stack.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stack assets: %w", err)
		}

		assetIDs := make([]string, len(assets))
		for j, asset := range assets {
			assetIDs[j] = pgtypeToUUID(asset.ID).String()
		}

		results[i] = &StackResponse{
			ID:             pgtypeToUUID(stack.ID).String(),
			PrimaryAssetID: pgtypeToUUID(stack.PrimaryAssetId).String(),
			AssetIDs:       assetIDs,
			AssetCount:     int32(stack.AssetCount),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
	}

	return &SearchStacksResponse{
		Stacks: results,
	}, nil
}

// GetUserStacks retrieves all stacks for a user with pagination
func (s *Service) GetUserStacks(ctx context.Context, userID string, limit, offset int32) (*SearchStacksResponse, error) {
	ctx, span := tracer.Start(ctx, "stacks.get_user_stacks",
		trace.WithAttributes(attribute.String("user_id", userID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_user_stacks")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_user_stacks")))
	}()

	// Parse user ID
	userUUID, err := stringToUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get stacks
	stacks, err := s.db.GetUserStacks(ctx, sqlc.GetUserStacksParams{
		OwnerId: uuidToPgtype(userUUID),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user stacks: %w", err)
	}

	// Convert to response
	results := make([]*StackResponse, len(stacks))
	for i, stack := range stacks {
		// Get assets for this stack
		assets, err := s.db.GetStackAssets(ctx, stack.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stack assets: %w", err)
		}

		assetIDs := make([]string, len(assets))
		for j, asset := range assets {
			assetIDs[j] = pgtypeToUUID(asset.ID).String()
		}

		results[i] = &StackResponse{
			ID:             pgtypeToUUID(stack.ID).String(),
			PrimaryAssetID: pgtypeToUUID(stack.PrimaryAssetId).String(),
			AssetIDs:       assetIDs,
			AssetCount:     int32(stack.AssetCount),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
	}

	return &SearchStacksResponse{
		Stacks: results,
	}, nil
}

// AddAssetsToStack adds assets to an existing stack
func (s *Service) AddAssetsToStack(ctx context.Context, stackID string, assetIDs []string) error {
	ctx, span := tracer.Start(ctx, "stacks.add_assets_to_stack",
		trace.WithAttributes(
			attribute.String("stack_id", stackID),
			attribute.Int("asset_count", len(assetIDs))))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "add_assets_to_stack")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "add_assets_to_stack")))
	}()

	// Parse stack ID
	stackUUID, err := stringToUUID(stackID)
	if err != nil {
		return fmt.Errorf("invalid stack ID: %w", err)
	}

	// Parse asset IDs
	assetUUIDs, err := stringsToUUIDs(assetIDs)
	if err != nil {
		return fmt.Errorf("invalid asset IDs: %w", err)
	}

	// Add assets to stack
	err = s.db.AddAssetsToStack(ctx, sqlc.AddAssetsToStackParams{
		StackId: uuidToPgtype(stackUUID),
		Column2: uuidsToPgtype(assetUUIDs),
	})
	if err != nil {
		return fmt.Errorf("failed to add assets to stack: %w", err)
	}

	return nil
}

// RemoveAssetsFromStack removes assets from a stack
func (s *Service) RemoveAssetsFromStack(ctx context.Context, assetIDs []string) error {
	ctx, span := tracer.Start(ctx, "stacks.remove_assets_from_stack",
		trace.WithAttributes(attribute.Int("asset_count", len(assetIDs))))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "remove_assets_from_stack")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "remove_assets_from_stack")))
	}()

	// Parse asset IDs
	assetUUIDs, err := stringsToUUIDs(assetIDs)
	if err != nil {
		return fmt.Errorf("invalid asset IDs: %w", err)
	}

	// Remove assets from stack
	err = s.db.RemoveAssetsFromStack(ctx, uuidsToPgtype(assetUUIDs))
	if err != nil {
		return fmt.Errorf("failed to remove assets from stack: %w", err)
	}

	return nil
}

// Request/Response types

type CreateStackRequest struct {
	AssetIDs []string
}

type UpdateStackRequest struct {
	PrimaryAssetID *string
}

type SearchStacksRequest struct {
	UserID         *string
	PrimaryAssetID *string
}

type SearchStacksResponse struct {
	Stacks []*StackResponse
}

type StackResponse struct {
	ID             string
	PrimaryAssetID string
	AssetIDs       []string
	AssetCount     int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
