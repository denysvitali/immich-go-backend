package people

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual people retrieval from database
	// For now, return empty people list
	return &immichv1.GetAllPeopleResponse{
		People:      []*immichv1.PersonResponse{},
		Total:       0,
		HasNextPage: false,
	}, nil
}

// CreatePerson creates a new person
func (s *Server) CreatePerson(ctx context.Context, request *immichv1.CreatePersonRequest) (*immichv1.PersonResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual person creation
	return nil, status.Error(codes.Unimplemented, "method not implemented")
}

// UpdatePeople updates multiple people
func (s *Server) UpdatePeople(ctx context.Context, request *immichv1.UpdatePeopleRequest) (*immichv1.UpdatePeopleResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual people update
	return &immichv1.UpdatePeopleResponse{
		People: []*immichv1.PersonResponse{},
	}, nil
}

// GetPerson gets a person by ID
func (s *Server) GetPerson(ctx context.Context, request *immichv1.GetPersonRequest) (*immichv1.PersonResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual person retrieval by ID
	return nil, status.Error(codes.NotFound, "person not found")
}

// UpdatePerson updates a person
func (s *Server) UpdatePerson(ctx context.Context, request *immichv1.UpdatePersonRequest) (*immichv1.PersonResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual person update
	return nil, status.Error(codes.Unimplemented, "method not implemented")
}

// MergePerson merges multiple people into one
func (s *Server) MergePerson(ctx context.Context, request *immichv1.MergePersonRequest) (*immichv1.MergePersonResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual person merge
	return nil, status.Error(codes.Unimplemented, "method not implemented")
}

// ReassignFaces reassigns faces to different people
func (s *Server) ReassignFaces(ctx context.Context, request *immichv1.ReassignFacesRequest) (*immichv1.ReassignFacesResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual face reassignment
	return &immichv1.ReassignFacesResponse{
		People: []*immichv1.PersonResponse{},
	}, nil
}

// GetPersonStatistics gets statistics for a person
func (s *Server) GetPersonStatistics(ctx context.Context, request *immichv1.GetPersonStatisticsRequest) (*immichv1.PersonStatisticsResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual person statistics
	return &immichv1.PersonStatisticsResponse{
		Assets: 0,
	}, nil
}

// GetPersonThumbnail gets thumbnail for a person
func (s *Server) GetPersonThumbnail(ctx context.Context, request *immichv1.GetPersonThumbnailRequest) (*immichv1.GetPersonThumbnailResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual thumbnail retrieval
	return nil, status.Error(codes.NotFound, "thumbnail not found")
}
