package storage

import (
	"context"
	"io"
	"time"
)

// StorageBackend defines the interface for different storage implementations
type StorageBackend interface {
	// Upload uploads a file to the storage backend
	Upload(ctx context.Context, path string, reader io.Reader, size int64, contentType string) error
	
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
	ExpiresAt time.Time         `json:"expires_at"`
}

// FileInfo represents basic information about a file
type FileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	IsDir        bool      `json:"is_dir"`
	ContentType  string    `json:"content_type,omitempty"`
	ETag         string    `json:"etag,omitempty"`
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

// StorageConfig represents configuration for storage backends
type StorageConfig struct {
	// Backend type: "local", "s3", "gcs", "azure", etc.
	Backend string `yaml:"backend" env:"STORAGE_BACKEND" default:"local"`
	
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
	RootPath string `yaml:"root_path" env:"STORAGE_LOCAL_ROOT" default:"./uploads"`
	
	// File permissions (octal)
	FileMode string `yaml:"file_mode" env:"STORAGE_LOCAL_FILE_MODE" default:"0644"`
	
	// Directory permissions (octal)
	DirMode string `yaml:"dir_mode" env:"STORAGE_LOCAL_DIR_MODE" default:"0755"`
}

// S3Config represents S3-compatible storage configuration
type S3Config struct {
	// Enable S3 storage
	Enabled bool `yaml:"enabled" env:"S3_ENABLED" default:"false"`
	
	// Enable direct upload to S3 via pre-signed URLs
	DirectUpload bool `yaml:"direct_upload" env:"S3_DIRECT_UPLOAD" default:"false"`
	
	// S3 endpoint (leave empty for AWS S3)
	Endpoint string `yaml:"endpoint" env:"S3_ENDPOINT"`
	
	// S3 region
	Region string `yaml:"region" env:"S3_REGION" default:"us-east-1"`
	
	// S3 bucket name
	Bucket string `yaml:"bucket" env:"S3_BUCKET"`
	
	// S3 access key ID
	AccessKeyID string `yaml:"access_key_id" env:"S3_ACCESS_KEY_ID"`
	
	// S3 secret access key
	SecretAccessKey string `yaml:"secret_access_key" env:"S3_SECRET_ACCESS_KEY"`
	
	// Use SSL/TLS
	UseSSL bool `yaml:"use_ssl" env:"S3_USE_SSL" default:"true"`
	
	// Path prefix for all objects
	PathPrefix string `yaml:"path_prefix" env:"S3_PATH_PREFIX"`
	
	// Force path style (for MinIO and other S3-compatible services)
	ForcePathStyle bool `yaml:"force_path_style" env:"S3_FORCE_PATH_STYLE" default:"false"`
	
	// Pre-signed URL expiry duration
	PresignedURLExpiry time.Duration `yaml:"presigned_url_expiry" env:"S3_PRESIGNED_URL_EXPIRY" default:"15m"`
}

// RcloneConfig represents rclone storage configuration
type RcloneConfig struct {
	// Rclone remote name (from rclone config)
	Remote string `yaml:"remote" env:"RCLONE_REMOTE"`
	
	// Path within the remote
	Path string `yaml:"path" env:"RCLONE_PATH" default:"/"`
	
	// Rclone config file path
	ConfigFile string `yaml:"config_file" env:"RCLONE_CONFIG_FILE"`
	
	// Additional rclone flags
	Flags []string `yaml:"flags" env:"RCLONE_FLAGS"`
	
	// Connection timeout
	Timeout time.Duration `yaml:"timeout" env:"RCLONE_TIMEOUT" default:"30s"`
}

// UploadConfig represents upload-specific configuration
type UploadConfig struct {
	// Maximum file size in bytes
	MaxFileSize int64 `yaml:"max_file_size" env:"UPLOAD_MAX_FILE_SIZE" default:"104857600"` // 100MB
	
	// Allowed file extensions
	AllowedExtensions []string `yaml:"allowed_extensions" env:"UPLOAD_ALLOWED_EXTENSIONS"`
	
	// Allowed MIME types
	AllowedMimeTypes []string `yaml:"allowed_mime_types" env:"UPLOAD_ALLOWED_MIME_TYPES"`
	
	// Enable virus scanning
	VirusScanEnabled bool `yaml:"virus_scan_enabled" env:"UPLOAD_VIRUS_SCAN_ENABLED" default:"false"`
	
	// Temporary upload directory
	TempDir string `yaml:"temp_dir" env:"UPLOAD_TEMP_DIR" default:"/tmp/immich-uploads"`
	
	// Cleanup temporary files after this duration
	TempFileCleanup time.Duration `yaml:"temp_file_cleanup" env:"UPLOAD_TEMP_FILE_CLEANUP" default:"1h"`
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
	ErrFileNotFound      = &StorageError{Op: "file not found", Err: io.EOF}
	ErrFileAlreadyExists = &StorageError{Op: "file already exists"}
	ErrInvalidPath       = &StorageError{Op: "invalid path"}
	ErrPermissionDenied  = &StorageError{Op: "permission denied"}
	ErrQuotaExceeded     = &StorageError{Op: "quota exceeded"}
	ErrBackendNotSupported = &StorageError{Op: "backend not supported"}
)