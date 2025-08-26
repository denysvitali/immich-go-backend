package partners

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
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
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual partner retrieval based on direction
	// For now, return empty partners list
	return &immichv1.GetPartnersResponse{
		Partners: []*immichv1.PartnerResponse{},
	}, nil
}

// RemovePartner removes a partnership
func (s *Server) RemovePartner(ctx context.Context, request *immichv1.RemovePartnerRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual partner removal
	// Validate partner ID and ownership before removal
	return &emptypb.Empty{}, nil
}

// CreatePartner creates a new partnership
func (s *Server) CreatePartner(ctx context.Context, request *immichv1.CreatePartnerRequest) (*immichv1.PartnerResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual partner creation
	// Validate partner user exists and isn't already a partner
	return nil, status.Error(codes.Unimplemented, "method not implemented")
}

// UpdatePartner updates partnership settings
func (s *Server) UpdatePartner(ctx context.Context, request *immichv1.UpdatePartnerRequest) (*immichv1.PartnerResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual partner update
	// Update timeline visibility and other partner settings
	return nil, status.Error(codes.Unimplemented, "method not implemented")
}