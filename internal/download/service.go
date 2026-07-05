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
	assets, err := s.resolveDownloadAssets(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	// Calculate total size
	var totalSize int64
	validAssetIDs := make([]string, 0, len(assets))

	for _, asset := range assets {
		totalSize += s.assetDownloadSize(ctx, asset)
		validAssetIDs = append(validAssetIDs, assetUUID(asset).String())
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
	asset, err := s.getAccessibleAsset(ctx, userID, assetID)
	if err != nil {
		return nil, "", err
	}

	// Get file from storage
	// Use original path or construct from ID
	filePath := assetStoragePath(asset)
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

	assets, err := s.resolveDownloadAssets(ctx, userID, req)
	if err != nil {
		return fmt.Errorf("failed to get download assets: %w", err)
	}

	// Track added files to avoid duplicates
	addedFiles := make(map[string]bool)

	// Add each asset to the archive
	for _, asset := range assets {
		assetID := assetUUID(asset)
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
		filePath := assetStoragePath(asset)
		reader, err := s.storageService.Download(ctx, filePath)
		if err != nil {
			s.logger.WithError(err).Warnf("Failed to get file for %s", assetID)
			continue
		}

		// Copy file to ZIP
		_, err = io.Copy(fileWriter, reader)
		if closeErr := reader.Close(); closeErr != nil {
			s.logger.WithError(closeErr).Warnf("Failed to close reader for %s", assetID)
		}
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

	if !s.userCanAccessAlbum(ctx, userID, album) {
		return fmt.Errorf("access denied")
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
	asset, err := s.getAccessibleAsset(ctx, userID, assetID)
	if err != nil {
		return nil, "", err
	}

	// Determine thumbnail path based on size
	thumbnailPath := fmt.Sprintf("thumbnails/%s/%s.webp", assetID.String(), size)

	// Get thumbnail from storage
	reader, err := s.storageService.Download(ctx, thumbnailPath)
	if err != nil {
		// Fallback to original if thumbnail doesn't exist
		filePath := assetStoragePath(asset)
		reader, err = s.storageService.Download(ctx, filePath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get thumbnail: %w", err)
		}
	}

	return reader, "image/webp", nil
}

// GetPresignedURL generates a presigned download URL for an asset
func (s *Service) GetPresignedURL(ctx context.Context, userID uuid.UUID, assetID uuid.UUID, expiry time.Duration) (string, error) {
	asset, err := s.getAccessibleAsset(ctx, userID, assetID)
	if err != nil {
		return "", err
	}

	// Get file path
	filePath := assetStoragePath(asset)

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
	return fmt.Sprintf("%s%s", assetUUID(*asset).String(), ext)
}

func (s *Service) resolveDownloadAssets(ctx context.Context, userID uuid.UUID, req *DownloadRequest) ([]sqlc.Asset, error) {
	if req.AlbumID != nil {
		return s.resolveAlbumDownloadAssets(ctx, userID, *req.AlbumID)
	}

	assets := make([]sqlc.Asset, 0, len(req.AssetIDs))
	for _, idStr := range req.AssetIDs {
		assetID, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		asset, err := s.getAccessibleAsset(ctx, userID, assetID)
		if err != nil {
			continue
		}
		assets = append(assets, asset)
	}
	return assets, nil
}

func (s *Service) resolveAlbumDownloadAssets(ctx context.Context, userID uuid.UUID, albumID string) ([]sqlc.Asset, error) {
	parsedAlbumID, err := uuid.Parse(albumID)
	if err != nil {
		return nil, fmt.Errorf("invalid album ID: %w", err)
	}

	albumAssets, err := s.db.GetAlbumAssets(ctx, pgUUID(parsedAlbumID))
	if err != nil {
		return nil, fmt.Errorf("failed to get album assets: %w", err)
	}

	assets := make([]sqlc.Asset, 0, len(albumAssets))
	for _, asset := range albumAssets {
		access, err := s.userCanAccessAsset(ctx, userID, asset)
		if err != nil || !access {
			continue
		}
		assets = append(assets, asset)
	}
	return assets, nil
}

func (s *Service) getAccessibleAsset(ctx context.Context, userID uuid.UUID, assetID uuid.UUID) (sqlc.Asset, error) {
	asset, err := s.db.GetAsset(ctx, pgUUID(assetID))
	if err != nil {
		return sqlc.Asset{}, fmt.Errorf("asset not found: %w", err)
	}

	access, err := s.userCanAccessAsset(ctx, userID, asset)
	if err != nil || !access {
		return sqlc.Asset{}, fmt.Errorf("access denied")
	}
	return asset, nil
}

func (s *Service) userCanAccessAsset(ctx context.Context, userID uuid.UUID, asset sqlc.Asset) (bool, error) {
	if asset.OwnerId.Valid && uuid.UUID(asset.OwnerId.Bytes) == userID {
		return true, nil
	}
	return s.checkSharedAccess(ctx, userID, assetUUID(asset))
}

func (s *Service) userCanAccessAlbum(ctx context.Context, userID uuid.UUID, album sqlc.Album) bool {
	if album.OwnerId.Valid && uuid.UUID(album.OwnerId.Bytes) == userID {
		return true
	}

	hasAccess, err := s.checkAlbumAccess(ctx, userID, uuidFromPG(album.ID))
	return err == nil && hasAccess
}

func (s *Service) assetDownloadSize(ctx context.Context, asset sqlc.Asset) int64 {
	exif, err := s.db.GetExifByAssetId(ctx, asset.ID)
	if err == nil && exif.FileSizeInByte.Valid {
		return exif.FileSizeInByte.Int64
	}
	return 1024 * 1024 // 1MB estimate when EXIF data is not available.
}

func assetStoragePath(asset sqlc.Asset) string {
	if asset.OriginalPath != "" {
		return asset.OriginalPath
	}
	return storage.AssetFallbackPath(assetUUID(asset), asset.OriginalFileName)
}

func assetUUID(asset sqlc.Asset) uuid.UUID {
	return uuidFromPG(asset.ID)
}

func uuidFromPG(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// checkSharedAccess checks if a user has shared access to an asset
func (s *Service) checkSharedAccess(ctx context.Context, userID uuid.UUID, assetID uuid.UUID) (bool, error) {
	// Check if asset is shared with user via any album
	isShared, err := s.db.CheckAssetSharedWithUser(ctx, sqlc.CheckAssetSharedWithUserParams{
		AssetsId: pgtype.UUID{Bytes: assetID, Valid: true},
		UsersId:  pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("failed to check asset access: %w", err)
	}

	return isShared, nil
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
