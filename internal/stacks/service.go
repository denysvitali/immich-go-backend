package stacks

import (
	"context"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
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

func stringsToPgtypeUUIDs(strs []string) ([]pgtype.UUID, error) {
	uuids := make([]pgtype.UUID, len(strs))
	for i, s := range strs {
		id, err := pgutil.StringToUUID(s)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID at index %d: %w", i, err)
		}
		uuids[i] = id
	}
	return uuids, nil
}

func userUUIDFromString(userID string) (pgtype.UUID, error) {
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid user ID: %w", err)
	}

	return userUUID, nil
}

func (s *Service) getOwnedStackUUID(ctx context.Context, stackID, userID string) (pgtype.UUID, error) {
	stackUUID, err := pgutil.StringToUUID(stackID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid stack ID: %w", err)
	}

	userUUID, err := userUUIDFromString(userID)
	if err != nil {
		return pgtype.UUID{}, err
	}

	stack, err := s.db.GetStack(ctx, stackUUID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("stack not found: %w", err)
	}
	if stack.OwnerId != userUUID {
		return pgtype.UUID{}, fmt.Errorf("access denied: stack is not owned by the user")
	}

	return stackUUID, nil
}

// CreateStack creates a new asset stack
func (s *Service) CreateStack(ctx context.Context, userID string, req CreateStackRequest) (*StackResponse, error) {
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

	userUUID, err := userUUIDFromString(userID)
	if err != nil {
		return nil, err
	}

	// Parse asset IDs
	assetUUIDs, err := stringsToPgtypeUUIDs(req.AssetIDs)
	if err != nil {
		return nil, fmt.Errorf("invalid asset IDs: %w", err)
	}

	// The first asset becomes the primary asset
	primaryAssetID := assetUUIDs[0]

	// Get the owner from the primary asset
	asset, err := s.db.GetAsset(ctx, primaryAssetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary asset: %w", err)
	}
	if asset.OwnerId != userUUID {
		return nil, fmt.Errorf("access denied: asset is not owned by the user")
	}

	for i, assetUUID := range assetUUIDs[1:] {
		stackAsset, err := s.db.GetAsset(ctx, assetUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stack asset at index %d: %w", i+1, err)
		}
		if stackAsset.OwnerId != userUUID {
			return nil, fmt.Errorf("access denied: asset is not owned by the user")
		}
	}

	// Create the stack
	stack, err := s.db.CreateStack(ctx, sqlc.CreateStackParams{
		PrimaryAssetId: primaryAssetID,
		OwnerId:        userUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stack: %w", err)
	}

	// Add all assets to the stack
	err = s.db.AddAssetsToStack(ctx, sqlc.AddAssetsToStackParams{
		StackId: stack.ID,
		Column2: assetUUIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add assets to stack: %w", err)
	}

	// Update metrics
	s.stackCounter.Add(ctx, 1)

	// Return the stack response
	return &StackResponse{
		ID:             pgutil.UUIDToString(stack.ID),
		PrimaryAssetID: pgutil.UUIDToString(stack.PrimaryAssetId),
		AssetIDs:       req.AssetIDs,
		AssetCount:     int32(len(req.AssetIDs)),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}, nil
}

// GetStack retrieves a stack by ID
func (s *Service) GetStack(ctx context.Context, userID, stackID string) (*StackResponse, error) {
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
	stackUUID, err := pgutil.StringToUUID(stackID)
	if err != nil {
		return nil, fmt.Errorf("invalid stack ID: %w", err)
	}

	userUUID, err := userUUIDFromString(userID)
	if err != nil {
		return nil, err
	}

	// Get stack with asset count
	stack, err := s.db.GetStackWithAssets(ctx, stackUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stack: %w", err)
	}
	if stack.OwnerId != userUUID {
		return nil, fmt.Errorf("access denied: stack is not owned by the user")
	}

	// Get assets in the stack
	assets, err := s.db.GetStackAssets(ctx, stackUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stack assets: %w", err)
	}

	// Convert asset IDs to strings
	assetIDs := make([]string, len(assets))
	for i, asset := range assets {
		assetIDs[i] = pgutil.UUIDToString(asset.ID)
	}

	return &StackResponse{
		ID:             pgutil.UUIDToString(stack.ID),
		PrimaryAssetID: pgutil.UUIDToString(stack.PrimaryAssetId),
		AssetIDs:       assetIDs,
		AssetCount:     int32(stack.AssetCount),
		CreatedAt:      time.Now(), // Stack table doesn't have timestamps
		UpdatedAt:      time.Now(),
	}, nil
}

// UpdateStack updates an existing stack
func (s *Service) UpdateStack(ctx context.Context, userID, stackID string, req UpdateStackRequest) (*StackResponse, error) {
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

	stackUUID, err := s.getOwnedStackUUID(ctx, stackID, userID)
	if err != nil {
		return nil, err
	}

	userUUID, err := userUUIDFromString(userID)
	if err != nil {
		return nil, err
	}

	// Update primary asset if specified
	if req.PrimaryAssetID != nil {
		primaryUUID, err := pgutil.StringToUUID(*req.PrimaryAssetID)
		if err != nil {
			return nil, fmt.Errorf("invalid primary asset ID: %w", err)
		}

		primaryAsset, err := s.db.GetAsset(ctx, primaryUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get primary asset: %w", err)
		}
		if primaryAsset.OwnerId != userUUID {
			return nil, fmt.Errorf("access denied: asset is not owned by the user")
		}

		assets, err := s.db.GetStackAssets(ctx, stackUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stack assets: %w", err)
		}
		found := false
		for _, asset := range assets {
			if asset.ID == primaryUUID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("asset not found: asset is not part of this stack")
		}

		_, err = s.db.UpdateStackPrimaryAsset(ctx, sqlc.UpdateStackPrimaryAssetParams{
			ID:             stackUUID,
			PrimaryAssetId: primaryUUID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update stack primary asset: %w", err)
		}
	}

	// Return updated stack
	return s.GetStack(ctx, userID, stackID)
}

// DeleteStack removes a stack
func (s *Service) DeleteStack(ctx context.Context, userID, stackID string) error {
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

	stackUUID, err := s.getOwnedStackUUID(ctx, stackID, userID)
	if err != nil {
		return err
	}

	pgStackID := stackUUID

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
func (s *Service) DeleteStacks(ctx context.Context, userID string, stackIDs []string) error {
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

	userUUID, err := userUUIDFromString(userID)
	if err != nil {
		return err
	}

	// Parse stack IDs
	stackUUIDs, err := stringsToPgtypeUUIDs(stackIDs)
	if err != nil {
		return fmt.Errorf("invalid stack IDs: %w", err)
	}

	for _, stackUUID := range stackUUIDs {
		stack, err := s.db.GetStack(ctx, stackUUID)
		if err != nil {
			return fmt.Errorf("stack not found: %w", err)
		}
		if stack.OwnerId != userUUID {
			return fmt.Errorf("access denied: stack is not owned by the user")
		}
	}

	// Clear stack references from assets for each stack
	for _, stackUUID := range stackUUIDs {
		err = s.db.ClearStackAssets(ctx, stackUUID)
		if err != nil {
			return fmt.Errorf("failed to clear stack assets: %w", err)
		}
	}

	// Delete all stacks
	err = s.db.DeleteStacksByIds(ctx, stackUUIDs)
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
	userUUID, err := pgutil.StringToUUID(*req.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Build search params
	params := sqlc.SearchStacksParams{
		OwnerId: userUUID,
		Limit:   100, // Default limit
		Offset:  0,
	}

	// Add primary asset filter if specified
	if req.PrimaryAssetID != nil {
		primaryUUID, err := pgutil.StringToUUID(*req.PrimaryAssetID)
		if err != nil {
			return nil, fmt.Errorf("invalid primary asset ID: %w", err)
		}
		params.PrimaryAssetID = primaryUUID
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
			assetIDs[j] = pgutil.UUIDToString(asset.ID)
		}

		results[i] = &StackResponse{
			ID:             pgutil.UUIDToString(stack.ID),
			PrimaryAssetID: pgutil.UUIDToString(stack.PrimaryAssetId),
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
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get stacks
	stacks, err := s.db.GetUserStacks(ctx, sqlc.GetUserStacksParams{
		OwnerId: userUUID,
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
			assetIDs[j] = pgutil.UUIDToString(asset.ID)
		}

		results[i] = &StackResponse{
			ID:             pgutil.UUIDToString(stack.ID),
			PrimaryAssetID: pgutil.UUIDToString(stack.PrimaryAssetId),
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
	stackUUID, err := pgutil.StringToUUID(stackID)
	if err != nil {
		return fmt.Errorf("invalid stack ID: %w", err)
	}

	// Parse asset IDs
	assetUUIDs, err := stringsToPgtypeUUIDs(assetIDs)
	if err != nil {
		return fmt.Errorf("invalid asset IDs: %w", err)
	}

	// Add assets to stack
	err = s.db.AddAssetsToStack(ctx, sqlc.AddAssetsToStackParams{
		StackId: stackUUID,
		Column2: assetUUIDs,
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
	assetUUIDs, err := stringsToPgtypeUUIDs(assetIDs)
	if err != nil {
		return fmt.Errorf("invalid asset IDs: %w", err)
	}

	// Remove assets from stack
	err = s.db.RemoveAssetsFromStack(ctx, assetUUIDs)
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

// RemoveAssetFromStack removes a single asset from a stack after verifying
// that the stack exists, is owned by the given user, and actually contains
// the asset.
func (s *Service) RemoveAssetFromStack(ctx context.Context, userID, stackID, assetID string) error {
	ctx, span := tracer.Start(ctx, "stacks.remove_asset_from_stack",
		trace.WithAttributes(
			attribute.String("stack_id", stackID),
			attribute.String("asset_id", assetID),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "remove_asset_from_stack")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "remove_asset_from_stack")))
	}()

	stackUUID, err := pgutil.StringToUUID(stackID)
	if err != nil {
		return fmt.Errorf("invalid stack ID: %w", err)
	}
	assetUUID, err := pgutil.StringToUUID(assetID)
	if err != nil {
		return fmt.Errorf("invalid asset ID: %w", err)
	}
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Verify the stack exists and is owned by the user.
	stack, err := s.db.GetStack(ctx, stackUUID)
	if err != nil {
		return fmt.Errorf("stack not found: %w", err)
	}
	if stack.OwnerId != userUUID {
		return fmt.Errorf("access denied: stack is not owned by the user")
	}

	// Verify the asset is actually part of this stack.
	assets, err := s.db.GetStackAssets(ctx, stackUUID)
	if err != nil {
		return fmt.Errorf("failed to load stack assets: %w", err)
	}
	found := false
	for _, asset := range assets {
		if asset.ID == assetUUID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("asset not found: asset is not part of this stack")
	}

	if err := s.db.RemoveAssetsFromStack(ctx, []pgtype.UUID{assetUUID}); err != nil {
		return fmt.Errorf("failed to remove asset from stack: %w", err)
	}

	return nil
}
