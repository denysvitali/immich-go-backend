package stacks

import (
	"context"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"github.com/google/uuid"
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

	// TODO: Implement actual stack creation when SQLC queries are available
	// For now, return a mock response
	stackID := uuid.New()
	primaryAssetID := ""
	if len(req.AssetIDs) > 0 {
		primaryAssetID = req.AssetIDs[0]
	}

	s.stackCounter.Add(ctx, 1)

	return &StackResponse{
		ID:             stackID.String(),
		PrimaryAssetID: primaryAssetID,
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

	// TODO: Implement actual stack retrieval when SQLC queries are available
	// For now, return a mock response
	return &StackResponse{
		ID:             stackID,
		PrimaryAssetID: "",
		AssetIDs:       []string{},
		AssetCount:     0,
		CreatedAt:      time.Now(),
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

	// TODO: Implement actual stack update when SQLC queries are available
	// For now, return a mock response
	primaryAssetID := ""
	if req.PrimaryAssetID != nil {
		primaryAssetID = *req.PrimaryAssetID
	}

	return &StackResponse{
		ID:             stackID,
		PrimaryAssetID: primaryAssetID,
		AssetIDs:       []string{},
		AssetCount:     0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}, nil
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

	// TODO: Implement actual stack deletion when SQLC queries are available
	// For now, just decrement counter
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

	// TODO: Implement actual bulk stack deletion when SQLC queries are available
	// For now, just decrement counter
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

	// TODO: Implement actual stack search when SQLC queries are available
	// For now, return empty response
	return &SearchStacksResponse{
		Stacks: []*StackResponse{},
	}, nil
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
