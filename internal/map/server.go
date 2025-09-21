package mapservice

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
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

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get bounding box from request or use defaults
	minLat := request.GetMinLatitude()
	maxLat := request.GetMaxLatitude()
	minLon := request.GetMinLongitude()
	maxLon := request.GetMaxLongitude()
	limit := request.GetLimit()
	offset := request.GetOffset()

	// Set defaults if not provided
	if minLat == 0 && maxLat == 0 {
		minLat = -90.0
		maxLat = 90.0
	}
	if minLon == 0 && maxLon == 0 {
		minLon = -180.0
		maxLon = 180.0
	}
	if limit == 0 {
		limit = 1000 // Default limit
	}

	// Get assets within the bounding box
	assets, err := s.queries.GetAssetsByLocation(ctx, sqlc.GetAssetsByLocationParams{
		OwnerId:  userUUID,
		Column2:  minLat,
		Column3:  maxLat,
		Column4:  minLon,
		Column5:  maxLon,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get assets with location: %v", err)
	}

	// Convert assets to map markers
	markers := make([]*immichv1.MapMarker, 0, len(assets))
	for _, asset := range assets {
		// Get exif data for location
		exif, err := s.queries.GetExifByAssetID(ctx, asset.ID)
		if err != nil || exif.Latitude == nil || exif.Longitude == nil {
			continue // Skip assets without location data
		}

		marker := &immichv1.MapMarker{
			Id:        uuid.UUID(asset.ID.Bytes).String(),
			Latitude:  *exif.Latitude,
			Longitude: *exif.Longitude,
			Timestamp: timestamppb.New(asset.LocalDateTime.Time),
		}

		// Add location info if available
		if exif.City != nil {
			marker.City = *exif.City
		}
		if exif.State != nil {
			marker.State = *exif.State
		}
		if exif.Country != nil {
			marker.Country = *exif.Country
		}

		markers = append(markers, marker)
	}

	return &immichv1.GetMapMarkersResponse{
		Markers: markers,
	}, nil
}

// ReverseGeocode converts coordinates to location information
func (s *Server) ReverseGeocode(ctx context.Context, request *immichv1.ReverseGeocodeRequest) (*immichv1.ReverseGeocodeResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get coordinates from request
	latitude := request.GetLatitude()
	longitude := request.GetLongitude()

	// Find nearest assets with location data to approximate the location
	// This is a simplified approach - in production, you'd use a real geocoding service
	// For now, find assets near this location and use their location info
	delta := 0.1 // Approximately 11km at the equator
	assets, err := s.queries.GetAssetsByLocation(ctx, sqlc.GetAssetsByLocationParams{
		OwnerId:  userUUID,
		Column2:  latitude - delta,
		Column3:  latitude + delta,
		Column4:  longitude - delta,
		Column5:  longitude + delta,
		Limit:    10,
		Offset:   0,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find nearby assets: %v", err)
	}

	// Find the most common location info from nearby assets
	var city, state, country string
	cityCount := make(map[string]int)
	stateCount := make(map[string]int)
	countryCount := make(map[string]int)

	for _, asset := range assets {
		exif, err := s.queries.GetExifByAssetID(ctx, asset.ID)
		if err != nil {
			continue
		}

		if exif.City != nil && *exif.City != "" {
			cityCount[*exif.City]++
		}
		if exif.State != nil && *exif.State != "" {
			stateCount[*exif.State]++
		}
		if exif.Country != nil && *exif.Country != "" {
			countryCount[*exif.Country]++
		}
	}

	// Find most common values
	maxCityCount := 0
	for c, count := range cityCount {
		if count > maxCityCount {
			city = c
			maxCityCount = count
		}
	}

	maxStateCount := 0
	for s, count := range stateCount {
		if count > maxStateCount {
			state = s
			maxStateCount = count
		}
	}

	maxCountryCount := 0
	for c, count := range countryCount {
		if count > maxCountryCount {
			country = c
			maxCountryCount = count
		}
	}

	return &immichv1.ReverseGeocodeResponse{
		City:    city,
		State:   state,
		Country: country,
	}, nil
}
