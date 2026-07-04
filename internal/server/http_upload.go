package server

import (
	"crypto/sha1" //nolint:gosec // Immich uses SHA-1 asset checksums; not for crypto.
	"encoding/hex"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// writeGrpcError converts a gRPC status error into the upstream-style JSON
// error envelope with the mapped HTTP status code.
func writeGrpcError(w http.ResponseWriter, err error) {
	st := status.Convert(err)
	code := runtime.HTTPStatusFromCode(st.Code())
	writeJSON(w, code, map[string]any{
		"message":    st.Message(),
		"statusCode": code,
	})
}

// maxUploadMemory is the in-memory buffer for multipart parsing; larger
// bodies spill to temp files.
const maxUploadMemory = 32 << 20

// maxUploadSize caps a single upload request. UploadAsset currently buffers
// the file in memory, so this also bounds per-request memory.
const maxUploadSize = 1 << 30

// handleAssetUpload implements the upstream `POST /api/assets` multipart
// upload (AssetMediaCreateDto). The grpc-gateway route only accepts JSON, so
// the real web/mobile upload flow — multipart/form-data with an `assetData`
// file part — must be handled before the gateway mux.
func (s *Server) handleAssetUpload(w http.ResponseWriter, r *http.Request) {
	ctx := gatewayIncomingContext(r)

	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"message":    "Unauthorized",
			"statusCode": http.StatusUnauthorized,
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	//nolint:gosec // G120: the body is bounded by MaxBytesReader above.
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid multipart form: " + err.Error()})
		return
	}
	defer func() {
		_ = r.MultipartForm.RemoveAll()
	}()

	file, header, err := r.FormFile("assetData")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing assetData file"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "failed to read assetData"})
		return
	}

	sum := sha1.Sum(content) //nolint:gosec // Immich asset checksum convention.
	checksum := hex.EncodeToString(sum[:])

	// Duplicate detection: same convention as UploadAsset — checksums are
	// stored as the bytes of the hex string.
	if existing, err := s.db.GetAssetsByChecksum(ctx, []byte(checksum)); err == nil {
		for _, a := range existing {
			if a.OwnerId.Valid && a.OwnerId.String() == claims.UserID {
				writeJSON(w, http.StatusOK, map[string]any{
					"id":     a.ID.String(),
					"status": "duplicate",
				})
				return
			}
		}
	}

	form := r.MultipartForm.Value
	formValue := func(key string) string {
		if v, ok := form[key]; ok && len(v) > 0 {
			return v[0]
		}
		return ""
	}

	originalFileName := formValue("filename")
	if originalFileName == "" {
		originalFileName = header.Filename
	}

	assetType := immichv1.AssetType_ASSET_TYPE_IMAGE
	if isVideoUpload(header.Header.Get("Content-Type"), originalFileName) {
		assetType = immichv1.AssetType_ASSET_TYPE_VIDEO
	}

	assetData := &immichv1.CreateAssetRequest{
		DeviceAssetId:    formValue("deviceAssetId"),
		DeviceId:         formValue("deviceId"),
		Type:             assetType,
		OriginalFileName: originalFileName,
	}
	if ts, ok := parseUploadTime(formValue("fileCreatedAt")); ok {
		assetData.FileCreatedAt = ts
	}
	if ts, ok := parseUploadTime(formValue("fileModifiedAt")); ok {
		assetData.FileModifiedAt = ts
	}
	if v := formValue("duration"); v != "" {
		assetData.Duration = &v
	}
	if v := formValue("isFavorite"); v != "" {
		fav, _ := strconv.ParseBool(v)
		assetData.IsFavorite = &fav
	}

	asset, err := s.UploadAsset(ctx, &immichv1.UploadAssetRequest{
		AssetData:   assetData,
		Checksum:    &checksum,
		FileContent: content,
	})
	if err != nil {
		writeGrpcError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     asset.Id,
		"status": "created",
	})
}

// parseUploadTime accepts the timestamp formats the Immich clients send
// (RFC3339 or unix epoch milliseconds).
func parseUploadTime(v string) (*timestamppb.Timestamp, bool) {
	if v == "" {
		return nil, false
	}
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return timestamppb.New(t), true
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return timestamppb.New(t), true
	}
	if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
		return timestamppb.New(time.UnixMilli(ms)), true
	}
	return nil, false
}

func isVideoUpload(contentType, filename string) bool {
	if strings.HasPrefix(contentType, "video/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp4", ".mov", ".avi", ".mkv", ".webm", ".m4v", ".3gp", ".mts", ".m2ts", ".wmv", ".flv":
		return true
	}
	return false
}
