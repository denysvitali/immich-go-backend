package mapservice

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

	minLat := -90.0
	if request.MinLatitude != nil {
		minLat = request.GetMinLatitude()
	}
	maxLat := 90.0
	if request.MaxLatitude != nil {
		maxLat = request.GetMaxLatitude()
	}
	minLon := -180.0
	if request.MinLongitude != nil {
		minLon = request.GetMinLongitude()
	}
	maxLon := 180.0
	if request.MaxLongitude != nil {
		maxLon = request.GetMaxLongitude()
	}
	var limit int32 = 1000
	if request.Limit != nil && request.GetLimit() > 0 {
		limit = request.GetLimit()
	}
	var offset int32 = 0
	if request.Offset != nil && request.GetOffset() > 0 {
		offset = request.GetOffset()
	}

	createdAfter, err := optionalMapTimestamp(request.FileCreatedAfter)
	if err != nil {
		return nil, err
	}
	createdBefore, err := optionalMapTimestamp(request.FileCreatedBefore)
	if err != nil {
		return nil, err
	}

	// Get assets within the bounding box
	assets, err := s.queries.GetAssetsByLocation(ctx, sqlc.GetAssetsByLocationParams{
		OwnerID:       userUUID,
		MinLat:        pgtype.Float8{Float64: minLat, Valid: true},
		MaxLat:        pgtype.Float8{Float64: maxLat, Valid: true},
		MinLon:        pgtype.Float8{Float64: minLon, Valid: true},
		MaxLon:        pgtype.Float8{Float64: maxLon, Valid: true},
		IsFavorite:    optionalMapBool(request.IsFavorite),
		IsArchived:    optionalMapBool(request.IsArchived),
		CreatedAfter:  createdAfter,
		CreatedBefore: createdBefore,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get assets with location: %v", err)
	}

	// Convert assets to map markers
	markers := make([]*immichv1.MapMarker, 0, len(assets))
	for _, asset := range assets {
		if !asset.ExifLatitude.Valid || !asset.ExifLongitude.Valid {
			continue // Skip assets without location data
		}

		marker := &immichv1.MapMarker{
			Id:        uuid.UUID(asset.ID.Bytes).String(),
			Lat:       asset.ExifLatitude.Float64,
			Lon:       asset.ExifLongitude.Float64,
			Latitude:  asset.ExifLatitude.Float64,
			Longitude: asset.ExifLongitude.Float64,
			Timestamp: asset.LocalDateTime.Time.Format("2006-01-02T15:04:05Z"),
		}

		// Add location info if available
		if asset.City.Valid {
			marker.City = asset.City.String
		}
		if asset.State.Valid {
			marker.State = asset.State.String
		}
		if asset.Country.Valid {
			marker.Country = asset.Country.String
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
		OwnerID: userUUID,
		MinLat:  pgtype.Float8{Float64: latitude - delta, Valid: true},
		MaxLat:  pgtype.Float8{Float64: latitude + delta, Valid: true},
		MinLon:  pgtype.Float8{Float64: longitude - delta, Valid: true},
		MaxLon:  pgtype.Float8{Float64: longitude + delta, Valid: true},
		Limit:   10,
		Offset:  0,
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
		if asset.City.Valid && asset.City.String != "" {
			cityCount[asset.City.String]++
		}
		if asset.State.Valid && asset.State.String != "" {
			stateCount[asset.State.String]++
		}
		if asset.Country.Valid && asset.Country.String != "" {
			countryCount[asset.Country.String]++
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

func optionalMapBool(value *bool) pgtype.Bool {
	if value == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *value, Valid: true}
}

func optionalMapTimestamp(value *string) (pgtype.Timestamptz, error) {
	if value == nil || *value == "" {
		return pgtype.Timestamptz{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		return pgtype.Timestamptz{}, status.Errorf(codes.InvalidArgument, "invalid timestamp %q: %v", *value, err)
	}
	return pgtype.Timestamptz{Time: parsed, Valid: true}, nil
}
