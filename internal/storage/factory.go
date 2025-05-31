package storage

import (
	"fmt"
	"strings"
)

// NewStorageBackend creates a new storage backend based on the configuration
func NewStorageBackend(config StorageConfig) (StorageBackend, error) {
	backend := strings.ToLower(config.Backend)
	
	switch backend {
	case "local", "filesystem", "fs":
		return NewLocalBackend(config.Local)
		
	case "s3", "aws":
		return NewS3Backend(config.S3)
		
	case "rclone":
		return NewRcloneBackend(config.Rclone)
		
	default:
		return nil, &StorageError{
			Op:      "create storage backend",
			Backend: backend,
			Err:     fmt.Errorf("unsupported storage backend: %s", backend),
		}
	}
}

// ValidateStorageConfig validates the storage configuration
func ValidateStorageConfig(config StorageConfig) error {
	backend := strings.ToLower(config.Backend)
	
	switch backend {
	case "local", "filesystem", "fs":
		return validateLocalConfig(config.Local)
		
	case "s3", "aws":
		return validateS3Config(config.S3)
		
	case "rclone":
		return validateRcloneConfig(config.Rclone)
		
	default:
		return &StorageError{
			Op:      "validate storage config",
			Backend: backend,
			Err:     fmt.Errorf("unsupported storage backend: %s", backend),
		}
	}
}

// validateLocalConfig validates local storage configuration
func validateLocalConfig(config LocalConfig) error {
	if config.RootPath == "" {
		return &StorageError{
			Op:      "validate local config",
			Backend: "local",
			Err:     fmt.Errorf("root_path is required"),
		}
	}
	
	return nil
}

// validateS3Config validates S3 storage configuration
func validateS3Config(config S3Config) error {
	if config.Bucket == "" {
		return &StorageError{
			Op:      "validate s3 config",
			Backend: "s3",
			Err:     fmt.Errorf("bucket is required"),
		}
	}
	
	if config.AccessKeyID == "" {
		return &StorageError{
			Op:      "validate s3 config",
			Backend: "s3",
			Err:     fmt.Errorf("access_key_id is required"),
		}
	}
	
	if config.SecretAccessKey == "" {
		return &StorageError{
			Op:      "validate s3 config",
			Backend: "s3",
			Err:     fmt.Errorf("secret_access_key is required"),
		}
	}
	
	if config.Region == "" {
		return &StorageError{
			Op:      "validate s3 config",
			Backend: "s3",
			Err:     fmt.Errorf("region is required"),
		}
	}
	
	return nil
}

// validateRcloneConfig validates rclone storage configuration
func validateRcloneConfig(config RcloneConfig) error {
	if config.Remote == "" {
		return &StorageError{
			Op:      "validate rclone config",
			Backend: "rclone",
			Err:     fmt.Errorf("remote is required"),
		}
	}
	
	return nil
}

// GetDefaultStorageConfig returns a default storage configuration
func GetDefaultStorageConfig() StorageConfig {
	return StorageConfig{
		Backend: "local",
		Local: LocalConfig{
			RootPath: "./uploads",
			FileMode: "0644",
			DirMode:  "0755",
		},
		Upload: UploadConfig{
			MaxFileSize: 104857600, // 100MB
			AllowedExtensions: []string{
				".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp",
				".mp4", ".mov", ".avi", ".mkv", ".webm", ".m4v",
				".heic", ".heif", ".dng", ".raw", ".cr2", ".nef", ".arw",
			},
			AllowedMimeTypes: []string{
				"image/jpeg", "image/png", "image/gif", "image/bmp", "image/tiff", "image/webp",
				"video/mp4", "video/quicktime", "video/x-msvideo", "video/x-matroska", "video/webm",
				"image/heic", "image/heif", "image/x-adobe-dng", "image/x-canon-cr2", "image/x-nikon-nef",
			},
			VirusScanEnabled: false,
			TempDir:         "/tmp/immich-uploads",
		},
	}
}