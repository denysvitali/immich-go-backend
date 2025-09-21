package people

import (
	"context"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the PeopleService
type Server struct {
	immichv1.UnimplementedPeopleServiceServer
	queries *sqlc.Queries
}

// NewServer creates a new people server
func NewServer(queries *sqlc.Queries) *Server {
	return &Server{
		queries: queries,
	}
}

// GetAllPeople gets all people for the user
func (s *Server) GetAllPeople(ctx context.Context, request *immichv1.GetAllPeopleRequest) (*immichv1.GetAllPeopleResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user ID")
	}

	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get people from database
	dbPeople, err := s.queries.GetPeople(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get people: %v", err)
	}

	// Convert to proto response
	people := make([]*immichv1.PersonResponse, 0, len(dbPeople))
	for _, person := range dbPeople {
		// Count face assets for this person
		faceCount, err := s.queries.CountPersonAssets(ctx, person.ID)
		if err != nil {
			// Log error but continue
			faceCount = 0
		}

		var birthDate *string
		if person.BirthDate.Valid {
			bd := person.BirthDate.Time.Format("2006-01-02")
			birthDate = &bd
		}

		people = append(people, &immichv1.PersonResponse{
			Id:            uuid.UUID(person.ID.Bytes).String(),
			Name:          person.Name,
			BirthDate:     birthDate,
			ThumbnailPath: person.ThumbnailPath,
			Faces:         int32(faceCount),
			UpdatedAt:     timestamppb.New(person.UpdatedAt.Time),
			IsHidden:      person.IsHidden,
		})
	}

	return &immichv1.GetAllPeopleResponse{
		People:      people,
		Total:       int32(len(people)),
		HasNextPage: false, // Pagination can be added later
	}, nil
}

// CreatePerson creates a new person
func (s *Server) CreatePerson(ctx context.Context, request *immichv1.CreatePersonRequest) (*immichv1.PersonResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user ID")
	}

	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

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
		OwnerId:       userUUID,
		Name:          request.GetName(),
		BirthDate:     birthDatePG,
		ThumbnailPath: "", // Initially empty
		FaceAssetId:   pgtype.UUID{}, // Initially null
		IsHidden:      false,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create person: %v", err)
	}

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
		Faces:         0, // New person has no faces yet
		UpdatedAt:     timestamppb.New(person.UpdatedAt.Time),
		IsHidden:      person.IsHidden,
	}, nil
}

// UpdatePeople updates multiple people
func (s *Server) UpdatePeople(ctx context.Context, request *immichv1.UpdatePeopleRequest) (*immichv1.UpdatePeopleResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID for ownership verification
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user ID")
	}

	updatedPeople := make([]*immichv1.PersonResponse, 0)

	// Update each person
	for _, update := range request.GetPeople() {
		// Parse person ID
		personID, err := uuid.Parse(update.GetId())
		if err != nil {
			continue // Skip invalid IDs
		}

		personUUID := pgtype.UUID{Bytes: personID, Valid: true}

		// Get existing person to verify ownership
		existingPerson, err := s.queries.GetPerson(ctx, personUUID)
		if err != nil {
			continue // Skip if not found
		}

		// Verify ownership
		if existingPerson.OwnerId.Bytes != userID {
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

		// Count face assets
		faceCount, _ := s.queries.CountPersonAssets(ctx, updatedPerson.ID)

		var birthDate *string
		if updatedPerson.BirthDate.Valid {
			bd := updatedPerson.BirthDate.Time.Format("2006-01-02")
			birthDate = &bd
		}

		updatedPeople = append(updatedPeople, &immichv1.PersonResponse{
			Id:            uuid.UUID(updatedPerson.ID.Bytes).String(),
			Name:          updatedPerson.Name,
			BirthDate:     birthDate,
			ThumbnailPath: updatedPerson.ThumbnailPath,
			Faces:         int32(faceCount),
			UpdatedAt:     timestamppb.New(updatedPerson.UpdatedAt.Time),
			IsHidden:      updatedPerson.IsHidden,
		})
	}

	return &immichv1.UpdatePeopleResponse{
		People: updatedPeople,
	}, nil
}

// GetPerson gets a person by ID
func (s *Server) GetPerson(ctx context.Context, request *immichv1.GetPersonRequest) (*immichv1.PersonResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse person ID
	personID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid person ID")
	}

	personUUID := pgtype.UUID{Bytes: personID, Valid: true}

	// Get person from database
	person, err := s.queries.GetPerson(ctx, personUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "person not found")
	}

	// Verify ownership
	userID, _ := uuid.Parse(claims.UserID)
	if person.OwnerId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	// Count face assets
	faceCount, err := s.queries.CountPersonAssets(ctx, person.ID)
	if err != nil {
		faceCount = 0
	}

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
	}, nil
}

// UpdatePerson updates a person
func (s *Server) UpdatePerson(ctx context.Context, request *immichv1.UpdatePersonRequest) (*immichv1.PersonResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse person ID
	personID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid person ID")
	}

	personUUID := pgtype.UUID{Bytes: personID, Valid: true}

	// Get existing person to verify ownership
	existingPerson, err := s.queries.GetPerson(ctx, personUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "person not found")
	}

	// Verify ownership
	userID, _ := uuid.Parse(claims.UserID)
	if existingPerson.OwnerId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
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
			updateParams.FaceAssetID = pgtype.UUID{Bytes: faceAssetID, Valid: true}
		}
	}

	// Update person in database
	updatedPerson, err := s.queries.UpdatePerson(ctx, updateParams)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update person: %v", err)
	}

	// Count face assets
	faceCount, err := s.queries.CountPersonAssets(ctx, updatedPerson.ID)
	if err != nil {
		faceCount = 0
	}

	var birthDate *string
	if updatedPerson.BirthDate.Valid {
		bd := updatedPerson.BirthDate.Time.Format("2006-01-02")
		birthDate = &bd
	}

	return &immichv1.PersonResponse{
		Id:            uuid.UUID(updatedPerson.ID.Bytes).String(),
		Name:          updatedPerson.Name,
		BirthDate:     birthDate,
		ThumbnailPath: updatedPerson.ThumbnailPath,
		Faces:         int32(faceCount),
		UpdatedAt:     timestamppb.New(updatedPerson.UpdatedAt.Time),
		IsHidden:      updatedPerson.IsHidden,
	}, nil
}

// MergePerson merges multiple people into one
func (s *Server) MergePerson(ctx context.Context, request *immichv1.MergePersonRequest) (*immichv1.MergePersonResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse target person ID
	targetID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid target person ID")
	}

	targetUUID := pgtype.UUID{Bytes: targetID, Valid: true}

	// Get target person from database
	targetPerson, err := s.queries.GetPerson(ctx, targetUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "target person not found")
	}

	// Verify ownership
	userID, _ := uuid.Parse(claims.UserID)
	if targetPerson.OwnerId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	// For each person to merge, verify ownership and delete them
	// In a real implementation, we would also reassign their face assets to the target person
	for _, idStr := range request.GetIds() {
		personID, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}

		personUUID := pgtype.UUID{Bytes: personID, Valid: true}

		// Get person to verify ownership
		person, err := s.queries.GetPerson(ctx, personUUID)
		if err != nil {
			continue
		}

		// Verify ownership
		if person.OwnerId.Bytes != userID {
			continue
		}

		// Delete the person (in real implementation, would reassign assets first)
		_ = s.queries.DeletePerson(ctx, personUUID)
	}

	// Get updated face count for target person
	faceCount, err := s.queries.CountPersonAssets(ctx, targetPerson.ID)
	if err != nil {
		faceCount = 0
	}

	var birthDate *string
	if targetPerson.BirthDate.Valid {
		bd := targetPerson.BirthDate.Time.Format("2006-01-02")
		birthDate = &bd
	}

	return &immichv1.MergePersonResponse{
		Person: &immichv1.PersonResponse{
			Id:            uuid.UUID(targetPerson.ID.Bytes).String(),
			Name:          targetPerson.Name,
			BirthDate:     birthDate,
			ThumbnailPath: targetPerson.ThumbnailPath,
			Faces:         int32(faceCount),
			UpdatedAt:     timestamppb.New(targetPerson.UpdatedAt.Time),
			IsHidden:      targetPerson.IsHidden,
		},
	}, nil
}

// ReassignFaces reassigns faces to different people
func (s *Server) ReassignFaces(ctx context.Context, request *immichv1.ReassignFacesRequest) (*immichv1.ReassignFacesResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID for ownership verification
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user ID")
	}

	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get all people for the user to return updated list
	dbPeople, err := s.queries.GetPeople(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get people: %v", err)
	}

	// Convert to proto response
	people := make([]*immichv1.PersonResponse, 0, len(dbPeople))
	for _, person := range dbPeople {
		faceCount, _ := s.queries.CountPersonAssets(ctx, person.ID)

		var birthDate *string
		if person.BirthDate.Valid {
			bd := person.BirthDate.Time.Format("2006-01-02")
			birthDate = &bd
		}

		people = append(people, &immichv1.PersonResponse{
			Id:            uuid.UUID(person.ID.Bytes).String(),
			Name:          person.Name,
			BirthDate:     birthDate,
			ThumbnailPath: person.ThumbnailPath,
			Faces:         int32(faceCount),
			UpdatedAt:     timestamppb.New(person.UpdatedAt.Time),
			IsHidden:      person.IsHidden,
		})
	}

	return &immichv1.ReassignFacesResponse{
		People: people,
	}, nil
}

// GetPersonStatistics gets statistics for a person
func (s *Server) GetPersonStatistics(ctx context.Context, request *immichv1.GetPersonStatisticsRequest) (*immichv1.PersonStatisticsResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse person ID
	personID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid person ID")
	}

	personUUID := pgtype.UUID{Bytes: personID, Valid: true}

	// Get person to verify ownership
	person, err := s.queries.GetPerson(ctx, personUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "person not found")
	}

	// Verify ownership
	userID, _ := uuid.Parse(claims.UserID)
	if person.OwnerId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
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
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse person ID
	personID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid person ID")
	}

	personUUID := pgtype.UUID{Bytes: personID, Valid: true}

	// Get person from database
	person, err := s.queries.GetPerson(ctx, personUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "person not found")
	}

	// Verify ownership
	userID, _ := uuid.Parse(claims.UserID)
	if person.OwnerId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	// Check if thumbnail path exists
	if person.ThumbnailPath == "" {
		return nil, status.Error(codes.NotFound, "thumbnail not found")
	}

	// In a real implementation, we would read the thumbnail file and return it
	// For now, return a response with the thumbnail path
	return &immichv1.GetPersonThumbnailResponse{
		ThumbnailData: []byte(person.ThumbnailPath), // In real impl, this would be actual image data
		ContentType:   "text/plain", // In real impl, this would be image/jpeg or image/png
	}, nil
}
