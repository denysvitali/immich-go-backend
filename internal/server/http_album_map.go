package server

import (
	"net/http"
	"strings"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func albumMapMarkersIDFromPath(path string) (string, bool) {
	albumID, ok := strings.CutPrefix(path, "/api/albums/")
	if !ok {
		return "", false
	}
	albumID, ok = strings.CutSuffix(albumID, "/map-markers")
	if !ok || albumID == "" || strings.Contains(albumID, "/") {
		return "", false
	}
	return albumID, true
}

func (s *Server) handleAlbumMapMarkers(w http.ResponseWriter, r *http.Request, albumID string) {
	ctx, ok := s.frontendGatewayContext(w, r)
	if !ok {
		return
	}

	resp, err := s.GetAlbumMapMarkers(ctx, &immichv1.GetAlbumMapMarkersRequest{
		Id:   albumID,
		Key:  optionalStringQuery(r, "key"),
		Slug: optionalStringQuery(r, "slug"),
	})
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}

	writeProtoJSONArray(w, frontendProtoMarshaler(), resp.Markers)
}
