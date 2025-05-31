package storage

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Service provides high-level storage operations
type Service struct {
	backend StorageBackend
	config  StorageConfig
}

// NewService creates a new storage service
func NewService(config StorageConfig) (*Service, error) {
	if err := ValidateStorageConfig(config); err != nil {
		return nil, err
	}

	backend, err := NewStorageBackend(config)
	if err != nil {
		return nil, err
	}

	return &Service{
		backend: backend,
		config:  config,
	}, nil
}

// UploadAsset uploads an asset file with validation and path generation
func (s *Service) UploadAsset(ctx context.Context, userID string, filename string, reader io.Reader, size int64) (*AssetUploadResult, error) {
	ctx, span := tracer.Start(ctx, "storage.UploadAsset",
		trace.WithAttributes(
			attribute.String("storage.user_id", userID),
			attribute.String("storage.filename", filename),
			attribute.Int64("storage.size", size),
		))
	defer span.End()

	// Validate file size
	if size > s.config.Upload.MaxFileSize {
		return nil, &StorageError{
			Op:      "upload asset",
			Path:    filename,
			Backend: s.config.Backend,
			Err:     fmt.Errorf("file size %d exceeds maximum allowed size %d", size, s.config.Upload.MaxFileSize),
		}
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if !s.isAllowedExtension(ext) {
		return nil, &StorageError{
			Op:      "upload asset",
			Path:    filename,
			Backend: s.config.Backend,
			Err:     fmt.Errorf("file extension %s is not allowed", ext),
		}
	}

	// Detect content type
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Validate MIME type
	if !s.isAllowedMimeType(contentType) {
		return nil, &StorageError{
			Op:      "upload asset",
			Path:    filename,
			Backend: s.config.Backend,
			Err:     fmt.Errorf("MIME type %s is not allowed", contentType),
		}
	}

	// Generate asset path
	assetPath := s.generateAssetPath(userID, filename)

	// Upload the file
	if err := s.backend.Upload(ctx, assetPath, reader, size, contentType); err != nil {
		span.RecordError(err)
		return nil, err
	}

	result := &AssetUploadResult{
		Path:        assetPath,
		Size:        size,
		ContentType: contentType,
		Filename:    filename,
		UserID:      userID,
		UploadedAt:  time.Now(),
	}

	// Generate URLs if supported
	if s.backend.SupportsPresignedURLs() {
		downloadURL, err := s.backend.GetPresignedDownloadURL(ctx, assetPath, 24*time.Hour)
		if err == nil {
			result.DownloadURL = downloadURL.URL
		}
	} else {
		publicURL, err := s.backend.GetPublicURL(ctx, assetPath)
		if err == nil {
			result.DownloadURL = publicURL
		}
	}

	return result, nil
}

// GetAssetUploadURL generates a pre-signed URL for direct asset upload (S3 only)
func (s *Service) GetAssetUploadURL(ctx context.Context, userID string, filename string, contentType string) (*AssetUploadURL, error) {
	ctx, span := tracer.Start(ctx, "storage.GetAssetUploadURL",
		trace.WithAttributes(
			attribute.String("storage.user_id", userID),
			attribute.String("storage.filename", filename),
			attribute.String("storage.content_type", contentType),
		))
	defer span.End()

	if !s.backend.SupportsPresignedURLs() {
		return nil, &StorageError{
			Op:      "get asset upload URL",
			Backend: s.config.Backend,
			Err:     fmt.Errorf("presigned URLs not supported by backend %s", s.config.Backend),
		}
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if !s.isAllowedExtension(ext) {
		return nil, &StorageError{
			Op:      "get asset upload URL",
			Path:    filename,
			Backend: s.config.Backend,
			Err:     fmt.Errorf("file extension %s is not allowed", ext),
		}
	}

	// Validate MIME type
	if !s.isAllowedMimeType(contentType) {
		return nil, &StorageError{
			Op:      "get asset upload URL",
			Path:    filename,
			Backend: s.config.Backend,
			Err:     fmt.Errorf("MIME type %s is not allowed", contentType),
		}
	}

	// Generate asset path
	assetPath := s.generateAssetPath(userID, filename)

	// Generate presigned upload URL
	presignedURL, err := s.backend.GetPresignedUploadURL(ctx, assetPath, contentType, 15*time.Minute)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	return &AssetUploadURL{
		UploadURL:   presignedURL.URL,
		Method:      presignedURL.Method,
		Headers:     presignedURL.Headers,
		ExpiresAt:   presignedURL.ExpiresAt,
		AssetPath:   assetPath,
		ContentType: contentType,
	}, nil
}

// GetAssetDownloadURL generates a download URL for an asset
func (s *Service) GetAssetDownloadURL(ctx context.Context, assetPath string, expiry time.Duration) (string, error) {
	ctx, span := tracer.Start(ctx, "storage.GetAssetDownloadURL",
		trace.WithAttributes(
			attribute.String("storage.asset_path", assetPath),
			attribute.String("storage.expiry", expiry.String()),
		))
	defer span.End()

	if s.backend.SupportsPresignedURLs() {
		presignedURL, err := s.backend.GetPresignedDownloadURL(ctx, assetPath, expiry)
		if err != nil {
			span.RecordError(err)
			return "", err
		}
		return presignedURL.URL, nil
	}

	// Fall back to public URL
	return s.backend.GetPublicURL(ctx, assetPath)
}

// DownloadAsset downloads an asset file
func (s *Service) DownloadAsset(ctx context.Context, assetPath string) (io.ReadCloser, *FileMetadata, error) {
	ctx, span := tracer.Start(ctx, "storage.DownloadAsset",
		trace.WithAttributes(attribute.String("storage.asset_path", assetPath)))
	defer span.End()

	// Get metadata first
	metadata, err := s.backend.GetMetadata(ctx, assetPath)
	if err != nil {
		span.RecordError(err)
		return nil, nil, err
	}

	// Download the file
	reader, err := s.backend.Download(ctx, assetPath)
	if err != nil {
		span.RecordError(err)
		return nil, nil, err
	}

	return reader, metadata, nil
}

// DeleteAsset deletes an asset file
func (s *Service) DeleteAsset(ctx context.Context, assetPath string) error {
	ctx, span := tracer.Start(ctx, "storage.DeleteAsset",
		trace.WithAttributes(attribute.String("storage.asset_path", assetPath)))
	defer span.End()

	return s.backend.Delete(ctx, assetPath)
}

// AssetExists checks if an asset exists
func (s *Service) AssetExists(ctx context.Context, assetPath string) (bool, error) {
	ctx, span := tracer.Start(ctx, "storage.AssetExists",
		trace.WithAttributes(attribute.String("storage.asset_path", assetPath)))
	defer span.End()

	return s.backend.Exists(ctx, assetPath)
}

// GetAssetMetadata returns metadata about an asset
func (s *Service) GetAssetMetadata(ctx context.Context, assetPath string) (*FileMetadata, error) {
	ctx, span := tracer.Start(ctx, "storage.GetAssetMetadata",
		trace.WithAttributes(attribute.String("storage.asset_path", assetPath)))
	defer span.End()

	return s.backend.GetMetadata(ctx, assetPath)
}

// ListAssets lists assets with optional prefix filtering
func (s *Service) ListAssets(ctx context.Context, userID string, prefix string, recursive bool) ([]FileInfo, error) {
	ctx, span := tracer.Start(ctx, "storage.ListAssets",
		trace.WithAttributes(
			attribute.String("storage.user_id", userID),
			attribute.String("storage.prefix", prefix),
			attribute.Bool("storage.recursive", recursive),
		))
	defer span.End()

	// Construct user-specific prefix
	userPrefix := fmt.Sprintf("users/%s", userID)
	if prefix != "" {
		userPrefix = fmt.Sprintf("%s/%s", userPrefix, prefix)
	}

	return s.backend.List(ctx, userPrefix, recursive)
}

// generateAssetPath generates a unique path for an asset
func (s *Service) generateAssetPath(userID string, filename string) string {
	// Generate a hash-based directory structure for better distribution
	hash := md5.Sum([]byte(userID + filename + time.Now().String()))
	hashStr := fmt.Sprintf("%x", hash)
	
	// Create a directory structure: users/{userID}/{year}/{month}/{day}/{hash[0:2]}/{hash[2:4]}/{filename}
	now := time.Now()
	year := now.Format("2006")
	month := now.Format("01")
	day := now.Format("02")
	
	return fmt.Sprintf("users/%s/%s/%s/%s/%s/%s/%s",
		userID, year, month, day, hashStr[0:2], hashStr[2:4], filename)
}

// isAllowedExtension checks if a file extension is allowed
func (s *Service) isAllowedExtension(ext string) bool {
	if len(s.config.Upload.AllowedExtensions) == 0 {
		return true // No restrictions
	}

	for _, allowed := range s.config.Upload.AllowedExtensions {
		if strings.EqualFold(ext, allowed) {
			return true
		}
	}

	return false
}

// isAllowedMimeType checks if a MIME type is allowed
func (s *Service) isAllowedMimeType(mimeType string) bool {
	if len(s.config.Upload.AllowedMimeTypes) == 0 {
		return true // No restrictions
	}

	for _, allowed := range s.config.Upload.AllowedMimeTypes {
		if strings.EqualFold(mimeType, allowed) {
			return true
		}
	}

	return false
}

// Close closes the storage service
func (s *Service) Close() error {
	return s.backend.Close()
}

// AssetUploadResult represents the result of an asset upload
type AssetUploadResult struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	Filename    string    `json:"filename"`
	UserID      string    `json:"user_id"`
	UploadedAt  time.Time `json:"uploaded_at"`
	DownloadURL string    `json:"download_url,omitempty"`
}

// AssetUploadURL represents a pre-signed upload URL
type AssetUploadURL struct {
	UploadURL   string            `json:"upload_url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers,omitempty"`
	ExpiresAt   time.Time         `json:"expires_at"`
	AssetPath   string            `json:"asset_path"`
	ContentType string            `json:"content_type"`
}

// GeneratePresignedUploadURL generates a presigned URL for uploading
func (s *Service) GeneratePresignedUploadURL(ctx context.Context, path string, contentType string, expiry time.Duration) (string, map[string]string, error) {
	ctx, span := tracer.Start(ctx, "storage.GeneratePresignedUploadURL",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.String("storage.content_type", contentType),
			attribute.String("storage.expiry", expiry.String()),
		))
	defer span.End()

	presignedURL, err := s.backend.GetPresignedUploadURL(ctx, path, contentType, expiry)
	if err != nil {
		return "", nil, err
	}
	
	return presignedURL.URL, presignedURL.Fields, nil
}

// Upload uploads data to the specified path
func (s *Service) Upload(ctx context.Context, path string, reader io.Reader, contentType string) error {
	ctx, span := tracer.Start(ctx, "storage.Upload",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.String("storage.content_type", contentType),
		))
	defer span.End()

	// For now, we'll use -1 to indicate unknown size
	// In a real implementation, we might want to buffer the reader to get the size
	return s.backend.Upload(ctx, path, reader, -1, contentType)
}

// Download downloads data from the specified path
func (s *Service) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	ctx, span := tracer.Start(ctx, "storage.Download",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	return s.backend.Download(ctx, path)
}

// UploadBytes uploads byte data to the specified path
func (s *Service) UploadBytes(ctx context.Context, path string, data []byte, contentType string) error {
	ctx, span := tracer.Start(ctx, "storage.UploadBytes",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.String("storage.content_type", contentType),
			attribute.Int("storage.size", len(data)),
		))
	defer span.End()

	return s.backend.UploadBytes(ctx, path, data, contentType)
}

// GeneratePresignedDownloadURL generates a presigned URL for downloading
func (s *Service) GeneratePresignedDownloadURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	ctx, span := tracer.Start(ctx, "storage.GeneratePresignedDownloadURL",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.String("storage.expiry", expiry.String()),
		))
	defer span.End()

	presignedURL, err := s.backend.GetPresignedDownloadURL(ctx, path, expiry)
	if err != nil {
		return "", err
	}
	
	return presignedURL.URL, nil
}