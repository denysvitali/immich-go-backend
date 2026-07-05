package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/ffmpeg"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

const (
	hlsPlaylistContentType = "application/vnd.apple.mpegurl"
	hlsSegmentContentType  = "application/octet-stream"
	hlsVersion             = 7
	hlsVariantIndex        = uint64(0)
	hlsSegmentDuration     = 2
	hlsMaxWidth            = 1920
	hlsMaxHeight           = 1080
	hlsDefaultBandwidth    = 2_500_000
	hlsSessionTTL          = 6 * time.Hour
)

var (
	hlsGenerationMu       sync.Mutex
	hlsSegmentFilenameRE  = regexp.MustCompile(`^(init\.mp4|seg_\d+\.m4s)$`)
	hlsSessionIDNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("github.com/denysvitali/immich-go-backend/hls"))
)

type hlsMetadata struct {
	Width     int `json:"width"`
	Height    int `json:"height"`
	Bandwidth int `json:"bandwidth"`
}

func (s *Server) GetMainPlaylist(ctx context.Context, req *immichv1.GetMainPlaylistRequest) (*immichv1.HLSPlaylistResponse, error) {
	asset, assetUUID, err := s.hlsAsset(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	sessionID := hlsSessionID(assetUUID)
	if err := s.ensureHLS(ctx, asset, sessionID); err != nil {
		return nil, err
	}

	metadata := readHLSMetadata(sessionID)
	return &immichv1.HLSPlaylistResponse{
		Data:        buildHLSMainPlaylist(sessionID, metadata),
		ContentType: hlsPlaylistContentType,
	}, nil
}

func (s *Server) EndSession(ctx context.Context, req *immichv1.EndSessionRequest) (*emptypb.Empty, error) {
	_, assetUUID, err := s.hlsAsset(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if err := validateHLSSession(assetUUID, req.GetSessionId()); err != nil {
		return nil, err
	}

	if err := os.RemoveAll(hlsSessionDir(req.GetSessionId())); err != nil {
		return nil, SanitizedInternal(ctx, "failed to remove HLS session", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetMediaPlaylist(ctx context.Context, req *immichv1.GetMediaPlaylistRequest) (*immichv1.HLSPlaylistResponse, error) {
	asset, assetUUID, err := s.hlsAsset(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if err := validateHLSSession(assetUUID, req.GetSessionId()); err != nil {
		return nil, err
	}
	if req.GetVariantIndex() != hlsVariantIndex {
		return nil, status.Errorf(codes.NotFound, "HLS variant not found")
	}
	if err := s.ensureHLS(ctx, asset, req.GetSessionId()); err != nil {
		return nil, err
	}

	playlistPath := filepath.Join(hlsVariantDir(req.GetSessionId(), req.GetVariantIndex()), "playlist.m3u8")
	data, err := os.ReadFile(playlistPath)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to read HLS media playlist", err)
	}

	return &immichv1.HLSPlaylistResponse{
		Data:        string(data),
		ContentType: hlsPlaylistContentType,
	}, nil
}

func (s *Server) GetSegment(ctx context.Context, req *immichv1.GetSegmentRequest) (*immichv1.HLSSegmentResponse, error) {
	path, err := s.hlsSegmentFilePath(ctx, req.GetId(), req.GetSessionId(), req.GetVariantIndex(), req.GetFilename())
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to read HLS segment", err)
	}
	return &immichv1.HLSSegmentResponse{
		Data:        data,
		ContentType: hlsSegmentContentType,
		Filename:    req.GetFilename(),
	}, nil
}

func (s *Server) handleHLSMainPlaylist(w http.ResponseWriter, r *http.Request, assetID string) {
	ctx, ok := s.hlsHTTPContext(w, r)
	if !ok {
		return
	}

	resp, err := s.GetMainPlaylist(ctx, &immichv1.GetMainPlaylistRequest{Id: assetID})
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}
	writeHLSPlaylist(w, resp.GetData())
}

func (s *Server) handleHLSEndSession(w http.ResponseWriter, r *http.Request, assetID, sessionID string) {
	ctx, ok := s.hlsHTTPContext(w, r)
	if !ok {
		return
	}

	_, err := s.EndSession(ctx, &immichv1.EndSessionRequest{Id: assetID, SessionId: sessionID})
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHLSMediaPlaylist(w http.ResponseWriter, r *http.Request, req hlsMediaPlaylistPath) {
	ctx, ok := s.hlsHTTPContext(w, r)
	if !ok {
		return
	}

	resp, err := s.GetMediaPlaylist(ctx, &immichv1.GetMediaPlaylistRequest{
		Id:           req.assetID,
		SessionId:    req.sessionID,
		VariantIndex: req.variantIndex,
	})
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}
	writeHLSPlaylist(w, resp.GetData())
}

func (s *Server) handleHLSSegment(w http.ResponseWriter, r *http.Request, req hlsSegmentPath) {
	ctx, ok := s.hlsHTTPContext(w, r)
	if !ok {
		return
	}

	segmentPath, err := s.hlsSegmentFilePath(ctx, req.assetID, req.sessionID, req.variantIndex, req.filename)
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}

	w.Header().Set("Content-Type", hlsSegmentContentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, segmentPath)
}

func (s *Server) hlsHTTPContext(w http.ResponseWriter, r *http.Request) (context.Context, bool) {
	claims, ok := s.requireAuth(w, r)
	if !ok {
		return nil, false
	}
	return auth.WithClaims(r.Context(), claims), true
}

func writeHLSPlaylist(w http.ResponseWriter, playlist string) {
	w.Header().Set("Content-Type", hlsPlaylistContentType)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(playlist))
}

func (s *Server) hlsAsset(ctx context.Context, assetID string) (sqlc.Asset, uuid.UUID, error) {
	parsedID, err := uuid.Parse(assetID)
	if err != nil {
		return sqlc.Asset{}, uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
	}

	assetUUID := pgtype.UUID{Bytes: parsedID, Valid: true}
	asset, err := s.db.GetAssetByID(ctx, assetUUID)
	if err != nil {
		return sqlc.Asset{}, uuid.Nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}
	if asset.Type != "VIDEO" {
		return sqlc.Asset{}, uuid.Nil, status.Error(codes.InvalidArgument, "asset is not a video")
	}

	return asset, parsedID, nil
}

func (s *Server) ensureHLS(ctx context.Context, asset sqlc.Asset, sessionID string) error {
	if s.config == nil || !s.config.Features.VideoTranscodingEnabled {
		return status.Error(codes.FailedPrecondition, "Real-time transcoding is not enabled")
	}

	hlsGenerationMu.Lock()
	defer hlsGenerationMu.Unlock()

	if err := cleanupStaleHLSSessions(); err != nil {
		logHLSCleanupError(err)
	}

	playlistPath := filepath.Join(hlsVariantDir(sessionID, hlsVariantIndex), "playlist.m3u8")
	if _, err := os.Stat(playlistPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return SanitizedInternal(ctx, "failed to inspect HLS playlist", err)
	}

	root := hlsRootDir()
	if err := os.MkdirAll(root, 0o750); err != nil {
		return SanitizedInternal(ctx, "failed to create HLS root directory", err)
	}

	stagingDir, err := os.MkdirTemp(root, sessionID+"-*")
	if err != nil {
		return SanitizedInternal(ctx, "failed to create HLS staging directory", err)
	}
	defer os.RemoveAll(stagingDir)

	inputPath, err := downloadAssetVideoToTemp(ctx, s.assetService.GetStorageService(), asset)
	if err != nil {
		return err
	}
	defer os.Remove(inputPath)

	metadata := hlsMetadata{Bandwidth: hlsDefaultBandwidth}
	if probe, probeErr := ffmpeg.ProbeVideo(ctx, inputPath); probeErr == nil && probe != nil {
		metadata.Width = probe.Width
		metadata.Height = probe.Height
		if probe.BitRate > 0 {
			metadata.Bandwidth = int(probe.BitRate)
		}
	}

	variantDir := filepath.Join(stagingDir, strconv.FormatUint(hlsVariantIndex, 10))
	opts := ffmpeg.DefaultHLSOptions()
	opts.SegmentDuration = hlsSegmentDuration
	if err := ffmpeg.GenerateHLS(ctx, inputPath, variantDir, opts); err != nil {
		if errors.Is(err, ffmpeg.ErrFFmpegNotFound) {
			return status.Error(codes.Unavailable, "ffmpeg/ffprobe not available")
		}
		return SanitizedInternal(ctx, "failed to generate HLS stream", err)
	}
	if err := writeHLSMetadata(stagingDir, metadata); err != nil {
		return SanitizedInternal(ctx, "failed to write HLS metadata", err)
	}

	sessionDir := hlsSessionDir(sessionID)
	if err := os.RemoveAll(sessionDir); err != nil {
		return SanitizedInternal(ctx, "failed to replace HLS session", err)
	}
	if err := os.Rename(stagingDir, sessionDir); err != nil {
		return SanitizedInternal(ctx, "failed to publish HLS session", err)
	}
	return nil
}

func (s *Server) hlsSegmentFilePath(ctx context.Context, assetID, sessionID string, variantIndex uint64, filename string) (string, error) {
	if !hlsSegmentFilenameRE.MatchString(filename) {
		return "", status.Error(codes.InvalidArgument, "invalid HLS segment filename")
	}

	asset, assetUUID, err := s.hlsAsset(ctx, assetID)
	if err != nil {
		return "", err
	}
	if err := validateHLSSession(assetUUID, sessionID); err != nil {
		return "", err
	}
	if variantIndex != hlsVariantIndex {
		return "", status.Error(codes.NotFound, "HLS variant not found")
	}
	if err := s.ensureHLS(ctx, asset, sessionID); err != nil {
		return "", err
	}

	path := filepath.Join(hlsVariantDir(sessionID, variantIndex), filename)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", status.Error(codes.NotFound, "HLS segment not found")
		}
		return "", SanitizedInternal(ctx, "failed to inspect HLS segment", err)
	}
	return path, nil
}

func downloadAssetVideoToTemp(ctx context.Context, storage interface {
	Download(context.Context, string) (io.ReadCloser, error)
}, asset sqlc.Asset,
) (string, error) {
	videoPath := asset.OriginalPath
	if asset.EncodedVideoPath.Valid && asset.EncodedVideoPath.String != "" {
		videoPath = asset.EncodedVideoPath.String
	}

	reader, err := storage.Download(ctx, videoPath)
	if err != nil {
		return "", SanitizedInternal(ctx, "failed to retrieve video", err)
	}
	defer reader.Close()

	ext := filepath.Ext(asset.OriginalFileName)
	if ext == "" {
		ext = ".mp4"
	}
	tmpFile, err := os.CreateTemp("", "immich-hls-input-*"+ext)
	if err != nil {
		return "", SanitizedInternal(ctx, "failed to create HLS input file", err)
	}
	tmpPath := tmpFile.Name()
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, reader); err != nil {
		_ = os.Remove(tmpPath)
		return "", SanitizedInternal(ctx, "failed to write HLS input file", err)
	}
	return tmpPath, nil
}

func validateHLSSession(assetID uuid.UUID, sessionID string) error {
	parsedSessionID, err := uuid.Parse(sessionID)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid HLS session ID: %v", err)
	}
	if parsedSessionID.String() != hlsSessionID(assetID) {
		return status.Error(codes.NotFound, "HLS session not found")
	}
	return nil
}

func hlsSessionID(assetID uuid.UUID) string {
	return uuid.NewSHA1(hlsSessionIDNamespace, []byte(assetID.String())).String()
}

func hlsRootDir() string {
	return filepath.Join(os.TempDir(), "immich-go-backend-hls")
}

func hlsSessionDir(sessionID string) string {
	return filepath.Join(hlsRootDir(), sessionID)
}

func hlsVariantDir(sessionID string, variantIndex uint64) string {
	return filepath.Join(hlsSessionDir(sessionID), strconv.FormatUint(variantIndex, 10))
}

func hlsMetadataPath(sessionID string) string {
	return filepath.Join(hlsSessionDir(sessionID), "metadata.json")
}

func readHLSMetadata(sessionID string) hlsMetadata {
	metadata := hlsMetadata{Width: hlsMaxWidth, Height: hlsMaxHeight, Bandwidth: hlsDefaultBandwidth}
	data, err := os.ReadFile(hlsMetadataPath(sessionID))
	if err != nil {
		return metadata
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return hlsMetadata{Width: hlsMaxWidth, Height: hlsMaxHeight, Bandwidth: hlsDefaultBandwidth}
	}
	if metadata.Width <= 0 {
		metadata.Width = hlsMaxWidth
	}
	if metadata.Height <= 0 {
		metadata.Height = hlsMaxHeight
	}
	if metadata.Bandwidth <= 0 {
		metadata.Bandwidth = hlsDefaultBandwidth
	}
	return metadata
}

func writeHLSMetadata(sessionDir string, metadata hlsMetadata) error {
	if metadata.Width <= 0 {
		metadata.Width = hlsMaxWidth
	}
	if metadata.Height <= 0 {
		metadata.Height = hlsMaxHeight
	}
	if metadata.Bandwidth <= 0 {
		metadata.Bandwidth = hlsDefaultBandwidth
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sessionDir, "metadata.json"), data, 0o600)
}

func buildHLSMainPlaylist(sessionID string, metadata hlsMetadata) string {
	width, height := hlsDisplaySize(metadata.Width, metadata.Height)
	lines := []string{
		"#EXTM3U",
		fmt.Sprintf("#EXT-X-VERSION:%d", hlsVersion),
		fmt.Sprintf(
			`#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS="avc1.64001f,mp4a.40.2",VIDEO-RANGE=SDR`,
			metadata.Bandwidth,
			width,
			height,
		),
		fmt.Sprintf("%s/%d/playlist.m3u8", sessionID, hlsVariantIndex),
		"",
	}
	return strings.Join(lines, "\n")
}

func hlsDisplaySize(width, height int) (int, int) {
	if width <= 0 || height <= 0 {
		return hlsMaxWidth, hlsMaxHeight
	}
	if width <= hlsMaxWidth && height <= hlsMaxHeight {
		return width, height
	}

	scaledWidth := hlsMaxWidth
	scaledHeight := height * hlsMaxWidth / width
	if scaledHeight > hlsMaxHeight {
		scaledHeight = hlsMaxHeight
		scaledWidth = width * hlsMaxHeight / height
	}
	return max(1, scaledWidth), max(1, scaledHeight)
}

func cleanupStaleHLSSessions() error {
	entries, err := os.ReadDir(hlsRootDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-hlsSessionTTL)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(hlsRootDir(), entry.Name()))
		}
	}
	return nil
}

func logHLSCleanupError(err error) {
	logrus.WithError(err).Warn("Failed to clean stale HLS sessions")
}

type hlsMediaPlaylistPath struct {
	assetID      string
	sessionID    string
	variantIndex uint64
}

type hlsSegmentPath struct {
	assetID      string
	sessionID    string
	variantIndex uint64
	filename     string
}

func hlsMainPlaylistAssetIDFromPath(path string) (string, bool) {
	assetID, ok := strings.CutPrefix(path, "/api/assets/")
	if !ok {
		return "", false
	}
	assetID, ok = strings.CutSuffix(assetID, "/video/stream/main.m3u8")
	if !ok || assetID == "" || strings.Contains(assetID, "/") {
		return "", false
	}
	return assetID, true
}

func hlsSessionFromPath(path string) (assetID string, sessionID string, ok bool) {
	parts, ok := hlsPathParts(path)
	if !ok || len(parts) != 4 {
		return "", "", false
	}
	return parts[0], parts[3], true
}

func hlsMediaPlaylistFromPath(path string) (hlsMediaPlaylistPath, bool) {
	parts, ok := hlsPathParts(path)
	if !ok || len(parts) != 6 || parts[5] != "playlist.m3u8" {
		return hlsMediaPlaylistPath{}, false
	}
	variantIndex, err := strconv.ParseUint(parts[4], 10, 64)
	if err != nil {
		return hlsMediaPlaylistPath{}, false
	}
	return hlsMediaPlaylistPath{
		assetID:      parts[0],
		sessionID:    parts[3],
		variantIndex: variantIndex,
	}, true
}

func hlsSegmentFromPath(path string) (hlsSegmentPath, bool) {
	parts, ok := hlsPathParts(path)
	if !ok || len(parts) != 6 || parts[5] == "playlist.m3u8" {
		return hlsSegmentPath{}, false
	}
	variantIndex, err := strconv.ParseUint(parts[4], 10, 64)
	if err != nil {
		return hlsSegmentPath{}, false
	}
	return hlsSegmentPath{
		assetID:      parts[0],
		sessionID:    parts[3],
		variantIndex: variantIndex,
		filename:     parts[5],
	}, true
}

func hlsPathParts(path string) ([]string, bool) {
	trimmed, ok := strings.CutPrefix(path, "/api/assets/")
	if !ok {
		return nil, false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) < 4 || parts[0] == "" || parts[1] != "video" || parts[2] != "stream" || parts[3] == "" {
		return nil, false
	}
	return parts, true
}
