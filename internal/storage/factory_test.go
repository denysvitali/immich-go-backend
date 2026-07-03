package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStorageConfigBackendAliases(t *testing.T) {
	tests := []struct {
		name   string
		config StorageConfig
	}{
		{
			name: "local",
			config: StorageConfig{
				Backend: "local",
				Local:   LocalConfig{RootPath: t.TempDir()},
			},
		},
		{
			name: "filesystem alias",
			config: StorageConfig{
				Backend: "filesystem",
				Local:   LocalConfig{RootPath: t.TempDir()},
			},
		},
		{
			name: "fs alias",
			config: StorageConfig{
				Backend: "fs",
				Local:   LocalConfig{RootPath: t.TempDir()},
			},
		},
		{
			name: "uppercase local",
			config: StorageConfig{
				Backend: "LOCAL",
				Local:   LocalConfig{RootPath: t.TempDir()},
			},
		},
		{
			name: "s3",
			config: StorageConfig{
				Backend: "s3",
				S3: S3Config{
					Bucket:          "photos",
					AccessKeyID:     "access-key",
					SecretAccessKey: "secret-key",
					Region:          "us-east-1",
				},
			},
		},
		{
			name: "aws alias",
			config: StorageConfig{
				Backend: "aws",
				S3: S3Config{
					Bucket:          "photos",
					AccessKeyID:     "access-key",
					SecretAccessKey: "secret-key",
					Region:          "us-east-1",
				},
			},
		},
		{
			name: "rclone",
			config: StorageConfig{
				Backend: "rclone",
				Rclone:  RcloneConfig{Remote: "photos"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, ValidateStorageConfig(tt.config))
		})
	}
}

func TestNewStorageBackendLocalAliases(t *testing.T) {
	tests := []string{"local", "filesystem", "fs", "LOCAL"}

	for _, backendName := range tests {
		t.Run(backendName, func(t *testing.T) {
			backend, err := NewStorageBackend(StorageConfig{
				Backend: backendName,
				Local:   LocalConfig{RootPath: t.TempDir()},
			})

			require.NoError(t, err)
			require.IsType(t, &LocalBackend{}, backend)
			assert.NoError(t, backend.Close())
		})
	}
}

func TestLookupStorageBackendDefinitionRejectsUnsupportedBackend(t *testing.T) {
	_, ok := lookupStorageBackendDefinition("memory")

	assert.False(t, ok)
}
