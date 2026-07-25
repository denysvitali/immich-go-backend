package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// StorageBackend defines the interface for different storage implementations
type StorageBackend interface {
	// Upload uploads a file to the storage backend
	Upload(ctx context.Context, path string, reader io.Reader, size int64, contentType string) error

	// UploadBytes uploads byte data to the storage backend
	UploadBytes(ctx context.Context, path string, data []byte, contentType string) error

	// Download downloads a file from the storage backend
	Download(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete deletes a file from the storage backend
	Delete(ctx context.Context, path string) error

	// Exists checks if a file exists in the storage backend
	Exists(ctx context.Context, path string) (bool, error)

	// GetSize returns the size of a file in bytes
	GetSize(ctx context.Context, path string) (int64, error)

	// GetPresignedUploadURL generates a pre-signed URL for uploading (S3 only)
	GetPresignedUploadURL(ctx context.Context, path string, contentType string, expiry time.Duration) (*PresignedURL, error)

	// GetPresignedDownloadURL generates a pre-signed URL for downloading (S3 only)
	GetPresignedDownloadURL(ctx context.Context, path string, expiry time.Duration) (*PresignedURL, error)

	// SupportsPresignedURLs returns true if the backend supports pre-signed URLs
	SupportsPresignedURLs() bool

	// GetPublicURL returns a public URL for accessing the file (if supported)
	GetPublicURL(ctx context.Context, path string) (string, error)

	// Copy copies a file from one path to another within the same backend
	Copy(ctx context.Context, srcPath, dstPath string) error

	// Move moves a file from one path to another within the same backend
	Move(ctx context.Context, srcPath, dstPath string) error

	// List lists files in a directory with optional prefix filtering
	List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error)

	// GetMetadata returns metadata about a file
	GetMetadata(ctx context.Context, path string) (*FileMetadata, error)

	// Close closes the storage backend and cleans up resources
	Close() error
}

// PresignedURL represents a pre-signed URL for upload or download
type PresignedURL struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"` // For S3 form uploads
	ExpiresAt time.Time         `json:"expires_at"`
}

// FileInfo represents basic information about a file
type FileInfo struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	IsDir       bool      `json:"is_dir"`
	ContentType string    `json:"content_type,omitempty"`
	ETag        string    `json:"etag,omitempty"`
}

// FileMetadata represents detailed metadata about a file
type FileMetadata struct {
	Path        string            `json:"path"`
	Size        int64             `json:"size"`
	ModTime     time.Time         `json:"mod_time"`
	ContentType string            `json:"content_type"`
	ETag        string            `json:"etag,omitempty"`
	Checksum    string            `json:"checksum,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// StorageConfig selects and configures a storage backend.
//
// It is populated from the `storage` section of the YAML document. Only
// STORAGE_BACKEND, STORAGE_LOCAL_ROOT and UPLOAD_TEMP_DIR can be overridden
// from the environment (see config.loadFromEnv); everything else, S3
// credentials included, must come from YAML.
type StorageConfig struct {
	// Backend type: "local", "s3", "gcs", "azure", etc.
	Backend string `yaml:"backend"`

	// Local storage configuration
	Local LocalConfig `yaml:"local,omitempty"`

	// S3 configuration
	S3 S3Config `yaml:"s3,omitempty"`

	// Rclone configuration
	Rclone RcloneConfig `yaml:"rclone,omitempty"`

	// Upload configuration
	Upload UploadConfig `yaml:"upload"`
}

// LocalConfig represents local filesystem storage configuration
type LocalConfig struct {
	// Root directory for storing files
	RootPath string `yaml:"root_path"`

	// File permissions (octal)
	FileMode string `yaml:"file_mode"`

	// Directory permissions (octal)
	DirMode string `yaml:"dir_mode"`
}

// S3Config represents S3-compatible storage configuration
type S3Config struct {
	// Enable S3 storage
	Enabled bool `yaml:"enabled"`

	// Enable direct upload to S3 via pre-signed URLs
	DirectUpload bool `yaml:"direct_upload"`

	// S3 endpoint (leave empty for AWS S3)
	Endpoint string `yaml:"endpoint"`

	// S3 region
	Region string `yaml:"region"`

	// S3 bucket name
	Bucket string `yaml:"bucket"`

	// S3 access key ID
	AccessKeyID string `yaml:"access_key_id"`

	// S3 secret access key
	SecretAccessKey string `yaml:"secret_access_key"`

	// Use SSL/TLS
	UseSSL bool `yaml:"use_ssl"`

	// Path prefix for all objects
	PathPrefix string `yaml:"path_prefix"`

	// Force path style (for MinIO and other S3-compatible services)
	ForcePathStyle bool `yaml:"force_path_style"`

	// Pre-signed URL expiry duration
	PresignedURLExpiry time.Duration `yaml:"presigned_url_expiry"`
}

// RcloneConfig represents rclone storage configuration
type RcloneConfig struct {
	// Rclone remote name (from rclone config)
	Remote string `yaml:"remote"`

	// Path within the remote
	Path string `yaml:"path"`

	// Rclone config file path
	ConfigFile string `yaml:"config_file"`

	// Additional rclone flags
	Flags []string `yaml:"flags"`

	// Connection timeout
	Timeout time.Duration `yaml:"timeout"`
}

// UploadConfig represents upload-specific configuration
type UploadConfig struct {
	// Maximum file size in bytes
	MaxFileSize int64 `yaml:"max_file_size"` // 100MB

	// Allowed file extensions
	AllowedExtensions []string `yaml:"allowed_extensions"`

	// Allowed MIME types
	AllowedMimeTypes []string `yaml:"allowed_mime_types"`

	// Enable virus scanning
	VirusScanEnabled bool `yaml:"virus_scan_enabled"`

	// Temporary upload directory
	TempDir string `yaml:"temp_dir"`

	// Cleanup temporary files after this duration
	TempFileCleanup time.Duration `yaml:"temp_file_cleanup"`
}

// StorageError represents a storage-specific error
type StorageError struct {
	Op      string // Operation that failed
	Path    string // File path involved
	Backend string // Storage backend name
	Err     error  // Underlying error
}

func (e *StorageError) Error() string {
	if e.Path != "" {
		return e.Op + " " + e.Path + " (" + e.Backend + "): " + e.Err.Error()
	}
	return e.Op + " (" + e.Backend + "): " + e.Err.Error()
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

// Common storage errors
var (
	ErrFileNotFound        = &StorageError{Op: "file not found", Err: io.EOF}
	ErrFileAlreadyExists   = &StorageError{Op: "file already exists"}
	ErrInvalidPath         = &StorageError{Op: "invalid path"}
	ErrPermissionDenied    = &StorageError{Op: "permission denied"}
	ErrQuotaExceeded       = &StorageError{Op: "quota exceeded"}
	ErrBackendNotSupported = &StorageError{Op: "backend not supported"}
)

// Sentinel errors for unsupported operations
var (
	ErrPresignedURLsNotSupported = &StorageError{Op: "presigned URL", Err: errors.New("presigned URLs not supported by backend")}
	ErrPublicURLNotSupported     = &StorageError{Op: "public URL", Err: errors.New("public URLs not supported by backend")}
)
