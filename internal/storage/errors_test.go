package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStorageConfigErrorsUseStorageError(t *testing.T) {
	tests := []struct {
		name        string
		config      StorageConfig
		wantOp      string
		wantBackend string
		wantMessage string
	}{
		{
			name:        "unsupported backend",
			config:      StorageConfig{Backend: "memory"},
			wantOp:      "validate storage config",
			wantBackend: "memory",
			wantMessage: "unsupported storage backend: memory",
		},
		{
			name:        "missing local root",
			config:      StorageConfig{Backend: "local"},
			wantOp:      "validate local config",
			wantBackend: "local",
			wantMessage: "root_path is required",
		},
		{
			name:        "missing s3 bucket",
			config:      StorageConfig{Backend: "s3"},
			wantOp:      "validate s3 config",
			wantBackend: "s3",
			wantMessage: "bucket is required",
		},
		{
			name:        "missing rclone remote",
			config:      StorageConfig{Backend: "rclone"},
			wantOp:      "validate rclone config",
			wantBackend: "rclone",
			wantMessage: "remote is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStorageConfig(tt.config)

			var storageErr *StorageError
			require.ErrorAs(t, err, &storageErr)
			assert.Equal(t, tt.wantOp, storageErr.Op)
			assert.Equal(t, tt.wantBackend, storageErr.Backend)
			assert.Contains(t, storageErr.Err.Error(), tt.wantMessage)
		})
	}
}

func TestNewStorageBackendUnsupportedBackendUsesStorageError(t *testing.T) {
	_, err := NewStorageBackend(StorageConfig{Backend: "memory"})

	assertStorageError(t, err, "create storage backend", "", "memory", "unsupported storage backend: memory")
}

func TestStorageServiceValidationErrorsUseStorageError(t *testing.T) {
	svc := &Service{
		config: StorageConfig{
			Backend: "local",
			Upload: UploadConfig{
				MaxFileSize:       10,
				AllowedExtensions: []string{".jpg"},
				AllowedMimeTypes:  []string{"image/jpeg"},
			},
		},
	}

	_, err := svc.validateUpload("asset.png", "image/png")
	assertStorageError(t, err, "upload asset", "asset.png", "local", "file extension .png is not allowed")

	_, err = svc.validateUpload("asset.jpg", "application/json")
	assertStorageError(t, err, "upload asset", "asset.jpg", "local", "MIME type application/json is not allowed")

	_, err = svc.UploadAsset(context.Background(), "user-id", "asset.jpg", strings.NewReader("too large"), 11)
	assertStorageError(t, err, "upload asset", "asset.jpg", "local", "file size 11 exceeds maximum allowed size 10")
}

func TestDefaultStorageConfigAcceptsAVIFUploads(t *testing.T) {
	config := GetDefaultStorageConfig()
	svc := &Service{config: config}

	_, err := svc.validateUpload("asset.avif", "")

	require.NoError(t, err)
}

func TestGetAssetUploadURLUnsupportedBackendUsesStorageError(t *testing.T) {
	backend, err := NewLocalBackend(LocalConfig{RootPath: t.TempDir()})
	require.NoError(t, err)

	svc := &Service{
		backend: backend,
		config: StorageConfig{
			Backend: "local",
			Upload: UploadConfig{
				AllowedExtensions: []string{".jpg"},
				AllowedMimeTypes:  []string{"image/jpeg"},
			},
		},
	}

	_, err = svc.GetAssetUploadURL(context.Background(), "user-id", "asset.jpg", "image/jpeg")

	assertStorageError(t, err, "get asset upload URL", "", "local", "presigned URLs not supported by backend local")
}

func TestUnsupportedOperationErrorsWrapSentinels(t *testing.T) {
	ctx := context.Background()
	local := &LocalBackend{}
	rclone := &RcloneBackend{}

	_, err := local.GetPresignedUploadURL(ctx, "asset.jpg", "image/jpeg", time.Minute)
	assertStorageError(t, err, "get presigned upload URL", "asset.jpg", "local", "presigned URLs not supported")
	assert.True(t, errors.Is(err, ErrPresignedURLsNotSupported))

	_, err = rclone.GetPresignedDownloadURL(ctx, "asset.jpg", time.Minute)
	assertStorageError(t, err, "get presigned download URL", "asset.jpg", "rclone", "presigned URLs not supported")
	assert.True(t, errors.Is(err, ErrPresignedURLsNotSupported))

	_, err = local.GetPublicURL(ctx, "asset.jpg")
	assertStorageError(t, err, "get public URL", "asset.jpg", "local", "public URLs not supported")
	assert.True(t, errors.Is(err, ErrPublicURLNotSupported))

	_, err = rclone.GetPublicURL(ctx, "asset.jpg")
	assertStorageError(t, err, "get public URL", "asset.jpg", "rclone", "public URLs not supported")
	assert.True(t, errors.Is(err, ErrPublicURLNotSupported))
}

func assertStorageError(t *testing.T, err error, op, path, backend, contains string) {
	t.Helper()

	var storageErr *StorageError
	require.ErrorAs(t, err, &storageErr)
	assert.Equal(t, op, storageErr.Op)
	assert.Equal(t, path, storageErr.Path)
	assert.Equal(t, backend, storageErr.Backend)
	assert.Contains(t, storageErr.Err.Error(), contains)
}
