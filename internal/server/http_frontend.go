package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/timeline"
)

// frontendShapeHandlers intercepts a small set of REST endpoints whose default
// grpc-gateway response shape differs from the upstream Immich SDK/web UI.
func (s *Server) handleFrontendShape(w http.ResponseWriter, r *http.Request) (handled bool) {
	if r.Method != http.MethodGet {
		return false
	}

	switch r.URL.Path {
	case "/api/timeline/buckets":
		s.handleTimelineBuckets(w, r)
		return true
	case "/api/timeline/bucket":
		s.handleTimelineBucket(w, r)
		return true
	case "/api/albums":
		s.handleAlbums(w, r)
		return true
	}

	return false
}

func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	authHeader := requestAuthorization(r)
	if !strings.HasPrefix(authHeader, "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, false
	}

	claims, err := s.authService.ValidateToken(strings.TrimPrefix(authHeader, "Bearer "))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, false
	}

	return claims, true
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func parseBoolQuery(r *http.Request, key string) bool {
	v := r.URL.Query().Get(key)
	b, _ := strconv.ParseBool(v)
	return b
}

func (s *Server) handleTimelineBuckets(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	opts := timeline.ListOptions{
		UserID:     claims.UserID,
		Bucket:     "day",
		IsFavorite: parseBoolQuery(r, "isFavorite"),
		IsTrashed:  parseBoolQuery(r, "isTrashed"),
	}

	buckets, err := s.timelineService.GetTimeBuckets(r.Context(), opts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	resp := make([]map[string]any, len(buckets))
	for i, b := range buckets {
		resp[i] = map[string]any{
			"timeBucket": b.Date,
			"count":      b.Count,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleTimelineBucket(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	timeBucket := r.URL.Query().Get("timeBucket")
	if timeBucket == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing timeBucket"})
		return
	}

	bucketDate, layout, err := parseTimeBucket(timeBucket)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid timeBucket"})
		return
	}

	opts := timeline.ListOptions{
		UserID:     claims.UserID,
		Bucket:     bucketSizeForLayout(layout),
		Date:       bucketDate.Format("2006-01-02"),
		IsFavorite: parseBoolQuery(r, "isFavorite"),
		IsTrashed:  parseBoolQuery(r, "isTrashed"),
		Limit:      500,
	}

	assets, err := s.timelineService.GetBucketAssets(r.Context(), opts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	resp := map[string]any{
		"id":               make([]string, len(assets)),
		"city":             make([]*string, len(assets)),
		"country":          make([]*string, len(assets)),
		"duration":         make([]*string, len(assets)),
		"fileCreatedAt":    make([]string, len(assets)),
		"isFavorite":       make([]bool, len(assets)),
		"isImage":          make([]bool, len(assets)),
		"isTrashed":        make([]bool, len(assets)),
		"latitude":         make([]*float64, len(assets)),
		"livePhotoVideoId": make([]*string, len(assets)),
		"localOffsetHours": make([]float64, len(assets)),
		"longitude":        make([]*float64, len(assets)),
		"ownerId":          make([]string, len(assets)),
		"projectionType":   make([]*string, len(assets)),
		"ratio":            make([]float64, len(assets)),
		"stack":            make([][]string, len(assets)),
		"thumbhash":        make([]*string, len(assets)),
		"type":             make([]string, len(assets)),
		"width":            make([]*int32, len(assets)),
		"height":           make([]*int32, len(assets)),
		"originalFileName": make([]string, len(assets)),
		"originalPath":     make([]string, len(assets)),
		"exifImageWidth":   make([]*int32, len(assets)),
		"exifImageHeight":  make([]*int32, len(assets)),
		"exifInfo":         make([]map[string]any, len(assets)),
	}

	for i, a := range assets {
		resp["id"].([]string)[i] = a.ID.String()
		resp["city"].([]*string)[i] = a.City
		resp["country"].([]*string)[i] = a.Country
		resp["duration"].([]*string)[i] = a.Duration
		resp["fileCreatedAt"].([]string)[i] = a.FileCreatedAt.Format(time.RFC3339)
		resp["isFavorite"].([]bool)[i] = a.IsFavorite
		resp["isImage"].([]bool)[i] = a.Type == "IMAGE"
		resp["isTrashed"].([]bool)[i] = a.Status == "trashed"
		resp["latitude"].([]*float64)[i] = a.Latitude
		resp["longitude"].([]*float64)[i] = a.Longitude
		resp["ownerId"].([]string)[i] = a.OwnerId.String()
		resp["projectionType"].([]*string)[i] = a.ProjectionType
		resp["localOffsetHours"].([]float64)[i] = 0
		resp["type"].([]string)[i] = a.Type
		resp["originalFileName"].([]string)[i] = a.OriginalFileName
		resp["originalPath"].([]string)[i] = a.OriginalPath
		resp["width"].([]*int32)[i] = a.Width
		resp["height"].([]*int32)[i] = a.Height
		resp["exifImageWidth"].([]*int32)[i] = a.Width
		resp["exifImageHeight"].([]*int32)[i] = a.Height

		if a.LivePhotoVideoId != nil {
			id := a.LivePhotoVideoId.String()
			resp["livePhotoVideoId"].([]*string)[i] = &id
		}
		if a.Thumbhash != nil {
			resp["thumbhash"].([]*string)[i] = a.Thumbhash
		}

		var ratio float64
		if a.Width != nil && a.Height != nil && *a.Height != 0 {
			ratio = float64(*a.Width) / float64(*a.Height)
		} else {
			ratio = 1
		}
		resp["ratio"].([]float64)[i] = ratio

		if a.StackId != nil {
			resp["stack"].([][]string)[i] = []string{a.StackId.String()}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAlbums(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	userUUID, err := pgutil.StringToUUID(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid user id"})
		return
	}

	albums, err := s.db.GetAlbumsByOwner(r.Context(), userUUID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	resp := make([]map[string]any, len(albums))
	for i, album := range albums {
		item := map[string]any{
			"id":                    album.ID.String(),
			"albumName":             album.AlbumName,
			"description":           album.Description,
			"ownerId":               album.OwnerId.String(),
			"assetCount":            0,
			"assets":                []any{},
			"albumUsers":            []any{},
			"shared":                false,
			"hasSharedLink":         false,
			"isActivityEnabled":     album.IsActivityEnabled,
			"createdAt":             album.CreatedAt.Time.Format(time.RFC3339),
			"updatedAt":             album.UpdatedAt.Time.Format(time.RFC3339),
			"albumThumbnailAssetId": nil,
		}
		if album.AlbumThumbnailAssetId.Valid {
			item["albumThumbnailAssetId"] = album.AlbumThumbnailAssetId.String()
		}
		resp[i] = item
	}

	writeJSON(w, http.StatusOK, resp)
}
