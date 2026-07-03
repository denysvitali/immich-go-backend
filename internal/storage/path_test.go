package storage

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetFallbackPath(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000/photo.jpg", AssetFallbackPath(id, "photo.jpg"))
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		prefix string
		want   string
	}{
		{name: "plain path", path: "asset/file.jpg", want: "asset/file.jpg"},
		{name: "leading slash", path: "/asset/file.jpg", want: "asset/file.jpg"},
		{name: "prefix", path: "asset/file.jpg", prefix: "library", want: "library/asset/file.jpg"},
		{name: "prefix trailing slash", path: "/asset/file.jpg", prefix: "library/", want: "library/asset/file.jpg"},
		{name: "empty path with prefix", path: "", prefix: "library", want: "library/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizePath(tt.path, tt.prefix))
		})
	}
}

func TestNormalizePathFS(t *testing.T) {
	root := t.TempDir()

	assert.Equal(t, filepath.Join(root, "asset", "file.jpg"), normalizePathFS("/asset/file.jpg", root))
	assert.Equal(t, filepath.Join(root, "asset", "file.jpg"), normalizePathFS("asset/../asset/file.jpg", root))
}

func TestBuildS3PublicURL(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		forcePathStyle bool
		useSSL         bool
		want           string
	}{
		{
			name: "aws",
			want: "https://bucket.s3.us-east-1.amazonaws.com/path/file.jpg",
		},
		{
			name:           "custom virtual host https",
			endpoint:       "s3.example.test",
			useSSL:         true,
			forcePathStyle: false,
			want:           "https://bucket.s3.example.test/path/file.jpg",
		},
		{
			name:           "custom path style http",
			endpoint:       "localhost:9000",
			useSSL:         false,
			forcePathStyle: true,
			want:           "http://localhost:9000/bucket/path/file.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildS3PublicURL("bucket", "us-east-1", tt.endpoint, "path/file.jpg", tt.forcePathStyle, tt.useSSL)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWrapError(t *testing.T) {
	cause := errors.New("boom")

	err := wrapError("upload", "asset/file.jpg", "local", cause)

	var storageErr *StorageError
	require.ErrorAs(t, err, &storageErr)
	assert.Equal(t, "upload", storageErr.Op)
	assert.Equal(t, "asset/file.jpg", storageErr.Path)
	assert.Equal(t, "local", storageErr.Backend)
	assert.ErrorIs(t, storageErr.Err, cause)
}
