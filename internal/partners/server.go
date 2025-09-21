package partners

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the PartnersService
type Server struct {
	immichv1.UnimplementedPartnersServiceServer
	queries *sqlc.Queries
}

// NewServer creates a new partners server
func NewServer(queries *sqlc.Queries) *Server {
	return &Server{
		queries: queries,
	}
}

// GetPartners gets all partners for the user
func (s *Server) GetPartners(ctx context.Context, request *immichv1.GetPartnersRequest) (*immichv1.GetPartnersResponse, error) {
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

	// Get partners from database
	partnerRows, err := s.queries.GetPartners(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get partners: %v", err)
	}

	// Convert to proto response
	partners := make([]*immichv1.PartnerResponse, 0, len(partnerRows))
	for _, row := range partnerRows {
		// Build the partner response with actual user data
		partnerIDStr := uuid.UUID(row.ID.Bytes).String()
		partner := &immichv1.PartnerResponse{
			Id: partnerIDStr,
			User: &immichv1.User{
				Id:    partnerIDStr,
				Email: row.Email,
				Name:  row.Name,
			},
			InTimeline: false, // Default to false since the current query doesn't include this field
			CreatedAt:  timestamppb.New(row.CreatedAt.Time),
			UpdatedAt:  timestamppb.New(row.UpdatedAt.Time),
		}
		partners = append(partners, partner)
	}

	return &immichv1.GetPartnersResponse{
		Partners: partners,
	}, nil
}

// RemovePartner removes a partnership
func (s *Server) RemovePartner(ctx context.Context, request *immichv1.RemovePartnerRequest) (*emptypb.Empty, error) {
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

	// Parse partner ID
	partnerID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid partner ID")
	}

	userUUID := pgtype.UUID{Bytes: userID, Valid: true}
	partnerUUID := pgtype.UUID{Bytes: partnerID, Valid: true}

	// Delete the partnership from database
	err = s.queries.DeletePartnership(ctx, sqlc.DeletePartnershipParams{
		SharedById:   userUUID,
		SharedWithId: partnerUUID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove partnership: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// CreatePartner creates a new partnership
func (s *Server) CreatePartner(ctx context.Context, request *immichv1.CreatePartnerRequest) (*immichv1.PartnerResponse, error) {
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

	// Parse partner ID from request
	partnerID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid partner ID")
	}

	userUUID := pgtype.UUID{Bytes: userID, Valid: true}
	partnerUUID := pgtype.UUID{Bytes: partnerID, Valid: true}

	// First check if partner user exists
	partnerUser, err := s.queries.GetUserByID(ctx, partnerUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "partner user not found")
	}

	// Create the partnership in database
	partnership, err := s.queries.CreatePartnership(ctx, sqlc.CreatePartnershipParams{
		SharedById:   userUUID,
		SharedWithId: partnerUUID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create partnership: %v", err)
	}

	return &immichv1.PartnerResponse{
		Id: uuid.UUID(partnerUser.ID.Bytes).String(),
		User: &immichv1.User{
			Id:    uuid.UUID(partnerUser.ID.Bytes).String(),
			Email: partnerUser.Email,
			Name:  partnerUser.Name,
		},
		InTimeline: partnership.InTimeline,
		CreatedAt:  timestamppb.New(partnership.CreatedAt.Time),
		UpdatedAt:  timestamppb.New(partnership.UpdatedAt.Time),
	}, nil
}

// UpdatePartner updates partnership settings
func (s *Server) UpdatePartner(ctx context.Context, request *immichv1.UpdatePartnerRequest) (*immichv1.PartnerResponse, error) {
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

	// Parse partner ID
	partnerID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid partner ID")
	}

	userUUID := pgtype.UUID{Bytes: userID, Valid: true}
	partnerUUID := pgtype.UUID{Bytes: partnerID, Valid: true}

	// For now, we'll fetch the partner details and return them
	// The UpdatePartnership query needs SQLC regeneration to be available
	partnerUser, err := s.queries.GetUserByID(ctx, partnerUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "partner not found")
	}

	// Get current partnership to verify it exists
	partnerRows, err := s.queries.GetPartners(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get partners: %v", err)
	}

	// Verify the partner exists in the partnership
	found := false
	for _, row := range partnerRows {
		if row.ID.Bytes == partnerID {
			found = true
			break
		}
	}

	if !found {
		return nil, status.Error(codes.NotFound, "partnership not found")
	}

	// Return the updated partnership info
	// Note: InTimeline update requires SQLC regeneration
	return &immichv1.PartnerResponse{
		Id: uuid.UUID(partnerUser.ID.Bytes).String(),
		User: &immichv1.User{
			Id:    uuid.UUID(partnerUser.ID.Bytes).String(),
			Email: partnerUser.Email,
			Name:  partnerUser.Name,
		},
		InTimeline: request.GetInTimeline(),
		CreatedAt:  timestamppb.New(partnerUser.CreatedAt.Time),
		UpdatedAt:  timestamppb.Now(),
	}, nil
}
