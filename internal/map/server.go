package mapservice

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the MapService
type Server struct {
	immichv1.UnimplementedMapServiceServer
	queries *sqlc.Queries
}

// NewServer creates a new map server
func NewServer(queries *sqlc.Queries) *Server {
	return &Server{
		queries: queries,
	}
}

// GetMapMarkers gets map markers for assets with location data
func (s *Server) GetMapMarkers(ctx context.Context, request *immichv1.GetMapMarkersRequest) (*immichv1.GetMapMarkersResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual map marker retrieval based on request parameters
	// For now, return empty markers list
	return &immichv1.GetMapMarkersResponse{
		Markers: []*immichv1.MapMarker{},
	}, nil
}

// ReverseGeocode converts coordinates to location information
func (s *Server) ReverseGeocode(ctx context.Context, request *immichv1.ReverseGeocodeRequest) (*immichv1.ReverseGeocodeResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual reverse geocoding service integration
	// For now, return empty location information
	return &immichv1.ReverseGeocodeResponse{
		City:    "",
		State:   "",
		Country: "",
	}, nil
}
