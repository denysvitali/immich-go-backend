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
	TotalSize   int64    `json:"totalSize"`
	ArchiveSize int64    `json:"archiveSize"`
	AssetCount  int      `json:"assetCount"`
	AssetIDs    []string `json:"assetIds"`
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
		albumAssets, err := s.db.GetAlbumAssets(ctx, pgtype.UUID{Bytes: albumID, Valid: true})
		if err != nil {
			return nil, fmt.Errorf("failed to get album assets: %w", err)
		}

		for _, asset := range albumAssets {
			// Convert pgtype.UUID to uuid.UUID
			if asset.ID.Valid {
				assetIDs = append(assetIDs, asset.ID.Bytes)
			}
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
		asset, err := s.db.GetAsset(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
		if err != nil {
			continue
		}

		// Verify user has access to the asset
		if asset.OwnerId.Bytes != userID {
			// Check if user has shared access
			hasAccess, err := s.checkSharedAccess(ctx, userID, assetID)
			if err != nil || !hasAccess {
				continue
			}
		}

		// For now, estimate file size (TODO: add FileSize field to Asset)
		totalSize += 1024 * 1024 // 1MB estimate per file
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
	asset, err := s.db.GetAsset(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
	if err != nil {
		return nil, "", fmt.Errorf("asset not found: %w", err)
	}

	// Verify user has access
	if asset.OwnerId.Bytes != userID {
		hasAccess, err := s.checkSharedAccess(ctx, userID, assetID)
		if err != nil || !hasAccess {
			return nil, "", fmt.Errorf("access denied")
		}
	}

	// Get file from storage
	// Use original path or construct from ID
	filePath := asset.OriginalPath
	if filePath == "" {
		filePath = fmt.Sprintf("%s/%s", assetID.String(), asset.OriginalFileName)
	}
	reader, err := s.storageService.Download(ctx, filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file: %w", err)
	}

	// Get filename
	filename := asset.OriginalFileName
	if filename == "" {
		filename = fmt.Sprintf("%s.jpg", assetID.String()) // Default extension
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
		asset, err := s.db.GetAsset(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
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
		filePath := asset.OriginalPath
		if filePath == "" {
			filePath = fmt.Sprintf("%s/%s", assetID.String(), asset.OriginalFileName)
		}
		reader, err := s.storageService.Download(ctx, filePath)
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
	album, err := s.db.GetAlbum(ctx, pgtype.UUID{Bytes: albumID, Valid: true})
	if err != nil {
		return fmt.Errorf("album not found: %w", err)
	}

	// Check if user owns or has access to the album
	if album.OwnerId.Bytes != userID {
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

// GetThumbnail retrieves a thumbnail for an asset
func (s *Service) GetThumbnail(ctx context.Context, userID uuid.UUID, assetID uuid.UUID, size string) (io.ReadCloser, string, error) {
	// Get asset to verify access
	asset, err := s.db.GetAsset(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
	if err != nil {
		return nil, "", fmt.Errorf("asset not found: %w", err)
	}

	// Verify user has access
	if asset.OwnerId.Bytes != userID {
		hasAccess, err := s.checkSharedAccess(ctx, userID, assetID)
		if err != nil || !hasAccess {
			return nil, "", fmt.Errorf("access denied")
		}
	}

	// Determine thumbnail path based on size
	thumbnailPath := fmt.Sprintf("thumbnails/%s/%s.webp", assetID.String(), size)

	// Get thumbnail from storage
	reader, err := s.storageService.Download(ctx, thumbnailPath)
	if err != nil {
		// Fallback to original if thumbnail doesn't exist
		filePath := asset.OriginalPath
		if filePath == "" {
			filePath = fmt.Sprintf("%s/%s", assetID.String(), asset.OriginalFileName)
		}
		reader, err = s.storageService.Download(ctx, filePath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get thumbnail: %w", err)
		}
	}

	return reader, "image/webp", nil
}

// GetPresignedURL generates a presigned download URL for an asset
func (s *Service) GetPresignedURL(ctx context.Context, userID uuid.UUID, assetID uuid.UUID, expiry time.Duration) (string, error) {
	// Get asset to verify access
	asset, err := s.db.GetAsset(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
	if err != nil {
		return "", fmt.Errorf("asset not found: %w", err)
	}

	// Verify user has access
	if asset.OwnerId.Bytes != userID {
		hasAccess, err := s.checkSharedAccess(ctx, userID, assetID)
		if err != nil || !hasAccess {
			return "", fmt.Errorf("access denied")
		}
	}

	// Get file path
	filePath := asset.OriginalPath
	if filePath == "" {
		filePath = fmt.Sprintf("%s/%s", assetID.String(), asset.OriginalFileName)
	}

	// Generate presigned URL
	url, err := s.storageService.GeneratePresignedDownloadURL(ctx, filePath, expiry)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url, nil
}

// generateArchivePath generates a path for an asset in an archive
func (s *Service) generateArchivePath(asset *sqlc.Asset) string {
	// Use original filename if available
	if asset.OriginalFileName != "" {
		// Add date prefix for organization
		date := asset.FileCreatedAt.Time
		if date.IsZero() {
			date = asset.CreatedAt.Time
		}
		return path.Join(
			date.Format("2006"),
			date.Format("01-January"),
			asset.OriginalFileName,
		)
	}

	// Fallback to ID-based name
	ext := ".jpg" // Default extension
	if asset.Type == "VIDEO" {
		ext = ".mp4"
	}
	return fmt.Sprintf("%s%s", asset.ID.Bytes, ext)
}

// checkSharedAccess checks if a user has shared access to an asset
func (s *Service) checkSharedAccess(ctx context.Context, userID uuid.UUID, assetID uuid.UUID) (bool, error) {
	// For now, we'll check by iterating through user's shared albums
	// TODO: Add a more efficient query for checking asset access

	// Check if asset is shared via shared link
	// TODO: Implement shared link checking once SharedLinks service is fixed

	return false, nil
}

// checkAlbumAccess checks if a user has access to an album
func (s *Service) checkAlbumAccess(ctx context.Context, userID uuid.UUID, albumID uuid.UUID) (bool, error) {
	// Check if user is shared with the album
	users, err := s.db.GetAlbumSharedUsers(ctx, pgtype.UUID{Bytes: albumID, Valid: true})
	if err != nil {
		return false, err
	}

	for _, userRow := range users {
		if userRow.ID.Valid && userRow.ID.Bytes == userID {
			return true, nil
		}
	}

	return false, nil
}
