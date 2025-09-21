package systemmetadata

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

var tracer = telemetry.GetTracer("systemmetadata")

// Service handles system metadata operations
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// Metrics
	operationCounter  metric.Int64Counter
	operationDuration metric.Float64Histogram
}

// NewService creates a new system metadata service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	meter := telemetry.GetMeter()

	operationCounter, err := meter.Int64Counter(
		"systemmetadata_operations_total",
		metric.WithDescription("Total number of system metadata operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation counter: %w", err)
	}

	operationDuration, err := meter.Float64Histogram(
		"systemmetadata_operation_duration_seconds",
		metric.WithDescription("Time spent on system metadata operations"),
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

// GetAdminOnboarding retrieves admin onboarding status
func (s *Service) GetAdminOnboarding(ctx context.Context) (*GetAdminOnboardingResponse, error) {
	ctx, span := tracer.Start(ctx, "systemmetadata.get_admin_onboarding")
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_admin_onboarding")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_admin_onboarding")))
	}()

	// Get admin onboarding status from system metadata
	metadata, err := s.db.GetSystemMetadata(ctx, "admin_onboarding_completed")
	if err != nil {
		// If key doesn't exist, admin hasn't been onboarded yet
		return &GetAdminOnboardingResponse{
			IsOnboarded: false,
		}, nil
	}

	// Parse the value as boolean
	isOnboarded := false
	if len(metadata.Value) > 0 {
		isOnboarded = string(metadata.Value) == "true"
	}

	return &GetAdminOnboardingResponse{
		IsOnboarded: isOnboarded,
	}, nil
}

// UpdateAdminOnboarding updates admin onboarding status
func (s *Service) UpdateAdminOnboarding(ctx context.Context, req UpdateAdminOnboardingRequest) (*UpdateAdminOnboardingResponse, error) {
	ctx, span := tracer.Start(ctx, "systemmetadata.update_admin_onboarding",
		trace.WithAttributes(attribute.Bool("is_onboarded", req.IsOnboarded)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "update_admin_onboarding")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "update_admin_onboarding")))
	}()

	// Update admin onboarding status in system metadata
	value := "false"
	if req.IsOnboarded {
		value = "true"
	}

	_, err := s.db.SetSystemMetadata(ctx, sqlc.SetSystemMetadataParams{
		Key:   "admin_onboarding_completed",
		Value: []byte(value),
	})
	if err != nil {
		return nil, err
	}

	return &UpdateAdminOnboardingResponse{
		IsOnboarded: req.IsOnboarded,
	}, nil
}

// GetReverseGeocodingState retrieves reverse geocoding state
func (s *Service) GetReverseGeocodingState(ctx context.Context) (*GetReverseGeocodingStateResponse, error) {
	ctx, span := tracer.Start(ctx, "systemmetadata.get_reverse_geocoding_state")
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_reverse_geocoding_state")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_reverse_geocoding_state")))
	}()

	// Get reverse geocoding state from system metadata
	updateMetadata, _ := s.db.GetSystemMetadata(ctx, "reverse_geocoding_last_update")
	fileMetadata, _ := s.db.GetSystemMetadata(ctx, "reverse_geocoding_last_file")

	lastUpdate := int32(0)
	if len(updateMetadata.Value) > 0 {
		// Parse as int32
		fmt.Sscanf(string(updateMetadata.Value), "%d", &lastUpdate)
	}

	lastFile := int32(0)
	if len(fileMetadata.Value) > 0 {
		// Parse as int32
		fmt.Sscanf(string(fileMetadata.Value), "%d", &lastFile)
	}

	return &GetReverseGeocodingStateResponse{
		LastUpdate:         lastUpdate,
		LastImportFileName: lastFile,
	}, nil
}

// SetReverseGeocodingState updates reverse geocoding state
func (s *Service) SetReverseGeocodingState(ctx context.Context, lastUpdate int32, lastImportFileName int32) error {
	ctx, span := tracer.Start(ctx, "systemmetadata.set_reverse_geocoding_state",
		trace.WithAttributes(
			attribute.Int("last_update", int(lastUpdate)),
			attribute.Int("last_import_file_name", int(lastImportFileName)),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "set_reverse_geocoding_state")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "set_reverse_geocoding_state")))
	}()

	// Update reverse geocoding state in system metadata
	_, err := s.db.SetSystemMetadata(ctx, sqlc.SetSystemMetadataParams{
		Key:   "reverse_geocoding_last_update",
		Value: []byte(fmt.Sprintf("%d", lastUpdate)),
	})
	if err != nil {
		return err
	}

	_, err = s.db.SetSystemMetadata(ctx, sqlc.SetSystemMetadataParams{
		Key:   "reverse_geocoding_last_file",
		Value: []byte(fmt.Sprintf("%d", lastImportFileName)),
	})
	if err != nil {
		return err
	}

	return nil
}

// Request/Response types

type GetAdminOnboardingResponse struct {
	IsOnboarded bool
}

type UpdateAdminOnboardingRequest struct {
	IsOnboarded bool
}

type UpdateAdminOnboardingResponse struct {
	IsOnboarded bool
}

type GetReverseGeocodingStateResponse struct {
	LastUpdate         int32
	LastImportFileName int32
}
