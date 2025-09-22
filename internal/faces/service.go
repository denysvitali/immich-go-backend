package faces

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

	// Get faces based on request parameters
	var faces []sqlc.AssetFace
	var err error

	if req.AssetID != "" {
		// Get faces by asset
		assetID, err := uuid.Parse(req.AssetID)
		if err != nil {
			return nil, fmt.Errorf("invalid asset ID: %w", err)
		}
		faces, err = s.db.GetFacesByAsset(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
		if err != nil {
			return nil, fmt.Errorf("failed to get faces for asset: %w", err)
		}
	} else if req.PersonID != "" {
		// Get faces by person
		personID, err := uuid.Parse(req.PersonID)
		if err != nil {
			return nil, fmt.Errorf("invalid person ID: %w", err)
		}
		faces, err = s.db.GetFacesByPerson(ctx, pgtype.UUID{Bytes: personID, Valid: true})
		if err != nil {
			return nil, fmt.Errorf("failed to get faces for person: %w", err)
		}
	} else {
		// Return empty if no filter specified
		faces = []sqlc.AssetFace{}
	}

	// Convert to response format
	response := &GetFacesResponse{
		Faces: make([]*FaceResponse, len(faces)),
	}
	for i, face := range faces {
		response.Faces[i] = s.convertToFaceResponse(face)
	}

	return response, nil
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

	// Parse IDs
	assetID, err := uuid.Parse(req.AssetID)
	if err != nil {
		return nil, fmt.Errorf("invalid asset ID: %w", err)
	}
	assetUUID := pgtype.UUID{Bytes: assetID, Valid: true}

	personID, err := uuid.Parse(req.PersonID)
	if err != nil {
		return nil, fmt.Errorf("invalid person ID: %w", err)
	}
	personUUID := pgtype.UUID{Bytes: personID, Valid: true}

	// Get image dimensions from the bounding box (if not provided separately)
	// For now, we'll use the bounding box max values as approximations
	imageWidth := req.BoundingBox.X2
	imageHeight := req.BoundingBox.Y2

	// Create face record in database
	face, err := s.db.CreateAssetFace(ctx, sqlc.CreateAssetFaceParams{
		AssetId:       assetUUID,
		PersonId:      personUUID,
		ImageWidth:    imageWidth,
		ImageHeight:   imageHeight,
		BoundingBoxX1: req.BoundingBox.X1,
		BoundingBoxY1: req.BoundingBox.Y1,
		BoundingBoxX2: req.BoundingBox.X2,
		BoundingBoxY2: req.BoundingBox.Y2,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create face: %w", err)
	}

	// Update face counter metric
	s.faceCounter.Add(ctx, 1)

	// Convert to response
	imgWidth := fmt.Sprintf("%d", face.ImageWidth)
	imgHeight := fmt.Sprintf("%d", face.ImageHeight)
	return &FaceResponse{
		ID:       uuid.UUID(face.ID.Bytes).String(),
		AssetID:  uuid.UUID(face.AssetId.Bytes).String(),
		PersonID: uuid.UUID(face.PersonId.Bytes).String(),
		BoundingBox: BoundingBox{
			X1: face.BoundingBoxX1,
			Y1: face.BoundingBoxY1,
			X2: face.BoundingBoxX2,
			Y2: face.BoundingBoxY2,
		},
		ImageWidth:  &imgWidth,
		ImageHeight: &imgHeight,
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

	// Parse face ID
	fID, err := uuid.Parse(faceID)
	if err != nil {
		return fmt.Errorf("invalid face ID: %w", err)
	}
	faceUUID := pgtype.UUID{Bytes: fID, Valid: true}

	// Delete the face record
	// Note: In production, you would want to verify ownership first
	// by joining with the asset and checking the owner
	err = s.db.DeleteAssetFace(ctx, faceUUID)
	if err != nil {
		return fmt.Errorf("failed to delete face: %w", err)
	}

	// Update face counter metric
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

	// Face reassignment requires database queries to be implemented
	// This should:
	// 1. Verify user owns the face
	// 2. Verify user owns the target person
	// 3. Update face record to point to new person
	// Return empty response (not mock data)
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

// Helper function to convert database face to response format
func (s *Service) convertToFaceResponse(face sqlc.AssetFace) *FaceResponse {
	resp := &FaceResponse{
		ID:      uuidToString(face.ID),
		AssetID: uuidToString(face.AssetId),
		BoundingBox: BoundingBox{
			X1: face.BoundingBoxX1,
			Y1: face.BoundingBoxY1,
			X2: face.BoundingBoxX2,
			Y2: face.BoundingBoxY2,
		},
	}

	// Add person ID if present
	if face.PersonId.Valid {
		resp.PersonID = uuidToString(face.PersonId)
	}

	// Add image dimensions
	if face.ImageWidth > 0 {
		width := fmt.Sprintf("%d", face.ImageWidth)
		resp.ImageWidth = &width
	}
	if face.ImageHeight > 0 {
		height := fmt.Sprintf("%d", face.ImageHeight)
		resp.ImageHeight = &height
	}

	return resp
}

func uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}
