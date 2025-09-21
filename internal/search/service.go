package search

import (
	"context"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service handles search operations
type Service struct {
	db *sqlc.Queries
}

// NewService creates a new search service
func NewService(db *sqlc.Queries) *Service {
	return &Service{
		db: db,
	}
}

// SearchMetadata searches assets by metadata
func (s *Service) SearchMetadata(ctx context.Context, userID uuid.UUID, req MetadataSearchRequest) (*SearchResult, error) {
	// Build query parameters
	params := sqlc.SearchAssetsParams{
		OwnerId: UUIDToPgtype(userID),
		Column2: pgtype.Text{String: req.Query, Valid: true},
		Limit:   int32(req.Size),
		Offset:  int32(req.Page * req.Size),
	}

	// Note: Our basic search only supports text search for now
	// TODO: Add support for other metadata filters

	// Execute search
	assets, err := s.db.SearchAssets(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to search assets: %w", err)
	}

	// Get total count
	countParams := sqlc.CountSearchAssetsParams{
		OwnerId: params.OwnerId,
		Column2: params.Column2,
	}
	count, err := s.db.CountSearchAssets(ctx, countParams)
	if err != nil {
		return nil, fmt.Errorf("failed to count search results: %w", err)
	}

	// Convert to search results
	items := make([]*SearchResultItem, len(assets))
	for i, asset := range assets {
		items[i] = &SearchResultItem{
			ID:           PgtypeToUUID(asset.ID).String(),
			Type:         asset.Type,
			OriginalPath: asset.OriginalPath,
			OriginalName: asset.OriginalFileName,
			CreatedAt:    PgtypeToTime(asset.CreatedAt),
			UpdatedAt:    PgtypeToTime(asset.UpdatedAt),
			IsFavorite:   asset.IsFavorite,
			IsArchived:   false, // Not in current schema
			Duration:     &asset.Duration.String,
			// Add more fields as needed
		}
	}

	return &SearchResult{
		Items: items,
		Total: int(count),
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}

// SearchPeople searches for people/faces
func (s *Service) SearchPeople(ctx context.Context, userID uuid.UUID, req PeopleSearchRequest) (*PeopleSearchResult, error) {
	// Search for people
	people, err := s.db.SearchPeople(ctx, sqlc.SearchPeopleParams{
		OwnerId: UUIDToPgtype(userID),
		Column2: pgtype.Text{String: req.Query, Valid: true},
		Limit:   int32(req.Size),
		Offset:  int32(req.Page * req.Size),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search people: %w", err)
	}

	// Convert to search results
	items := make([]*PersonResult, len(people))
	for i, person := range people {
		items[i] = &PersonResult{
			ID:         PgtypeToUUID(person.ID).String(),
			Name:       person.Name,
			AssetCount: 0,  // Not returned by basic query
			Thumbnail:  "", // Not returned by basic query
		}
	}

	return &PeopleSearchResult{
		People: items,
		Total:  len(items),
	}, nil
}

// SearchPlaces searches for locations
func (s *Service) SearchPlaces(ctx context.Context, userID uuid.UUID, req PlacesSearchRequest) (*PlacesSearchResult, error) {
	// Search for places
	places, err := s.db.SearchPlaces(ctx, sqlc.SearchPlacesParams{
		OwnerId: UUIDToPgtype(userID),
		Column2: pgtype.Text{String: req.Query, Valid: true},
		Limit:   int32(req.Size),
		Offset:  int32(req.Page * req.Size),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search places: %w", err)
	}

	// Convert to search results
	items := make([]*PlaceResult, len(places))
	for i, place := range places {
		var city, state, country string
		if place.City.Valid {
			city = place.City.String
		}
		if place.State.Valid {
			state = place.State.String
		}
		if place.Country.Valid {
			country = place.Country.String
		}
		items[i] = &PlaceResult{
			City:       city,
			State:      state,
			Country:    country,
			AssetCount: 0, // Not returned by our query
		}
	}

	return &PlacesSearchResult{
		Places: items,
		Total:  len(items),
	}, nil
}

// SearchCities searches for cities
func (s *Service) SearchCities(ctx context.Context, userID uuid.UUID, req CitiesSearchRequest) ([]*CityResult, error) {
	cities, err := s.db.GetDistinctCities(ctx, sqlc.GetDistinctCitiesParams{
		OwnerId: UUIDToPgtype(userID),
		Limit:   int32(req.Size),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get cities: %w", err)
	}

	results := make([]*CityResult, len(cities))
	for i, city := range cities {
		var cityName string
		if city.Valid {
			cityName = city.String
		}
		results[i] = &CityResult{
			City:    cityName,
			State:   "", // Not returned by our query
			Country: "", // Not returned by our query
		}
	}

	return results, nil
}

// GetSearchSuggestions returns search suggestions based on user's data
func (s *Service) GetSearchSuggestions(ctx context.Context, userID uuid.UUID, req SuggestionsRequest) (*SuggestionsResult, error) {
	suggestions := &SuggestionsResult{
		People:    []string{},
		Places:    []string{},
		Tags:      []string{},
		Albums:    []string{},
		Cameras:   []string{},
		FileTypes: []string{},
	}

	// Get people suggestions
	if req.IncludePeople {
		people, err := s.db.GetTopPeople(ctx, sqlc.GetTopPeopleParams{
			OwnerId: UUIDToPgtype(userID),
			Limit:   10,
		})
		if err == nil {
			for _, p := range people {
				suggestions.People = append(suggestions.People, p.Name)
			}
		}
	}

	// Get place suggestions
	// TODO: Implement GetTopPlaces query
	// if req.IncludePlaces {
	// 	places, err := s.db.GetTopPlaces(ctx, sqlc.GetTopPlacesParams{
	// 		UserID: userID,
	// 		Limit:  10,
	// 	})
	// 	if err == nil {
	// 		for _, p := range places {
	// 			if p.City != "" {
	// 				suggestions.Places = append(suggestions.Places, p.City)
	// 			}
	// 		}
	// 	}
	// }

	// Get camera make/model suggestions
	// TODO: Implement GetDistinctCameras query
	// if req.IncludeCameras {
	// 	cameras, err := s.db.GetDistinctCameras(ctx, userID)
	// 	if err == nil {
	// 		for _, c := range cameras {
	// 			if c.Make != "" && c.Model != "" {
	// 				suggestions.Cameras = append(suggestions.Cameras, fmt.Sprintf("%s %s", c.Make, c.Model))
	// 			}
	// 		}
	// 	}
	// }

	return suggestions, nil
}

// SearchSmart performs AI-powered smart search (placeholder)
func (s *Service) SearchSmart(ctx context.Context, userID uuid.UUID, req SmartSearchRequest) (*SearchResult, error) {
	// This would integrate with a machine learning service
	// For now, fall back to metadata search
	return s.SearchMetadata(ctx, userID, MetadataSearchRequest{
		Query: req.Query,
		Page:  req.Page,
		Size:  req.Size,
	})
}

// SearchExplore returns explore/discovery results
func (s *Service) SearchExplore(ctx context.Context, userID uuid.UUID) (*ExploreResult, error) {
	result := &ExploreResult{
		Categories: []ExploreCategory{},
	}

	// TODO: Implement explore categories when queries are available
	return result, nil

	/* TODO: Uncomment when queries are implemented
	// This Year category
	thisYear := time.Now().Year()
	thisYearAssets, err := s.db.GetAssetsByYear(ctx, sqlc.GetAssetsByYearParams{
		UserID: userID,
		Year:   int32(thisYear),
		Limit:  12,
	})
	if err == nil && len(thisYearAssets) > 0 {
		result.Categories = append(result.Categories, ExploreCategory{
			Name:      fmt.Sprintf("This Year (%d)", thisYear),
			AssetIDs:  assetIDsToStrings(thisYearAssets),
			Thumbnail: thisYearAssets[0].ID.String(),
		})
	}

	// Recent uploads
	recentAssets, err := s.db.GetRecentAssets(ctx, sqlc.GetRecentAssetsParams{
		UserID: userID,
		Limit:  12,
	})
	if err == nil && len(recentAssets) > 0 {
		result.Categories = append(result.Categories, ExploreCategory{
			Name:      "Recently Added",
			AssetIDs:  assetIDsToStrings(recentAssets),
			Thumbnail: recentAssets[0].ID.String(),
		})
	}

	// Favorites
	favoriteAssets, err := s.db.GetFavoriteAssets(ctx, sqlc.GetFavoriteAssetsParams{
		UserID: userID,
		Limit:  12,
	})
	if err == nil && len(favoriteAssets) > 0 {
		result.Categories = append(result.Categories, ExploreCategory{
			Name:      "Favorites",
			AssetIDs:  assetIDsToStrings(favoriteAssets),
			Thumbnail: favoriteAssets[0].ID.String(),
		})
	}

	// Videos
	videoAssets, err := s.db.GetVideoAssets(ctx, sqlc.GetVideoAssetsParams{
		UserID: userID,
		Limit:  12,
	})
	if err == nil && len(videoAssets) > 0 {
		result.Categories = append(result.Categories, ExploreCategory{
			Name:      "Videos",
			AssetIDs:  assetIDsToStrings(videoAssets),
			Thumbnail: videoAssets[0].ID.String(),
		})
	}

	return result, nil
	*/
}

// Request/Response types

type MetadataSearchRequest struct {
	Query       string    `json:"query"`
	Type        string    `json:"type"` // image, video, all
	Page        int       `json:"page"`
	Size        int       `json:"size"`
	IsFavorite  *bool     `json:"isFavorite,omitempty"`
	IsArchived  *bool     `json:"isArchived,omitempty"`
	City        string    `json:"city,omitempty"`
	State       string    `json:"state,omitempty"`
	Country     string    `json:"country,omitempty"`
	Make        string    `json:"make,omitempty"`
	Model       string    `json:"model,omitempty"`
	TakenAfter  time.Time `json:"takenAfter,omitempty"`
	TakenBefore time.Time `json:"takenBefore,omitempty"`
	PersonIDs   []string  `json:"personIds,omitempty"`
	AlbumIDs    []string  `json:"albumIds,omitempty"`
}

type SearchResult struct {
	Items []*SearchResultItem `json:"items"`
	Total int                 `json:"total"`
	Page  int                 `json:"page"`
	Size  int                 `json:"size"`
}

type SearchResultItem struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	OriginalPath string    `json:"originalPath"`
	OriginalName string    `json:"originalName"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	IsFavorite   bool      `json:"isFavorite"`
	IsArchived   bool      `json:"isArchived"`
	Duration     *string   `json:"duration,omitempty"`
	// Add more fields as needed
}

type PeopleSearchRequest struct {
	Query string `json:"query"`
	Page  int    `json:"page"`
	Size  int    `json:"size"`
}

type PeopleSearchResult struct {
	People []*PersonResult `json:"people"`
	Total  int             `json:"total"`
}

type PersonResult struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AssetCount int    `json:"assetCount"`
	Thumbnail  string `json:"thumbnail"`
}

type PlacesSearchRequest struct {
	Query string `json:"query"`
	Page  int    `json:"page"`
	Size  int    `json:"size"`
}

type PlacesSearchResult struct {
	Places []*PlaceResult `json:"places"`
	Total  int            `json:"total"`
}

type PlaceResult struct {
	City       string `json:"city"`
	State      string `json:"state"`
	Country    string `json:"country"`
	AssetCount int    `json:"assetCount"`
}

type CitiesSearchRequest struct {
	Query string `json:"query"`
	Size  int    `json:"size"`
}

type CityResult struct {
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
}

type SuggestionsRequest struct {
	IncludePeople  bool `json:"includePeople"`
	IncludePlaces  bool `json:"includePlaces"`
	IncludeTags    bool `json:"includeTags"`
	IncludeAlbums  bool `json:"includeAlbums"`
	IncludeCameras bool `json:"includeCameras"`
}

type SuggestionsResult struct {
	People    []string `json:"people"`
	Places    []string `json:"places"`
	Tags      []string `json:"tags"`
	Albums    []string `json:"albums"`
	Cameras   []string `json:"cameras"`
	FileTypes []string `json:"fileTypes"`
}

type SmartSearchRequest struct {
	Query string `json:"query"`
	Page  int    `json:"page"`
	Size  int    `json:"size"`
}

type ExploreResult struct {
	Categories []ExploreCategory `json:"categories"`
}

type ExploreCategory struct {
	Name      string   `json:"name"`
	AssetIDs  []string `json:"assetIds"`
	Thumbnail string   `json:"thumbnail"`
}
