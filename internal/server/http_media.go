package server

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// Asset media endpoints (thumbnail, original, video playback) must return raw
// bytes: browsers load them via <img>/<video> tags, and the mobile app streams
// them directly. The grpc-gateway route would JSON-encode the proto response
// ({"data":"<base64>","contentType":...}), which no client can display, so
// these are intercepted before the gateway mux.

type assetMediaKind int

const (
	assetMediaNone assetMediaKind = iota
	assetMediaThumbnail
	assetMediaOriginal
	assetMediaVideoPlayback
)

func assetMediaRouteFromPath(path string) (string, assetMediaKind) {
	rest, ok := strings.CutPrefix(path, "/api/assets/")
	if !ok {
		return "", assetMediaNone
	}
	assetID, action, ok := strings.Cut(rest, "/")
	if !ok || assetID == "" {
		return "", assetMediaNone
	}
	switch action {
	case "thumbnail":
		return assetID, assetMediaThumbnail
	case "original":
		return assetID, assetMediaOriginal
	case "video/playback":
		return assetID, assetMediaVideoPlayback
	}
	return "", assetMediaNone
}

func (s *Server) handleAssetMedia(w http.ResponseWriter, r *http.Request, assetID string, kind assetMediaKind) {
	ctx := gatewayIncomingContext(r)

	switch kind {
	case assetMediaThumbnail:
		request := &immichv1.GetAssetThumbnailRequest{AssetId: assetID}
		if size := r.URL.Query().Get("size"); size != "" {
			request.Size = &size
		}
		response, err := s.GetAssetThumbnail(ctx, request)
		if err != nil {
			writeGrpcError(w, err)
			return
		}
		writeMediaBytes(w, r, response.GetContentType(), response.GetData())

	case assetMediaOriginal:
		response, err := s.DownloadAsset(ctx, &immichv1.DownloadAssetRequest{AssetId: assetID})
		if err != nil {
			writeGrpcError(w, err)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", response.GetFilename()))
		writeMediaBytes(w, r, response.GetContentType(), response.GetData())

	case assetMediaVideoPlayback:
		response, err := s.PlayAssetVideo(ctx, &immichv1.PlayAssetVideoRequest{AssetId: assetID})
		if err != nil {
			writeGrpcError(w, err)
			return
		}
		writeMediaBytes(w, r, response.GetContentType(), response.GetData())
	}
}

// writeMediaBytes serves an in-memory media payload with Range support so
// <video> seeking and partial image loads work.
func writeMediaBytes(w http.ResponseWriter, r *http.Request, contentType string, data []byte) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}
