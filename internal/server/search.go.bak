package server

import (
	"context"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/search"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Ensure Server implements SearchServiceServer
var _ immichv1.SearchServiceServer = (*Server)(nil)

// Search performs a general search
func (s *Server) Search(ctx context.Context, req *immichv1.SearchRequest) (*immichv1.SearchResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Perform search
	results, err := s.searchService.Search(ctx, userID, &search.SearchQuery{
		Query:   req.Query,
		Type:    req.Type,
		Page:    int(req.Page),
		PerPage: int(req.PerPage),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	// Convert results to proto format
	var assets []*immichv1.AssetResponse
	for _, asset := range results.Assets {
		assets = append(assets, &immichv1.AssetResponse{
			Id:         asset.ID.String(),
			DeviceId:   asset.DeviceID,
			Type:       immichv1.AssetType(asset.Type),
			OriginalPath: asset.OriginalPath,
			CreatedAt:  asset.CreatedAt.Unix(),
			UpdatedAt:  asset.UpdatedAt.Unix(),
		})
	}

	return &immichv1.SearchResponse{
		Assets: assets,
		Total:  int32(results.Total),
	}, nil
}

// SearchMetadata searches asset metadata
func (s *Server) SearchMetadata(ctx context.Context, req *immichv1.SearchMetadataRequest) (*immichv1.SearchMetadataResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Search metadata
	results, err := s.searchService.SearchMetadata(ctx, userID, &search.MetadataQuery{
		Make:        req.Make,
		Model:       req.Model,
		City:        req.City,
		State:       req.State,
		Country:     req.Country,
		DateFrom:    req.TakenAfter,
		DateTo:      req.TakenBefore,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "metadata search failed: %v", err)
	}

	// Convert results
	var assets []*immichv1.AssetResponse
	for _, asset := range results {
		assets = append(assets, &immichv1.AssetResponse{
			Id:           asset.ID.String(),
			DeviceId:     asset.DeviceID,
			Type:         immichv1.AssetType(asset.Type),
			OriginalPath: asset.OriginalPath,
			CreatedAt:    asset.CreatedAt.Unix(),
			UpdatedAt:    asset.UpdatedAt.Unix(),
		})
	}

	return &immichv1.SearchMetadataResponse{
		Assets: assets,
	}, nil
}

// SearchPeople searches for people
func (s *Server) SearchPeople(ctx context.Context, req *immichv1.SearchPeopleRequest) (*immichv1.SearchPeopleResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Search people
	results, err := s.searchService.SearchPeople(ctx, userID, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "people search failed: %v", err)
	}

	// Convert results
	var people []*immichv1.PersonResponse
	for _, person := range results {
		people = append(people, &immichv1.PersonResponse{
			Id:        person.ID.String(),
			Name:      person.Name,
			FaceCount: int32(person.FaceCount),
		})
	}

	return &immichv1.SearchPeopleResponse{
		People: people,
	}, nil
}

// SearchPlaces searches for places
func (s *Server) SearchPlaces(ctx context.Context, req *immichv1.SearchPlacesRequest) (*immichv1.SearchPlacesResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Search places
	results, err := s.searchService.SearchPlaces(ctx, userID, req.Query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "places search failed: %v", err)
	}

	// Convert results
	var places []*immichv1.PlaceResponse
	for _, place := range results {
		places = append(places, &immichv1.PlaceResponse{
			City:    place.City,
			State:   place.State,
			Country: place.Country,
		})
	}

	return &immichv1.SearchPlacesResponse{
		Places: places,
	}, nil
}

// GetSearchSuggestions returns search suggestions
func (s *Server) GetSearchSuggestions(ctx context.Context, req *immichv1.GetSearchSuggestionsRequest) (*immichv1.GetSearchSuggestionsResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Get suggestions
	suggestions, err := s.searchService.GetSuggestions(ctx, userID, req.Type, req.Query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get suggestions: %v", err)
	}

	return &immichv1.GetSearchSuggestionsResponse{
		Suggestions: suggestions,
	}, nil
}

// GetExploreData returns explore categories
func (s *Server) GetExploreData(ctx context.Context, _ *emptypb.Empty) (*immichv1.GetExploreDataResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Get explore data
	categories, err := s.searchService.GetExploreCategories(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get explore data: %v", err)
	}

	// Convert to proto format
	var items []*immichv1.ExploreItem
	for _, cat := range categories {
		items = append(items, &immichv1.ExploreItem{
			Label: cat.Label,
			Data:  cat.Data,
		})
	}

	return &immichv1.GetExploreDataResponse{
		Items: items,
	}, nil
}