package faces

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

var tracer = telemetry.GetTracer("faces")

// Service handles face detection and recognition operations
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// Metrics
	faceCounter       metric.Int64UpDownCounter
	operationCounter  metric.Int64Counter
	operationDuration metric.Float64Histogram
}

// NewService creates a new faces service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	meter := telemetry.GetMeter()

	faceCounter, err := meter.Int64UpDownCounter(
		"faces_total",
		metric.WithDescription("Total number of faces in the system"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create face counter: %w", err)
	}

	operationCounter, err := meter.Int64Counter(
		"face_operations_total",
		metric.WithDescription("Total number of face operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation counter: %w", err)
	}

	operationDuration, err := meter.Float64Histogram(
		"face_operation_duration_seconds",
		metric.WithDescription("Time spent on face operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation duration histogram: %w", err)
	}

	return &Service{
		db:                queries,
		config:            cfg,
		faceCounter:       faceCounter,
		operationCounter:  operationCounter,
		operationDuration: operationDuration,
	}, nil
}

// GetFaces retrieves faces, optionally filtered by ID
func (s *Service) GetFaces(ctx context.Context, req GetFacesRequest) (*GetFacesResponse, error) {
	ctx, span := tracer.Start(ctx, "faces.get_faces")
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_faces")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_faces")))
	}()

	// TODO: Implement actual query when SQLC queries are available
	// This should retrieve faces, optionally filtered by face ID
	// For now, return empty response
	return &GetFacesResponse{
		Faces: []*FaceResponse{},
	}, nil
}

// CreateFace creates a new face detection record
func (s *Service) CreateFace(ctx context.Context, req CreateFaceRequest) (*FaceResponse, error) {
	ctx, span := tracer.Start(ctx, "faces.create_face",
		trace.WithAttributes(
			attribute.String("asset_id", req.AssetID),
			attribute.String("person_id", req.PersonID),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "create_face")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "create_face")))
	}()

	// TODO: Implement actual face creation when SQLC queries are available
	// This should:
	// 1. Validate asset exists and user has access
	// 2. Validate person exists and user has access
	// 3. Create face record with bounding box
	// For now, return a mock response
	faceID := uuid.New()

	s.faceCounter.Add(ctx, 1)

	return &FaceResponse{
		ID:           faceID.String(),
		AssetID:      req.AssetID,
		PersonID:     req.PersonID,
		BoundingBox:  req.BoundingBox,
		ImageWidth:   nil,
		ImageHeight:  nil,
	}, nil
}

// DeleteFace removes a face detection record
func (s *Service) DeleteFace(ctx context.Context, faceID string) error {
	ctx, span := tracer.Start(ctx, "faces.delete_face",
		trace.WithAttributes(attribute.String("face_id", faceID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "delete_face")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "delete_face")))
	}()

	// TODO: Implement actual face deletion when SQLC queries are available
	// This should verify user ownership before deletion
	s.faceCounter.Add(ctx, -1)

	return nil
}

// ReassignFacesById reassigns faces to a different person
func (s *Service) ReassignFacesById(ctx context.Context, faceID, personID string) (*ReassignFacesByIdResponse, error) {
	ctx, span := tracer.Start(ctx, "faces.reassign_faces_by_id",
		trace.WithAttributes(
			attribute.String("face_id", faceID),
			attribute.String("person_id", personID),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "reassign_faces_by_id")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "reassign_faces_by_id")))
	}()

	// TODO: Implement actual face reassignment when SQLC queries are available
	// This should:
	// 1. Verify user owns the face
	// 2. Verify user owns the target person
	// 3. Update face record to point to new person
	// For now, return empty response
	return &ReassignFacesByIdResponse{
		UpdatedFaces: []*FaceResponse{},
	}, nil
}

// Request/Response types

type GetFacesRequest struct {
	ID *string
}

type GetFacesResponse struct {
	Faces []*FaceResponse
}

type CreateFaceRequest struct {
	AssetID     string
	PersonID    string
	BoundingBox BoundingBox
}

type ReassignFacesByIdResponse struct {
	UpdatedFaces []*FaceResponse
}

type FaceResponse struct {
	ID          string
	AssetID     string
	PersonID    string
	BoundingBox BoundingBox
	ImageWidth  *string
	ImageHeight *string
}

type BoundingBox struct {
	X1 int32
	Y1 int32
	X2 int32
	Y2 int32
}