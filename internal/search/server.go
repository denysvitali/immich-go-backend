package search

import (
	"context"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	assetdomain "github.com/denysvitali/immich-go-backend/internal/assets"
	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/grpcutil"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
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

	searchReq := MetadataSearchRequest{
		Page: 0,
		Size: 30,
	}

	if filter := req.GetFilter(); filter != nil {
		searchReq.Query = stringValue(filter.OriginalFileName)
		if searchReq.Query == "" {
			searchReq.Query = stringValue(filter.OriginalPath)
		}
		searchReq.City = stringValue(filter.City)
		searchReq.State = stringValue(filter.State)
		searchReq.Country = stringValue(filter.Country)
		searchReq.Make = stringValue(filter.Make)
		searchReq.Model = stringValue(filter.Model)
		searchReq.LensModel = stringValue(filter.LensModel)
		searchReq.LibraryID = stringValue(filter.LibraryId)
		searchReq.IsFavorite = filter.IsFavorite
		searchReq.IsArchived = filter.IsArchived
		searchReq.IsEncoded = filter.IsEncoded
		searchReq.IsMotion = filter.IsMotion
		searchReq.IsOffline = filter.IsOffline
		searchReq.IsExternal = filter.IsExternal
		if filter.Type != nil {
			searchReq.Type = assetTypeFilter(*filter.Type)
		}
		if filter.TakenBefore != nil {
			searchReq.TakenBefore = filter.TakenBefore.AsTime()
		}
		if filter.TakenAfter != nil {
			searchReq.TakenAfter = filter.TakenAfter.AsTime()
		}
		if filter.Size != nil && filter.GetSize() > 0 {
			searchReq.Size = int(filter.GetSize())
		}
		if filter.Page != nil && filter.GetPage() > 0 {
			searchReq.Page = int(filter.GetPage())
		}
	}

	result, err := s.service.SearchMetadata(ctx, userID, searchReq)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "search failed", err)
	}

	// Convert real search results to proto
	assets := make([]*immichv1.AssetResponseDto, 0)
	for _, item := range result.Items {
		// Get full asset details
		id, err := uuid.Parse(item.ID)
		if err != nil {
			continue // Skip invalid UUIDs
		}
		assetUUID := pgtype.UUID{Bytes: id, Valid: true}
		asset, err := s.service.db.GetAssetByID(ctx, assetUUID)
		if err != nil {
			continue // Skip assets that can't be loaded
		}

		assets = append(assets, assetToSearchResponseDto(asset))
	}

	return &immichv1.SearchResponseDto{
		Assets: assets,
		Total:  int32(result.Total),
		Count:  int32(len(result.Items)),
		Page:   int32(result.Page),
		Size:   int32(result.Size),
	}, nil
}

// SearchSmart performs smart search using ML embeddings
func (s *Server) SearchSmart(ctx context.Context, req *immichv1.SearchSmartRequest) (*immichv1.SearchSmartResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	searchReq := SmartSearchRequest{
		Query: req.Query,
		Page:  0,
		Size:  30,
	}

	result, err := s.service.SearchSmart(ctx, userID, searchReq)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "smart search failed", err)
	}

	// Convert real search results to proto
	assets := make([]*immichv1.AssetResponseDto, 0)
	for _, item := range result.Items {
		// Get full asset details
		id, err := uuid.Parse(item.ID)
		if err != nil {
			continue // Skip invalid UUIDs
		}
		assetUUID := pgtype.UUID{Bytes: id, Valid: true}
		asset, err := s.service.db.GetAssetByID(ctx, assetUUID)
		if err != nil {
			continue // Skip assets that can't be loaded
		}

		assets = append(assets, &immichv1.AssetResponseDto{
			Id:               item.ID,
			DeviceAssetId:    asset.DeviceAssetId,
			DeviceId:         asset.DeviceId,
			Type:             assetdomain.AssetTypeFromString(asset.Type),
			OriginalPath:     asset.OriginalPath,
			OriginalFileName: asset.OriginalFileName,
			IsFavorite:       asset.IsFavorite,
			IsArchived:       asset.Visibility == "archive",
			CreatedAt:        timestamppb.New(asset.CreatedAt.Time),
			UpdatedAt:        timestamppb.New(asset.UpdatedAt.Time),
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
		Query:      req.Name,
		Page:       0,
		Size:       30,
		WithHidden: req.GetWithHidden(),
	}

	result, err := s.service.SearchPeople(ctx, userID, searchReq)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "person search failed", err)
	}

	// Convert results to proto
	people := make([]*immichv1.PersonResponseDto, len(result.People))
	for i, person := range result.People {
		people[i] = &immichv1.PersonResponseDto{
			Id:            person.ID,
			Name:          person.Name,
			BirthDate:     stringValue(person.BirthDate),
			ThumbnailPath: person.ThumbnailPath,
			IsHidden:      person.IsHidden,
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
		return nil, grpcutil.SanitizedInternal(ctx, "place search failed", err)
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
		return nil, grpcutil.SanitizedInternal(ctx, "city search failed", err)
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
		return nil, grpcutil.SanitizedInternal(ctx, "search suggestions failed", err)
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
		return nil, grpcutil.SanitizedInternal(ctx, "explore search failed", err)
	}

	// Convert results to proto
	items := make([]*immichv1.SearchExploreItemResponseDto, 0)
	// Convert actual search results from database
	for _, category := range result.Categories {
		// Create value items from category data
		valueItems := make([]*immichv1.SearchExploreItemValueResponseDto, 0)

		// Add a value item for this category
		// In a real implementation, this would be populated with actual data
		if len(category.AssetIDs) > 0 {
			valueItems = append(valueItems, &immichv1.SearchExploreItemValueResponseDto{
				Value: category.Name,
				Data:  nil, // Would be populated with actual asset data from DB
			})
		}

		items = append(items, &immichv1.SearchExploreItemResponseDto{
			FieldName: category.Name,
			Items:     valueItems,
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

	// Search assets
	var assets []*immichv1.AssetResponseDto
	if req.GetQuery() != "" {
		pgUserID := pgtype.UUID{Bytes: userID, Valid: true}
		searchResults, err := s.service.db.SearchAssets(ctx, sqlc.SearchAssetsParams{
			OwnerId: pgUserID,
			Column2: pgtype.Text{String: req.GetQuery(), Valid: true},
			Limit:   50,
			Offset:  0,
		})
		if err == nil {
			assets = make([]*immichv1.AssetResponseDto, 0, len(searchResults))
			for _, asset := range searchResults {
				assets = append(assets, &immichv1.AssetResponseDto{
					Id:               uuid.UUID(asset.ID.Bytes).String(),
					DeviceAssetId:    asset.DeviceAssetId,
					DeviceId:         asset.DeviceId,
					Type:             assetdomain.AssetTypeFromString(asset.Type),
					OriginalPath:     asset.OriginalPath,
					OriginalFileName: asset.OriginalFileName,
					IsFavorite:       asset.IsFavorite,
					IsArchived:       asset.Visibility == "archive",
					CreatedAt:        timestamppb.New(asset.CreatedAt.Time),
					UpdatedAt:        timestamppb.New(asset.UpdatedAt.Time),
				})
			}
		}
	}

	// Search albums
	var albums []*immichv1.AlbumResponseDto
	if req.GetQuery() != "" {
		pgUserID := pgtype.UUID{Bytes: userID, Valid: true}
		albumResults, err := s.service.db.SearchAlbums(ctx, sqlc.SearchAlbumsParams{
			OwnerId: pgUserID,
			Column2: pgtype.Text{String: req.GetQuery(), Valid: true},
			Limit:   50,
			Offset:  0,
		})
		if err == nil {
			albums = make([]*immichv1.AlbumResponseDto, 0, len(albumResults))
			for _, album := range albumResults {
				albums = append(albums, &immichv1.AlbumResponseDto{
					Id:          uuid.UUID(album.ID.Bytes).String(),
					AlbumName:   album.AlbumName,
					Description: album.Description,
					CreatedAt:   timestamppb.New(album.CreatedAt.Time),
					UpdatedAt:   timestamppb.New(album.UpdatedAt.Time),
					AssetCount:  0, // Would need a join query to get this
					Owner: &immichv1.User{
						Id: uuid.UUID(album.OwnerId.Bytes).String(),
					},
				})
			}
		}
	}

	return &immichv1.SearchResponse{
		Albums: albums,
		Assets: assets,
		Total:  int32(len(albums) + len(assets)),
	}, nil
}

// SearchRandom returns a randomized page of assets matching optional metadata filters.
func (s *Server) SearchRandom(ctx context.Context, req *immichv1.SearchRandomRequest) (*immichv1.SearchRandomResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	params := sqlc.SearchRandomAssetsParams{
		OwnerID:     pgtype.UUID{Bytes: userID, Valid: true},
		Type:        optionalAssetType(req.Type),
		IsFavorite:  optionalBool(req.IsFavorite),
		City:        optionalText(req.GetCity()),
		State:       optionalText(req.GetState()),
		Country:     optionalText(req.GetCountry()),
		Make:        optionalText(req.GetMake()),
		Model:       optionalText(req.GetModel()),
		LensModel:   optionalText(req.GetLensModel()),
		LibraryID:   optionalUUID(req.GetLibraryId()),
		DeviceID:    optionalText(req.GetDeviceId()),
		Limit:       optionalLimit(req.GetSize(), 100),
		TakenAfter:  timestampToPg(req.GetTakenAfter()),
		TakenBefore: timestampToPg(req.GetTakenBefore()),
	}
	if req.WithDeleted != nil {
		params.WithDeleted = pgtype.Bool{Bool: req.GetWithDeleted(), Valid: true}
	}

	assets, err := s.service.db.SearchRandomAssets(ctx, params)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "random search failed", err)
	}

	return &immichv1.SearchRandomResponse{Assets: assetsToSearchResponseDtos(assets)}, nil
}

// SearchLargeAssets returns the largest assets for the current user.
func (s *Server) SearchLargeAssets(ctx context.Context, req *immichv1.SearchLargeAssetsRequest) (*immichv1.SearchLargeAssetsResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	size := req.GetSize()
	if size <= 0 {
		size = 100
	}

	assets, err := s.service.db.SearchLargeAssets(ctx, sqlc.SearchLargeAssetsParams{
		OwnerId: pgtype.UUID{Bytes: userID, Valid: true},
		Limit:   size,
	})
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "large asset search failed", err)
	}

	return &immichv1.SearchLargeAssetsResponse{Assets: assetsToSearchResponseDtos(assets)}, nil
}

// SearchAssetStatistics returns an asset count for the supplied metadata filters.
func (s *Server) SearchAssetStatistics(ctx context.Context, req *immichv1.SearchAssetStatisticsRequest) (*immichv1.SearchAssetStatisticsResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	total, err := s.service.db.CountSearchAssetsFiltered(ctx, sqlc.CountSearchAssetsFilteredParams{
		OwnerID:     pgtype.UUID{Bytes: userID, Valid: true},
		Type:        optionalAssetType(req.Type),
		IsFavorite:  optionalBool(req.IsFavorite),
		City:        optionalText(req.GetCity()),
		State:       optionalText(req.GetState()),
		Country:     optionalText(req.GetCountry()),
		Make:        optionalText(req.GetMake()),
		Model:       optionalText(req.GetModel()),
		LensModel:   optionalText(req.GetLensModel()),
		LibraryID:   optionalUUID(req.GetLibraryId()),
		DeviceID:    optionalText(req.GetDeviceId()),
		TakenAfter:  timestampToPg(req.GetTakenAfter()),
		TakenBefore: timestampToPg(req.GetTakenBefore()),
	})
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "asset statistics search failed", err)
	}

	return &immichv1.SearchAssetStatisticsResponse{Total: total}, nil
}

// Helper functions
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func assetsToSearchResponseDtos(assets []sqlc.Asset) []*immichv1.AssetResponseDto {
	dtos := make([]*immichv1.AssetResponseDto, len(assets))
	for i, asset := range assets {
		dtos[i] = assetToSearchResponseDto(asset)
	}
	return dtos
}

func assetToSearchResponseDto(asset sqlc.Asset) *immichv1.AssetResponseDto {
	dto := &immichv1.AssetResponseDto{
		Id:               uuid.UUID(asset.ID.Bytes).String(),
		DeviceAssetId:    asset.DeviceAssetId,
		DeviceId:         asset.DeviceId,
		Type:             assetdomain.AssetTypeFromString(asset.Type),
		OriginalPath:     asset.OriginalPath,
		OriginalFileName: asset.OriginalFileName,
		IsFavorite:       asset.IsFavorite,
		IsArchived:       asset.Visibility == "archive",
		IsTrashed:        asset.DeletedAt.Valid || asset.Status == "trashed",
		CreatedAt:        timestamppb.New(asset.CreatedAt.Time),
		UpdatedAt:        timestamppb.New(asset.UpdatedAt.Time),
		FileCreatedAt:    timestamppb.New(asset.FileCreatedAt.Time),
		FileModifiedAt:   timestamppb.New(asset.FileModifiedAt.Time),
		Checksum:         hex.EncodeToString(asset.Checksum),
		IsExternal:       asset.IsExternal,
		IsOffline:        asset.IsOffline,
		Owner: &immichv1.User{
			Id: uuid.UUID(asset.OwnerId.Bytes).String(),
		},
	}
	if asset.Duration.Valid {
		dto.Duration = asset.Duration.String
	}
	if asset.LibraryId.Valid {
		dto.LibraryId = uuid.UUID(asset.LibraryId.Bytes).String()
	}
	if asset.StackId.Valid {
		dto.StackId = uuid.UUID(asset.StackId.Bytes).String()
	}
	if asset.DuplicateId.Valid {
		duplicateID := uuid.UUID(asset.DuplicateId.Bytes).String()
		dto.DuplicateId = &duplicateID
	}
	if len(asset.Thumbhash) > 0 {
		dto.Thumbhash = string(asset.Thumbhash)
	}
	return dto
}

func optionalAssetType(value *immichv1.AssetType) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return optionalText(assetTypeFilter(*value))
}

func assetTypeFilter(value immichv1.AssetType) string {
	switch value {
	case immichv1.AssetType_ASSET_TYPE_IMAGE:
		return "IMAGE"
	case immichv1.AssetType_ASSET_TYPE_VIDEO:
		return "VIDEO"
	case immichv1.AssetType_ASSET_TYPE_AUDIO:
		return "AUDIO"
	case immichv1.AssetType_ASSET_TYPE_OTHER:
		return "OTHER"
	default:
		return ""
	}
}

func optionalLimit(size int32, fallback int32) pgtype.Int4 {
	if size <= 0 {
		size = fallback
	}
	return pgtype.Int4{Int32: size, Valid: true}
}

func timestampToPg(ts *timestamppb.Timestamp) pgtype.Timestamptz {
	if ts == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: ts.AsTime(), Valid: true}
}
