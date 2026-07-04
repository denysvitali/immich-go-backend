package server

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/immich-go-backend/internal/download"
)

// downloadRequestBody mirrors upstream DownloadInfoDto / AssetIdsDto.
type downloadRequestBody struct {
	AssetIDs []string `json:"assetIds"`
	AlbumID  *string  `json:"albumId"`
}

func decodeDownloadRequest(r *http.Request) (*download.DownloadRequest, error) {
	var body downloadRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	return &download.DownloadRequest{
		AssetIDs: body.AssetIDs,
		AlbumID:  body.AlbumID,
	}, nil
}

// handleDownloadInfo implements upstream `POST /api/download/info`
// ({totalSize, archives:[{size, assetIds}]}). The gateway route's proto
// response has a different shape, so the web UI is served here.
func (s *Server) handleDownloadInfo(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid user id"})
		return
	}

	req, err := decodeDownloadRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	info, err := s.downloadService.GetDownloadInfo(r.Context(), userID, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"totalSize": info.TotalSize,
		"archives": []map[string]any{
			{
				"size":     info.TotalSize,
				"assetIds": info.AssetIDs,
			},
		},
	})
}

// handleDownloadArchive implements upstream `POST /api/download/archive`,
// streaming a zip of the requested assets. The proto RPC is metadata-only and
// cannot carry the archive bytes, so the real download happens here.
func (s *Server) handleDownloadArchive(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid user id"})
		return
	}

	req, err := decodeDownloadRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="immich.zip"`)
	if err := s.downloadService.DownloadArchive(r.Context(), userID, req, w); err != nil {
		// Headers are already sent once the first zip bytes are written; at
		// this point all we can do is abort the stream.
		logrus.WithError(err).Warn("download archive streaming failed")
	}
}
