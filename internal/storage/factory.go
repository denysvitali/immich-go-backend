package storage

import (
	"fmt"
	"strings"
)

type storageBackendDefinition struct {
	create   func(StorageConfig) (StorageBackend, error)
	validate func(StorageConfig) error
}

func lookupStorageBackendDefinition(backend string) (storageBackendDefinition, bool) {
	switch backend {
	case "local", "filesystem", "fs":
		return storageBackendDefinition{
			create: func(config StorageConfig) (StorageBackend, error) {
				return NewLocalBackend(config.Local)
			},
			validate: func(config StorageConfig) error {
				return validateLocalConfig(config.Local)
			},
		}, true

	case "s3", "aws":
		return storageBackendDefinition{
			create: func(config StorageConfig) (StorageBackend, error) {
				return NewS3Backend(config.S3)
			},
			validate: func(config StorageConfig) error {
				return validateS3Config(config.S3)
			},
		}, true

	case "rclone":
		return storageBackendDefinition{
			create: func(config StorageConfig) (StorageBackend, error) {
				return NewRcloneBackend(config.Rclone)
			},
			validate: func(config StorageConfig) error {
				return validateRcloneConfig(config.Rclone)
			},
		}, true

	default:
		return storageBackendDefinition{}, false
	}
}

// NewStorageBackend creates a new storage backend based on the configuration
func NewStorageBackend(config StorageConfig) (StorageBackend, error) {
	backend := strings.ToLower(config.Backend)

	definition, ok := lookupStorageBackendDefinition(backend)
	if !ok {
		return nil, wrapError("create storage backend", "", backend, fmt.Errorf("unsupported storage backend: %s", backend))
	}

	return definition.create(config)
}

// ValidateStorageConfig validates the storage configuration
func ValidateStorageConfig(config StorageConfig) error {
	backend := strings.ToLower(config.Backend)

	definition, ok := lookupStorageBackendDefinition(backend)
	if !ok {
		return wrapError("validate storage config", "", backend, fmt.Errorf("unsupported storage backend: %s", backend))
	}

	return definition.validate(config)
}

// validateLocalConfig validates local storage configuration
func validateLocalConfig(config LocalConfig) error {
	if config.RootPath == "" {
		return wrapError("validate local config", "", "local", fmt.Errorf("root_path is required"))
	}

	return nil
}

// validateS3Config validates S3 storage configuration
func validateS3Config(config S3Config) error {
	if config.Bucket == "" {
		return wrapError("validate s3 config", "", "s3", fmt.Errorf("bucket is required"))
	}

	if config.AccessKeyID == "" {
		return wrapError("validate s3 config", "", "s3", fmt.Errorf("access_key_id is required"))
	}

	if config.SecretAccessKey == "" {
		return wrapError("validate s3 config", "", "s3", fmt.Errorf("secret_access_key is required"))
	}

	if config.Region == "" {
		return wrapError("validate s3 config", "", "s3", fmt.Errorf("region is required"))
	}

	return nil
}

// validateRcloneConfig validates rclone storage configuration
func validateRcloneConfig(config RcloneConfig) error {
	if config.Remote == "" {
		return wrapError("validate rclone config", "", "rclone", fmt.Errorf("remote is required"))
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
				".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp", ".avif",
				".mp4", ".mov", ".avi", ".mkv", ".webm", ".m4v",
				".heic", ".heif", ".dng", ".raw", ".cr2", ".nef", ".arw",
			},
			AllowedMimeTypes: []string{
				"image/jpeg", "image/png", "image/gif", "image/bmp", "image/tiff", "image/webp", "image/avif",
				"video/mp4", "video/quicktime", "video/x-msvideo", "video/x-matroska", "video/webm",
				"image/heic", "image/heif", "image/x-adobe-dng", "image/x-canon-cr2", "image/x-nikon-nef",
			},
			VirusScanEnabled: false,
			TempDir:          "/tmp/immich-uploads",
		},
	}
}
