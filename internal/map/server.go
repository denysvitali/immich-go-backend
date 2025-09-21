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

	// Use default bounding box for now since request doesn't have these fields
	// TODO: Add these fields to the protobuf definition
	minLat := -90.0
	maxLat := 90.0
	minLon := -180.0
	maxLon := 180.0
	var limit int32 = 1000
	var offset int32 = 0

	// Get assets within the bounding box
	assets, err := s.queries.GetAssetsByLocation(ctx, sqlc.GetAssetsByLocationParams{
		OwnerId:     userUUID,
		Latitude:    pgtype.Float8{Float64: minLat, Valid: true},
		Latitude_2:  pgtype.Float8{Float64: maxLat, Valid: true},
		Longitude:   pgtype.Float8{Float64: minLon, Valid: true},
		Longitude_2: pgtype.Float8{Float64: maxLon, Valid: true},
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get assets with location: %v", err)
	}

	// Convert assets to map markers
	markers := make([]*immichv1.MapMarker, 0, len(assets))
	for _, asset := range assets {
		// Get exif data for location
		exif, err := s.queries.GetExifByAssetId(ctx, asset.ID)
		if err != nil || !exif.Latitude.Valid || !exif.Longitude.Valid {
			continue // Skip assets without location data
		}

		marker := &immichv1.MapMarker{
			Id:        uuid.UUID(asset.ID.Bytes).String(),
			Latitude:  exif.Latitude.Float64,
			Longitude: exif.Longitude.Float64,
			Timestamp: asset.LocalDateTime.Time.Format("2006-01-02T15:04:05Z"),
		}

		// Add location info if available
		if exif.City.Valid {
			marker.City = exif.City.String
		}
		if exif.State.Valid {
			marker.State = exif.State.String
		}
		if exif.Country.Valid {
			marker.Country = exif.Country.String
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
		OwnerId:     userUUID,
		Latitude:    pgtype.Float8{Float64: latitude - delta, Valid: true},
		Latitude_2:  pgtype.Float8{Float64: latitude + delta, Valid: true},
		Longitude:   pgtype.Float8{Float64: longitude - delta, Valid: true},
		Longitude_2: pgtype.Float8{Float64: longitude + delta, Valid: true},
		Limit:       10,
		Offset:      0,
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
		exif, err := s.queries.GetExifByAssetId(ctx, asset.ID)
		if err != nil {
			continue
		}

		if exif.City.Valid && exif.City.String != "" {
			cityCount[exif.City.String]++
		}
		if exif.State.Valid && exif.State.String != "" {
			stateCount[exif.State.String]++
		}
		if exif.Country.Valid && exif.Country.String != "" {
			countryCount[exif.Country.String]++
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
