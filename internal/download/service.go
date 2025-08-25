package download

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/storage"
)

// Service handles download operations
type Service struct {
	db             *sqlc.Queries
	storageService *storage.Service
	logger         *logrus.Logger
}

// NewService creates a new download service
func NewService(db *sqlc.Queries, storageService *storage.Service) *Service {
	return &Service{
		db:             db,
		storageService: storageService,
		logger:         logrus.StandardLogger(),
	}
}

// DownloadInfo represents download information
type DownloadInfo struct {
	TotalSize    int64    `json:"totalSize"`
	ArchiveSize  int64    `json:"archiveSize"`
	AssetCount   int      `json:"assetCount"`
	AssetIDs     []string `json:"assetIds"`
}

// DownloadRequest represents a download request
type DownloadRequest struct {
	AssetIDs []string `json:"assetIds"`
	AlbumID  *string  `json:"albumId,omitempty"`
}

// GetDownloadInfo retrieves information about a potential download
func (s *Service) GetDownloadInfo(ctx context.Context, userID uuid.UUID, req *DownloadRequest) (*DownloadInfo, error) {
	var assetIDs []uuid.UUID
	
	// If album ID is provided, get all assets from the album
	if req.AlbumID != nil {
		albumID, err := uuid.Parse(*req.AlbumID)
		if err != nil {
			return nil, fmt.Errorf("invalid album ID: %w", err)
		}

		// Get album assets
		albumAssets, err := s.db.GetAlbumAssets(ctx, sqlc.GetAlbumAssetsParams{
			AlbumID: albumID,
			UserID:  pgtype.UUID{Bytes: userID, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get album assets: %w", err)
		}

		for _, asset := range albumAssets {
			assetIDs = append(assetIDs, asset.ID)
		}
	} else {
		// Parse provided asset IDs
		for _, idStr := range req.AssetIDs {
			id, err := uuid.Parse(idStr)
			if err != nil {
				continue
			}
			assetIDs = append(assetIDs, id)
		}
	}

	// Calculate total size
	var totalSize int64
	validAssetIDs := make([]string, 0, len(assetIDs))

	for _, assetID := range assetIDs {
		asset, err := s.db.GetAsset(ctx, assetID)
		if err != nil {
			continue
		}

		// Verify user has access to the asset
		if asset.OwnerID.Bytes != userID {
			// Check if user has shared access
			hasAccess, err := s.checkSharedAccess(ctx, userID, assetID)
			if err != nil || !hasAccess {
				continue
			}
		}

		totalSize += asset.FileSize
		validAssetIDs = append(validAssetIDs, assetID.String())
	}

	// Estimate archive size (typically 90-95% of original for already compressed media)
	archiveSize := int64(float64(totalSize) * 0.95)

	return &DownloadInfo{
		TotalSize:   totalSize,
		ArchiveSize: archiveSize,
		AssetCount:  len(validAssetIDs),
		AssetIDs:    validAssetIDs,
	}, nil
}

// DownloadAsset downloads a single asset
func (s *Service) DownloadAsset(ctx context.Context, userID uuid.UUID, assetID uuid.UUID) (io.ReadCloser, string, error) {
	// Get asset from database
	asset, err := s.db.GetAsset(ctx, assetID)
	if err != nil {
		return nil, "", fmt.Errorf("asset not found: %w", err)
	}

	// Verify user has access
	if asset.OwnerID.Bytes != userID {
		hasAccess, err := s.checkSharedAccess(ctx, userID, assetID)
		if err != nil || !hasAccess {
			return nil, "", fmt.Errorf("access denied")
		}
	}

	// Get file from storage
	reader, err := s.storageService.GetFile(ctx, asset.FilePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file: %w", err)
	}

	// Get filename
	filename := filepath.Base(asset.OriginalPath)
	if filename == "" {
		filename = fmt.Sprintf("%s%s", assetID.String(), filepath.Ext(asset.FilePath))
	}

	return reader, filename, nil
}

// DownloadArchive creates and streams a ZIP archive of multiple assets
func (s *Service) DownloadArchive(ctx context.Context, userID uuid.UUID, req *DownloadRequest, writer io.Writer) error {
	// Create ZIP writer
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	// Get download info to validate assets
	info, err := s.GetDownloadInfo(ctx, userID, req)
	if err != nil {
		return fmt.Errorf("failed to get download info: %w", err)
	}

	// Track added files to avoid duplicates
	addedFiles := make(map[string]bool)

	// Add each asset to the archive
	for _, assetIDStr := range info.AssetIDs {
		assetID, err := uuid.Parse(assetIDStr)
		if err != nil {
			continue
		}

		// Get asset
		asset, err := s.db.GetAsset(ctx, assetID)
		if err != nil {
			s.logger.WithError(err).Warnf("Failed to get asset %s", assetID)
			continue
		}

		// Generate archive path
		archivePath := s.generateArchivePath(&asset)
		
		// Ensure unique filename in archive
		basePath := archivePath
		counter := 1
		for addedFiles[archivePath] {
			ext := filepath.Ext(basePath)
			name := basePath[:len(basePath)-len(ext)]
			archivePath = fmt.Sprintf("%s_%d%s", name, counter, ext)
			counter++
		}
		addedFiles[archivePath] = true

		// Create file in ZIP
		fileHeader := &zip.FileHeader{
			Name:     archivePath,
			Method:   zip.Deflate,
			Modified: asset.CreatedAt.Time,
		}

		fileWriter, err := zipWriter.CreateHeader(fileHeader)
		if err != nil {
			s.logger.WithError(err).Warnf("Failed to create ZIP entry for %s", assetID)
			continue
		}

		// Get file from storage
		reader, err := s.storageService.GetFile(ctx, asset.FilePath)
		if err != nil {
			s.logger.WithError(err).Warnf("Failed to get file for %s", assetID)
			continue
		}

		// Copy file to ZIP
		_, err = io.Copy(fileWriter, reader)
		reader.Close()
		if err != nil {
			s.logger.WithError(err).Warnf("Failed to write file %s to ZIP", assetID)
			continue
		}
	}

	return nil
}

// DownloadAlbum downloads all assets from an album as a ZIP archive
func (s *Service) DownloadAlbum(ctx context.Context, userID uuid.UUID, albumID uuid.UUID, writer io.Writer) error {
	// Get album to verify access
	album, err := s.db.GetAlbum(ctx, albumID)
	if err != nil {
		return fmt.Errorf("album not found: %w", err)
	}

	// Check if user owns or has access to the album
	if album.OwnerID.Bytes != userID {
		// Check if user is shared with
		hasAccess, err := s.checkAlbumAccess(ctx, userID, albumID)
		if err != nil || !hasAccess {
			return fmt.Errorf("access denied")
		}
	}

	// Create download request for album
	albumIDStr := albumID.String()
	req := &DownloadRequest{
		AlbumID: &albumIDStr,
	}

	return s.DownloadArchive(ctx, userID, req, writer)
}

// StreamAsset streams an asset for playback
func (s *Service) StreamAsset(ctx context.Context, userID uuid.UUID, assetID uuid.UUID, rangeHeader string) (*StreamResponse, error) {
	// Get asset from database
	asset, err := s.db.GetAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}

	// Verify user has access
	if asset.OwnerID.Bytes != userID {
		hasAccess, err := s.checkSharedAccess(ctx, userID, assetID)
		if err != nil || !hasAccess {
			return nil, fmt.Errorf("access denied")
		}
	}

	// Parse range header if present
	var start, end int64
	if rangeHeader != "" {
		// Parse Range header (e.g., "bytes=0-1023")
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
		if err != nil {
			// Try parsing just start (e.g., "bytes=1024-")
			_, err = fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
			if err == nil {
				end = asset.FileSize - 1
			}
		}
	} else {
		end = asset.FileSize - 1
	}

	// Get file from storage with range
	reader, err := s.storageService.GetFileRange(ctx, asset.FilePath, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return &StreamResponse{
		Reader:      reader,
		ContentType: asset.MimeType,
		Size:        asset.FileSize,
		Start:       start,
		End:         end,
	}, nil
}

// StreamResponse represents a streaming response
type StreamResponse struct {
	Reader      io.ReadCloser
	ContentType string
	Size        int64
	Start       int64
	End         int64
}

// generateArchivePath generates a path for an asset in the archive
func (s *Service) generateArchivePath(asset *sqlc.Asset) string {
	// Use creation date to organize files
	t := asset.CreatedAt.Time
	year := t.Year()
	month := t.Month()
	day := t.Day()

	// Get original filename or generate one
	filename := filepath.Base(asset.OriginalPath)
	if filename == "" || filename == "." {
		filename = fmt.Sprintf("%s%s", asset.ID.String(), filepath.Ext(asset.FilePath))
	}

	// Create path: YYYY/MM/DD/filename
	return path.Join(
		fmt.Sprintf("%04d", year),
		fmt.Sprintf("%02d", month),
		fmt.Sprintf("%02d", day),
		filename,
	)
}

// checkSharedAccess checks if a user has shared access to an asset
func (s *Service) checkSharedAccess(ctx context.Context, userID uuid.UUID, assetID uuid.UUID) (bool, error) {
	// Check album sharing
	albums, err := s.db.GetAssetAlbums(ctx, assetID)
	if err != nil {
		return false, err
	}

	for _, album := range albums {
		// Check if user has access to this album
		hasAccess, err := s.checkAlbumAccess(ctx, userID, album.ID)
		if err != nil {
			continue
		}
		if hasAccess {
			return true, nil
		}
	}

	// Check direct asset sharing via shared links
	// This would require additional queries
	
	return false, nil
}

// checkAlbumAccess checks if a user has access to an album
func (s *Service) checkAlbumAccess(ctx context.Context, userID uuid.UUID, albumID uuid.UUID) (bool, error) {
	// Check if user is explicitly shared with the album
	users, err := s.db.GetAlbumUsers(ctx, albumID)
	if err != nil {
		return false, err
	}

	for _, user := range users {
		if user.UserID.Valid && user.UserID.Bytes == userID {
			return true, nil
		}
	}

	return false, nil
}

// GetThumbnail retrieves a thumbnail for an asset
func (s *Service) GetThumbnail(ctx context.Context, userID uuid.UUID, assetID uuid.UUID, size string) (io.ReadCloser, string, error) {
	// Get asset from database
	asset, err := s.db.GetAsset(ctx, assetID)
	if err != nil {
		return nil, "", fmt.Errorf("asset not found: %w", err)
	}

	// Verify user has access
	if asset.OwnerID.Bytes != userID {
		hasAccess, err := s.checkSharedAccess(ctx, userID, assetID)
		if err != nil || !hasAccess {
			return nil, "", fmt.Errorf("access denied")
		}
	}

	// Determine thumbnail path based on size
	var thumbnailPath string
	switch size {
	case "thumbnail":
		thumbnailPath = asset.ThumbnailPath
	case "preview":
		thumbnailPath = asset.PreviewPath
	case "thumbnail_big":
		thumbnailPath = asset.ThumbnailBigPath
	default:
		return nil, "", fmt.Errorf("invalid thumbnail size")
	}

	if thumbnailPath == "" {
		return nil, "", fmt.Errorf("thumbnail not available")
	}

	// Get thumbnail from storage
	reader, err := s.storageService.GetFile(ctx, thumbnailPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get thumbnail: %w", err)
	}

	// Determine content type
	contentType := "image/jpeg"
	if filepath.Ext(thumbnailPath) == ".webp" {
		contentType = "image/webp"
	}

	return reader, contentType, nil
}

// GeneratePresignedURL generates a presigned URL for direct asset download
func (s *Service) GeneratePresignedURL(ctx context.Context, userID uuid.UUID, assetID uuid.UUID, duration time.Duration) (string, error) {
	// Get asset from database
	asset, err := s.db.GetAsset(ctx, assetID)
	if err != nil {
		return "", fmt.Errorf("asset not found: %w", err)
	}

	// Verify user has access
	if asset.OwnerID.Bytes != userID {
		hasAccess, err := s.checkSharedAccess(ctx, userID, assetID)
		if err != nil || !hasAccess {
			return "", fmt.Errorf("access denied")
		}
	}

	// Generate presigned URL
	url, err := s.storageService.GetPresignedURL(ctx, asset.FilePath, duration)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url, nil
}