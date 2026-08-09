package people

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the PeopleService
type Server struct {
	immichv1.UnimplementedPeopleServiceServer
	queries peopleQueries
	storage *storage.Service
}

type peopleQueries interface {
	CountPersonAssets(context.Context, pgtype.UUID) (int64, error)
	CreatePerson(context.Context, sqlc.CreatePersonParams) (sqlc.Person, error)
	DeletePerson(context.Context, pgtype.UUID) error
	GetPeople(context.Context, pgtype.UUID) ([]sqlc.Person, error)
	GetPerson(context.Context, pgtype.UUID) (sqlc.Person, error)
	MergePeople(context.Context, sqlc.MergePeopleParams) (sqlc.Person, error)
	ReassignFaces(context.Context, sqlc.ReassignFacesParams) (sqlc.Person, error)
	UpdatePerson(context.Context, sqlc.UpdatePersonParams) (sqlc.Person, error)
}

// NewServer creates a new people server. The storage service is required
// for GetPersonThumbnail to read the actual thumbnail bytes from the
// configured backend (local, S3, or rclone).
func NewServer(queries *sqlc.Queries, storageSvc *storage.Service) *Server {
	return &Server{
		queries: queries,
		storage: storageSvc,
	}
}

// thumbnailContentType maps a thumbnail path's extension to the right
// MIME type. Defaults to image/jpeg when the extension is unknown since
// person thumbnails are produced by the face-detection pipeline as JPEGs.
func thumbnailContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func currentUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	return auth.GetUserIDFromContext(ctx)
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func isPGUUID(id pgtype.UUID, want uuid.UUID) bool {
	return id.Valid && id.Bytes == want
}

func buildPersonResponse(person sqlc.Person, faceCount int64) *immichv1.PersonResponse {
	var birthDate *string
	if person.BirthDate.Valid {
		bd := person.BirthDate.Time.Format("2006-01-02")
		birthDate = &bd
	}

	return &immichv1.PersonResponse{
		Id:            uuid.UUID(person.ID.Bytes).String(),
		Name:          person.Name,
		BirthDate:     birthDate,
		ThumbnailPath: person.ThumbnailPath,
		Faces:         int32(faceCount),
		UpdatedAt:     timestamppb.New(person.UpdatedAt.Time),
		IsHidden:      person.IsHidden,
	}
}

func (s *Server) personResponse(ctx context.Context, person sqlc.Person) *immichv1.PersonResponse {
	faceCount, err := s.queries.CountPersonAssets(ctx, person.ID)
	if err != nil {
		faceCount = 0
	}
	return buildPersonResponse(person, faceCount)
}

func (s *Server) getOwnedPerson(
	ctx context.Context,
	userID uuid.UUID,
	id string,
	invalidIDMessage string,
	notFoundMessage string,
) (sqlc.Person, pgtype.UUID, error) {
	personID, err := uuid.Parse(id)
	if err != nil {
		return sqlc.Person{}, pgtype.UUID{}, status.Error(codes.InvalidArgument, invalidIDMessage)
	}

	personUUID := pgUUID(personID)
	person, err := s.queries.GetPerson(ctx, personUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Person{}, pgtype.UUID{}, status.Error(codes.NotFound, notFoundMessage)
		}
		return sqlc.Person{}, pgtype.UUID{}, status.Error(codes.Internal, "failed to get person")
	}
	if !isPGUUID(person.OwnerId, userID) {
		return sqlc.Person{}, pgtype.UUID{}, status.Error(codes.PermissionDenied, "access denied")
	}
	return person, personUUID, nil
}

// GetAllPeople gets all people for the user
func (s *Server) GetAllPeople(ctx context.Context, request *immichv1.GetAllPeopleRequest) (*immichv1.GetAllPeopleResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Get people from database
	dbPeople, err := s.queries.GetPeople(ctx, pgUUID(userID))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get people: %v", err)
	}

	// Convert to proto response
	people := make([]*immichv1.PersonResponse, 0, len(dbPeople))
	for _, person := range dbPeople {
		people = append(people, s.personResponse(ctx, person))
	}

	return &immichv1.GetAllPeopleResponse{
		People:      people,
		Total:       int32(len(people)),
		HasNextPage: false, // Pagination can be added later
	}, nil
}

// CreatePerson creates a new person
func (s *Server) CreatePerson(ctx context.Context, request *immichv1.CreatePersonRequest) (*immichv1.PersonResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Prepare birth date
	var birthDatePG pgtype.Date
	if request.BirthDate != nil && *request.BirthDate != "" {
		// Parse the birth date string (assuming format YYYY-MM-DD)
		parsedTime, err := time.Parse("2006-01-02", *request.BirthDate)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid birth date format")
		}
		birthDatePG = pgtype.Date{Time: parsedTime, Valid: true}
	}

	// Create person in database
	person, err := s.queries.CreatePerson(ctx, sqlc.CreatePersonParams{
		OwnerId:       pgUUID(userID),
		Name:          request.GetName(),
		BirthDate:     birthDatePG,
		ThumbnailPath: "",            // Initially empty
		FaceAssetId:   pgtype.UUID{}, // Initially null
		IsHidden:      false,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create person: %v", err)
	}

	return buildPersonResponse(person, 0), nil
}

// UpdatePeople updates multiple people
func (s *Server) UpdatePeople(ctx context.Context, request *immichv1.UpdatePeopleRequest) (*immichv1.UpdatePeopleResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	updatedPeople := make([]*immichv1.PersonResponse, 0)

	// Update each person
	for _, update := range request.GetPeople() {
		// Parse person ID
		personID, err := uuid.Parse(update.GetId())
		if err != nil {
			continue // Skip invalid IDs
		}

		personUUID := pgUUID(personID)

		// Get existing person to verify ownership
		existingPerson, err := s.queries.GetPerson(ctx, personUUID)
		if err != nil {
			continue // Skip if not found
		}

		// Verify ownership
		if !isPGUUID(existingPerson.OwnerId, userID) {
			continue // Skip if not owned by user
		}

		// Prepare update parameters
		updateParams := sqlc.UpdatePersonParams{
			ID: personUUID,
		}

		// Set fields if provided
		if update.Name != nil {
			updateParams.Name = pgtype.Text{String: *update.Name, Valid: true}
		}

		if update.BirthDate != nil {
			var birthDatePG pgtype.Date
			if *update.BirthDate != "" {
				parsedTime, err := time.Parse("2006-01-02", *update.BirthDate)
				if err == nil {
					birthDatePG = pgtype.Date{Time: parsedTime, Valid: true}
					updateParams.BirthDate = birthDatePG
				}
			}
		}

		if update.IsHidden != nil {
			updateParams.IsHidden = pgtype.Bool{Bool: *update.IsHidden, Valid: true}
		}

		// Update person in database
		updatedPerson, err := s.queries.UpdatePerson(ctx, updateParams)
		if err != nil {
			continue // Skip on error
		}

		updatedPeople = append(updatedPeople, s.personResponse(ctx, updatedPerson))
	}

	return &immichv1.UpdatePeopleResponse{
		People: updatedPeople,
	}, nil
}

// GetPerson gets a person by ID
func (s *Server) GetPerson(ctx context.Context, request *immichv1.GetPersonRequest) (*immichv1.PersonResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	person, _, err := s.getOwnedPerson(ctx, userID, request.GetId(), "invalid person ID", "person not found")
	if err != nil {
		return nil, err
	}

	return s.personResponse(ctx, person), nil
}

// DeletePeople deletes multiple people owned by the current user.
func (s *Server) DeletePeople(ctx context.Context, request *immichv1.DeletePeopleRequest) (*emptypb.Empty, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	for _, id := range request.GetIds() {
		if err := s.deletePersonByID(ctx, userID, id); err != nil {
			return nil, err
		}
	}

	return &emptypb.Empty{}, nil
}

// DeletePerson deletes one person owned by the current user.
func (s *Server) DeletePerson(ctx context.Context, request *immichv1.DeletePersonRequest) (*emptypb.Empty, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.deletePersonByID(ctx, userID, request.GetId()); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) deletePersonByID(ctx context.Context, userID uuid.UUID, id string) error {
	_, personUUID, err := s.getOwnedPerson(ctx, userID, id, "invalid person ID", "person not found")
	if err != nil {
		return err
	}

	if err := s.queries.DeletePerson(ctx, personUUID); err != nil {
		return status.Error(codes.Internal, "failed to delete person")
	}
	return nil
}

// UpdatePerson updates a person
func (s *Server) UpdatePerson(ctx context.Context, request *immichv1.UpdatePersonRequest) (*immichv1.PersonResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	_, personUUID, err := s.getOwnedPerson(ctx, userID, request.GetId(), "invalid person ID", "person not found")
	if err != nil {
		return nil, err
	}

	// Prepare update parameters
	updateParams := sqlc.UpdatePersonParams{
		ID: personUUID,
	}

	// Set name if provided
	if request.Name != nil {
		updateParams.Name = pgtype.Text{String: *request.Name, Valid: true}
	}

	// Set birth date if provided
	if request.BirthDate != nil {
		var birthDatePG pgtype.Date
		if *request.BirthDate != "" {
			parsedTime, err := time.Parse("2006-01-02", *request.BirthDate)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "invalid birth date format")
			}
			birthDatePG = pgtype.Date{Time: parsedTime, Valid: true}
		}
		updateParams.BirthDate = birthDatePG
	}

	// Set is hidden if provided
	if request.IsHidden != nil {
		updateParams.IsHidden = pgtype.Bool{Bool: *request.IsHidden, Valid: true}
	}

	// Set feature face asset if provided
	if request.FeatureFaceAssetId != nil {
		faceAssetID, err := uuid.Parse(*request.FeatureFaceAssetId)
		if err == nil {
			updateParams.FaceAssetID = pgUUID(faceAssetID)
		}
	}

	// Update person in database
	updatedPerson, err := s.queries.UpdatePerson(ctx, updateParams)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update person: %v", err)
	}

	return s.personResponse(ctx, updatedPerson), nil
}

// MergePerson merges multiple people into one
func (s *Server) MergePerson(ctx context.Context, request *immichv1.MergePersonRequest) (*immichv1.MergePersonResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	targetPerson, targetUUID, err := s.getOwnedPerson(ctx, userID, request.GetId(), "invalid target person ID", "target person not found")
	if err != nil {
		return nil, err
	}

	// Validate the complete request before changing any rows. This prevents a
	// cross-user, missing, or malformed source later in the list from leaving a
	// partially merged result.
	sourceUUIDs := make([]pgtype.UUID, 0, len(request.GetIds()))
	seen := make(map[uuid.UUID]struct{}, len(request.GetIds()))
	for _, idStr := range request.GetIds() {
		_, personUUID, err := s.getOwnedPerson(ctx, userID, idStr, "invalid person ID", "person not found")
		if err != nil {
			return nil, err
		}

		sourceID := uuid.UUID(personUUID.Bytes)
		if sourceID == uuid.UUID(targetUUID.Bytes) {
			return nil, status.Error(codes.InvalidArgument, "target person cannot be merged into itself")
		}
		if _, duplicate := seen[sourceID]; duplicate {
			continue
		}
		seen[sourceID] = struct{}{}
		sourceUUIDs = append(sourceUUIDs, personUUID)
	}

	if len(sourceUUIDs) > 0 {
		targetPerson, err = s.queries.MergePeople(ctx, sqlc.MergePeopleParams{
			TargetID:  targetUUID,
			OwnerID:   pgUUID(userID),
			SourceIds: sourceUUIDs,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, status.Error(codes.Aborted, "people changed during merge")
			}
			return nil, status.Error(codes.Internal, "failed to merge people")
		}
	}

	return &immichv1.MergePersonResponse{
		Person: s.personResponse(ctx, targetPerson),
	}, nil
}

// ReassignFaces reassigns faces to different people
func (s *Server) ReassignFaces(ctx context.Context, request *immichv1.ReassignFacesRequest) (*immichv1.ReassignFacesResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	target, targetUUID, err := s.getOwnedPerson(ctx, userID, request.GetId(), "invalid target person ID", "target person not found")
	if err != nil {
		return nil, err
	}

	faceUpdates := make([]sqlc.FaceReassignment, 0, len(request.GetFaceUpdates()))
	validatedSources := make(map[uuid.UUID]struct{}, len(request.GetFaceUpdates()))
	for _, update := range request.GetFaceUpdates() {
		assetID, err := uuid.Parse(update.GetAssetId())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid asset ID")
		}
		sourcePersonID, err := uuid.Parse(update.GetPersonId())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid source person ID")
		}
		if _, alreadyValidated := validatedSources[sourcePersonID]; !alreadyValidated {
			if _, _, err := s.getOwnedPerson(ctx, userID, sourcePersonID.String(), "invalid source person ID", "source person not found"); err != nil {
				return nil, err
			}
			validatedSources[sourcePersonID] = struct{}{}
		}
		faceUpdates = append(faceUpdates, sqlc.FaceReassignment{
			AssetID:        pgUUID(assetID),
			SourcePersonID: pgUUID(sourcePersonID),
		})
	}

	if len(faceUpdates) == 0 {
		return &immichv1.ReassignFacesResponse{People: []*immichv1.PersonResponse{}}, nil
	}

	target, err = s.queries.ReassignFaces(ctx, sqlc.ReassignFacesParams{
		TargetPersonID: targetUUID,
		OwnerID:        pgUUID(userID),
		FaceUpdates:    faceUpdates,
	})
	if err != nil {
		switch {
		case errors.Is(err, sqlc.ErrFaceReassignmentAccessDenied):
			return nil, status.Error(codes.PermissionDenied, "access denied")
		case errors.Is(err, sqlc.ErrFaceReassignmentNotFound):
			return nil, status.Error(codes.NotFound, "face not found")
		case errors.Is(err, sqlc.ErrFaceReassignmentConflict), errors.Is(err, pgx.ErrNoRows):
			return nil, status.Error(codes.Aborted, "people or faces changed during reassignment")
		default:
			return nil, status.Error(codes.Internal, "failed to reassign faces")
		}
	}

	faceCount, err := s.queries.CountPersonAssets(ctx, target.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to count reassigned faces")
	}
	targetResponse := buildPersonResponse(target, faceCount)
	people := make([]*immichv1.PersonResponse, 0, len(request.GetFaceUpdates()))
	for range request.GetFaceUpdates() {
		people = append(people, targetResponse)
	}
	return &immichv1.ReassignFacesResponse{
		People: people,
	}, nil
}

// GetPersonStatistics gets statistics for a person
func (s *Server) GetPersonStatistics(ctx context.Context, request *immichv1.GetPersonStatisticsRequest) (*immichv1.PersonStatisticsResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	person, _, err := s.getOwnedPerson(ctx, userID, request.GetId(), "invalid person ID", "person not found")
	if err != nil {
		return nil, err
	}

	// Get asset count for this person
	assetCount, err := s.queries.CountPersonAssets(ctx, person.ID)
	if err != nil {
		assetCount = 0
	}

	return &immichv1.PersonStatisticsResponse{
		Assets: int32(assetCount),
	}, nil
}

// GetPersonThumbnail gets thumbnail for a person
func (s *Server) GetPersonThumbnail(ctx context.Context, request *immichv1.GetPersonThumbnailRequest) (*immichv1.GetPersonThumbnailResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	person, _, err := s.getOwnedPerson(ctx, userID, request.GetId(), "invalid person ID", "person not found")
	if err != nil {
		return nil, err
	}

	// Check if thumbnail path exists
	if person.ThumbnailPath == "" {
		return nil, status.Error(codes.NotFound, "thumbnail not found")
	}

	// Storage backend is required to read the actual thumbnail bytes.
	if s.storage == nil {
		return nil, status.Error(codes.FailedPrecondition, "storage backend not configured")
	}

	reader, err := s.storage.Download(ctx, person.ThumbnailPath)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "thumbnail not found in storage: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read thumbnail: %v", err)
	}

	return &immichv1.GetPersonThumbnailResponse{
		ThumbnailData: data,
		ContentType:   thumbnailContentType(person.ThumbnailPath),
	}, nil
}
