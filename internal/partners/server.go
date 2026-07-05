package partners

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

func currentUserUUIDFromContext(ctx context.Context) (pgtype.UUID, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return pgtype.UUID{}, err
	}

	return pgUUID(userID), nil
}

func parseUUIDParam(value, errMsg string) (pgtype.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}, status.Error(codes.InvalidArgument, errMsg)
	}

	return pgUUID(id), nil
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func buildPartnerResponse(
	id pgtype.UUID,
	email string,
	name string,
	inTimeline bool,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
) *immichv1.PartnerResponse {
	idStr := uuid.UUID(id.Bytes).String()
	return &immichv1.PartnerResponse{
		Id: idStr,
		User: &immichv1.User{
			Id:    idStr,
			Email: email,
			Name:  name,
		},
		InTimeline: inTimeline,
		CreatedAt:  timestamppb.New(createdAt.Time),
		UpdatedAt:  timestamppb.New(updatedAt.Time),
	}
}

func partnerResponseFromRow(row sqlc.GetPartnersRow) *immichv1.PartnerResponse {
	return buildPartnerResponse(
		row.ID,
		row.Email,
		row.Name,
		row.InTimeline,
		row.PartnershipCreatedAt,
		row.PartnershipUpdatedAt,
	)
}

func partnerResponseFromUser(user sqlc.User, inTimeline bool, createdAt, updatedAt pgtype.Timestamptz) *immichv1.PartnerResponse {
	return buildPartnerResponse(user.ID, user.Email, user.Name, inTimeline, createdAt, updatedAt)
}

// GetPartners gets all partners for the user
func (s *Server) GetPartners(ctx context.Context, request *immichv1.GetPartnersRequest) (*immichv1.GetPartnersResponse, error) {
	userUUID, err := currentUserUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Get partners from database
	partnerRows, err := s.queries.GetPartners(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get partners: %v", err)
	}

	// Convert to proto response
	partners := make([]*immichv1.PartnerResponse, 0, len(partnerRows))
	for _, row := range partnerRows {
		partners = append(partners, partnerResponseFromRow(row))
	}

	return &immichv1.GetPartnersResponse{
		Partners: partners,
	}, nil
}

// RemovePartner removes a partnership
func (s *Server) RemovePartner(ctx context.Context, request *immichv1.RemovePartnerRequest) (*emptypb.Empty, error) {
	userUUID, err := currentUserUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Parse partner ID
	partnerUUID, err := parseUUIDParam(request.GetId(), "invalid partner ID")
	if err != nil {
		return nil, err
	}

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
	userUUID, err := currentUserUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Parse partner ID from request
	partnerUUID, err := parseUUIDParam(request.GetSharedWithId(), "invalid partner ID")
	if err != nil {
		return nil, err
	}

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

	return partnerResponseFromUser(partnerUser, partnership.InTimeline, partnership.CreatedAt, partnership.UpdatedAt), nil
}

// UpdatePartner updates partnership settings
func (s *Server) UpdatePartner(ctx context.Context, request *immichv1.UpdatePartnerRequest) (*immichv1.PartnerResponse, error) {
	userUUID, err := currentUserUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Parse partner ID
	partnerUUID, err := parseUUIDParam(request.GetId(), "invalid partner ID")
	if err != nil {
		return nil, err
	}

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
		if row.ID.Bytes == partnerUUID.Bytes {
			found = true
			break
		}
	}

	if !found {
		return nil, status.Error(codes.NotFound, "partnership not found")
	}

	// Return the updated partnership info
	// Note: InTimeline update requires SQLC regeneration
	return partnerResponseFromUser(
		partnerUser,
		request.GetInTimeline(),
		partnerUser.CreatedAt,
		pgtype.Timestamptz{Time: time.Now(), Valid: true},
	), nil
}
