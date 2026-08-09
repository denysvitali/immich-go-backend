package faces

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var tracer = telemetry.GetTracer("faces")

var ErrAccessDenied = errors.New("access denied")

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
	metrics, err := telemetry.NewServiceMetrics(telemetry.ServiceMetricNames{
		Operations:            "face_operations_total",
		OperationsDescription: "Total number of face operations",
		Duration:              "face_operation_duration_seconds",
		DurationDescription:   "Time spent on face operations",
		Items:                 "faces_total",
		ItemsDescription:      "Total number of faces in the system",
	})
	if err != nil {
		return nil, err
	}

	return &Service{
		db:                queries,
		config:            cfg,
		faceCounter:       metrics.Items,
		operationCounter:  metrics.Operations,
		operationDuration: metrics.Duration,
	}, nil
}

// GetFaces retrieves faces, optionally filtered by ID
func (s *Service) GetFaces(ctx context.Context, userID uuid.UUID, req GetFacesRequest) (*GetFacesResponse, error) {
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

	switch {
	case req.ID != nil:
		faceID, parseErr := uuid.Parse(*req.ID)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid face ID: %w", parseErr)
		}
		face, getErr := s.db.GetAssetFaceByID(ctx, pgutil.UUIDToPgtype(faceID))
		if getErr != nil {
			return nil, fmt.Errorf("failed to get face: %w", getErr)
		}
		if err := s.requireOwnedAsset(ctx, userID, face.AssetId); err != nil {
			return nil, err
		}
		faces = []sqlc.AssetFace{face}
	case req.AssetID != "":
		// Get faces by asset
		var assetID uuid.UUID
		assetID, err = uuid.Parse(req.AssetID)
		if err != nil {
			return nil, fmt.Errorf("invalid asset ID: %w", err)
		}
		assetUUID := pgutil.UUIDToPgtype(assetID)
		if err := s.requireOwnedAsset(ctx, userID, assetUUID); err != nil {
			return nil, err
		}
		faces, err = s.db.GetFacesByAsset(ctx, assetUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get faces for asset: %w", err)
		}
	case req.PersonID != "":
		// Get faces by person
		var personID uuid.UUID
		personID, err = uuid.Parse(req.PersonID)
		if err != nil {
			return nil, fmt.Errorf("invalid person ID: %w", err)
		}
		personUUID := pgutil.UUIDToPgtype(personID)
		if err := s.requireOwnedPerson(ctx, userID, personUUID); err != nil {
			return nil, err
		}
		faces, err = s.db.GetFacesByPerson(ctx, personUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get faces for person: %w", err)
		}
	default:
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
func (s *Service) CreateFace(ctx context.Context, userID uuid.UUID, req CreateFaceRequest) (*FaceResponse, error) {
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
	assetUUID := pgutil.UUIDToPgtype(assetID)

	personID, err := uuid.Parse(req.PersonID)
	if err != nil {
		return nil, fmt.Errorf("invalid person ID: %w", err)
	}
	personUUID := pgutil.UUIDToPgtype(personID)
	if err := s.requireOwnedAsset(ctx, userID, assetUUID); err != nil {
		return nil, err
	}
	if err := s.requireOwnedPerson(ctx, userID, personUUID); err != nil {
		return nil, err
	}

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
		ID:       pgutil.UUIDToString(face.ID),
		AssetID:  pgutil.UUIDToString(face.AssetId),
		PersonID: pgutil.UUIDToString(face.PersonId),
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
func (s *Service) DeleteFace(ctx context.Context, userID uuid.UUID, faceID string) error {
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
	faceUUID := pgutil.UUIDToPgtype(fID)
	face, err := s.db.GetAssetFaceByID(ctx, faceUUID)
	if err != nil {
		return fmt.Errorf("failed to get face: %w", err)
	}
	if err := s.requireOwnedAsset(ctx, userID, face.AssetId); err != nil {
		return err
	}

	// Delete the face record
	err = s.db.DeleteAssetFace(ctx, faceUUID)
	if err != nil {
		return fmt.Errorf("failed to delete face: %w", err)
	}

	// Update face counter metric
	s.faceCounter.Add(ctx, -1)

	return nil
}

// ReassignFacesById reassigns faces to a different person
func (s *Service) ReassignFacesById(ctx context.Context, userID uuid.UUID, faceID, personID string) (*ReassignFacesByIdResponse, error) {
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

	// Parse face ID
	fID, err := uuid.Parse(faceID)
	if err != nil {
		return nil, fmt.Errorf("invalid face ID: %w", err)
	}
	faceUUID := pgutil.UUIDToPgtype(fID)

	// Parse person ID
	pID, err := uuid.Parse(personID)
	if err != nil {
		return nil, fmt.Errorf("invalid person ID: %w", err)
	}
	personUUID := pgutil.UUIDToPgtype(pID)

	updatedFace, err := s.db.ReassignSingleFace(ctx, sqlc.ReassignSingleFaceParams{
		FaceID:         faceUUID,
		TargetPersonID: personUUID,
		OwnerID:        pgutil.UUIDToPgtype(userID),
	})
	if err != nil {
		if errors.Is(err, sqlc.ErrFaceReassignmentAccessDenied) {
			return nil, ErrAccessDenied
		}
		if errors.Is(err, sqlc.ErrFaceReassignmentNotFound) {
			return nil, fmt.Errorf("failed to get face: %w", err)
		}
		return nil, fmt.Errorf("failed to reassign face: %w", err)
	}

	// Convert to response format
	return &ReassignFacesByIdResponse{
		UpdatedFaces: []*FaceResponse{s.convertToFaceResponse(updatedFace)},
	}, nil
}

// Request/Response types

type GetFacesRequest struct {
	ID       *string
	AssetID  string
	PersonID string
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

func (s *Service) requireOwnedAsset(ctx context.Context, userID uuid.UUID, assetID pgtype.UUID) error {
	_, err := s.db.GetAssetByIDAndUser(ctx, sqlc.GetAssetByIDAndUserParams{
		ID:      assetID,
		OwnerId: pgutil.UUIDToPgtype(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAccessDenied
	}
	if err != nil {
		return fmt.Errorf("failed to check asset ownership: %w", err)
	}
	return nil
}

func (s *Service) requireOwnedPerson(ctx context.Context, userID uuid.UUID, personID pgtype.UUID) error {
	person, err := s.db.GetPerson(ctx, personID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAccessDenied
	}
	if err != nil {
		return fmt.Errorf("failed to check person ownership: %w", err)
	}
	if person.OwnerId != pgutil.UUIDToPgtype(userID) {
		return ErrAccessDenied
	}
	return nil
}

// Helper function to convert database face to response format
func (s *Service) convertToFaceResponse(face sqlc.AssetFace) *FaceResponse {
	resp := &FaceResponse{
		ID:      pgutil.UUIDToString(face.ID),
		AssetID: pgutil.UUIDToString(face.AssetId),
		BoundingBox: BoundingBox{
			X1: face.BoundingBoxX1,
			Y1: face.BoundingBoxY1,
			X2: face.BoundingBoxX2,
			Y2: face.BoundingBoxY2,
		},
	}

	// Add person ID if present
	if face.PersonId.Valid {
		resp.PersonID = pgutil.UUIDToString(face.PersonId)
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
