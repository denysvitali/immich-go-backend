package search

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/auth"
)

// Server implements the SearchServiceServer interface
type Server struct {
	immichv1.UnimplementedSearchServiceServer
	service *Service
}

// NewServer creates a new Search server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// SearchMetadata searches assets by metadata
func (s *Server) SearchMetadata(ctx context.Context, req *immichv1.SearchMetadataRequest) (*immichv1.SearchResponseDto, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	var isFavorite, isArchived *bool
	if req.IsFavorite != nil {
		f := boolValue(req.IsFavorite)
		isFavorite = &f
	}
	if req.IsArchived != nil {
		a := boolValue(req.IsArchived)
		isArchived = &a
	}

	searchReq := MetadataSearchRequest{
		City:           stringValue(req.City),
		State:          stringValue(req.State),
		Country:        stringValue(req.Country),
		Make:           stringValue(req.Make),
		Model:          stringValue(req.Model),
		IsFavorite:     isFavorite,
		IsArchived:     isArchived,
		Page:           0, // default page
		Size:           30, // default page size
	}

	// Parse date filters
	if req.TakenBefore != nil {
		searchReq.TakenBefore = req.TakenBefore.AsTime()
	}
	if req.TakenAfter != nil {
		searchReq.TakenAfter = req.TakenAfter.AsTime()
	}

	result, err := s.service.SearchMetadata(ctx, userID, searchReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert results to proto - simplified stub implementation
	assets := make([]*immichv1.AssetResponseDto, 0)
	for range result.Items {
		assets = append(assets, &immichv1.AssetResponseDto{
			Id:               uuid.New().String(),
			DeviceAssetId:    "device-asset-id",
			DeviceId:         "device-id",
			Type:             immichv1.AssetType_ASSET_TYPE_IMAGE,
			OriginalPath:     "/path/to/asset",
			OriginalFileName: "photo.jpg",
			IsFavorite:       false,
			IsArchived:       false,
			CreatedAt:        timestamppb.Now(),
			UpdatedAt:        timestamppb.Now(),
		})
	}

	return &immichv1.SearchResponseDto{
		Assets: assets,
		Total:  int32(result.Total),
		Count:  int32(len(result.Items)),
		Page:   0,
		Size:   30,
	}, nil
}

// SearchSmart performs smart search using ML embeddings
func (s *Server) SearchSmart(ctx context.Context, req *immichv1.SearchSmartRequest) (*immichv1.SearchSmartResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	searchReq := SmartSearchRequest{
		Query:    req.Query,
		Page:     0,
		Size:     30,
	}

	result, err := s.service.SearchSmart(ctx, userID, searchReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert results to proto
	assets := make([]*immichv1.AssetResponseDto, 0)
	for range result.Items {
		assets = append(assets, &immichv1.AssetResponseDto{
			Id:               uuid.New().String(),
			DeviceAssetId:    "device-asset-id",
			DeviceId:         "device-id",
			Type:             immichv1.AssetType_ASSET_TYPE_IMAGE,
			OriginalPath:     "/path/to/asset",
			OriginalFileName: "photo.jpg",
			IsFavorite:       false,
			IsArchived:       false,
			CreatedAt:        timestamppb.Now(),
			UpdatedAt:        timestamppb.Now(),
		})
	}

	return &immichv1.SearchSmartResponse{
		Assets: assets,
	}, nil
}

// SearchPerson searches for people/faces
func (s *Server) SearchPerson(ctx context.Context, req *immichv1.SearchPersonRequest) (*immichv1.SearchPersonResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	searchReq := PeopleSearchRequest{
		Query: req.Name,
		Page:  0,
		Size:  30,
	}

	result, err := s.service.SearchPeople(ctx, userID, searchReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert results to proto
	people := make([]*immichv1.PersonResponseDto, len(result.People))
	for i, person := range result.People {
		people[i] = &immichv1.PersonResponseDto{
			Id:            person.ID,
			Name:          person.Name,
			ThumbnailPath: person.Thumbnail,
			IsHidden:      false,
		}
	}

	return &immichv1.SearchPersonResponse{
		People: people,
	}, nil
}

// SearchPlaces searches for places
func (s *Server) SearchPlaces(ctx context.Context, req *immichv1.SearchPlacesRequest) (*immichv1.SearchPlacesResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	searchReq := PlacesSearchRequest{
		Query: req.Name,
		Page:  0,
		Size:  30,
	}

	result, err := s.service.SearchPlaces(ctx, userID, searchReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert results to proto
	places := make([]*immichv1.PlaceResponseDto, len(result.Places))
	for i, place := range result.Places {
		places[i] = &immichv1.PlaceResponseDto{
			Name:      place.City,
			Latitude:  0.0,
			Longitude: 0.0,
			Admin1:    place.State,
			Admin2:    place.City,
		}
	}

	return &immichv1.SearchPlacesResponse{
		Places: places,
	}, nil
}

// SearchCities searches for cities
func (s *Server) SearchCities(ctx context.Context, req *immichv1.SearchCitiesRequest) (*immichv1.SearchCitiesResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	searchReq := CitiesSearchRequest{
		Query: "",
		Size:  30,
	}

	cities, err := s.service.SearchCities(ctx, userID, searchReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert results to proto
	cityNames := make([]string, len(cities))
	for i, city := range cities {
		cityNames[i] = city.City
	}

	return &immichv1.SearchCitiesResponse{
		Cities: cityNames,
	}, nil
}

// GetSearchSuggestions gets search suggestions
func (s *Server) GetSearchSuggestions(ctx context.Context, req *immichv1.GetSearchSuggestionsRequest) (*immichv1.GetSearchSuggestionsResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	suggestReq := SuggestionsRequest{
		IncludePeople:  stringValue(req.Country) != "",
		IncludePlaces:  stringValue(req.State) != "",
		IncludeCameras: stringValue(req.Make) != "",
	}

	result, err := s.service.GetSearchSuggestions(ctx, userID, suggestReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Combine all suggestions into a single list
	var suggestions []string
	suggestions = append(suggestions, result.People...)
	suggestions = append(suggestions, result.Places...)
	suggestions = append(suggestions, result.Cameras...)

	return &immichv1.GetSearchSuggestionsResponse{
		Suggestions: suggestions,
	}, nil
}

// SearchExplore gets explore/discovery data
func (s *Server) SearchExplore(ctx context.Context, req *emptypb.Empty) (*immichv1.SearchExploreResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	result, err := s.service.SearchExplore(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert results to proto
	items := make([]*immichv1.SearchExploreItemResponseDto, 0)
	// Stub implementation - would need proper mapping
	for _, category := range result.Categories {
		items = append(items, &immichv1.SearchExploreItemResponseDto{
			FieldName: category.Name,
		Items: []*immichv1.SearchExploreItemValueResponseDto{
			{
				Value: "example",
				Data: &immichv1.AssetResponseDto{
					Id: uuid.New().String(),
				},
			},
		},
		})
	}

	return &immichv1.SearchExploreResponse{
		Items: items,
	}, nil
}

// Search performs a general search
func (s *Server) Search(ctx context.Context, req *immichv1.SearchRequest) (*immichv1.SearchResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Stub implementation
	_ = userID
	_ = req

	return &immichv1.SearchResponse{
		Albums: []*immichv1.AlbumResponseDto{},
		Assets: []*immichv1.AssetResponseDto{},
		Total:  0,
	}, nil
}

// Helper functions
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func boolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}