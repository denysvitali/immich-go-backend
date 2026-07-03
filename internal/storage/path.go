package storage

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// AssetFallbackPath returns the fallback storage path for an asset when
// OriginalPath is empty. It constructs the path from the asset ID and
// original file name.
func AssetFallbackPath(assetID uuid.UUID, originalFileName string) string {
	return fmt.Sprintf("%s/%s", assetID.String(), originalFileName)
}

// normalizePath strips a leading slash from path and joins it with prefix (if
// any). It is used by object-store backends to turn a storage path into a key.
func normalizePath(path, prefix string) string {
	path = strings.TrimPrefix(path, "/")
	if prefix == "" {
		return path
	}
	prefix = strings.TrimSuffix(prefix, "/")
	return prefix + "/" + path
}

// normalizePathFS is like normalizePath but uses filepath.Join for local
// filesystem paths. The root is joined with the cleaned path.
func normalizePathFS(path, root string) string {
	path = filepath.Clean(strings.TrimPrefix(path, "/"))
	return filepath.Join(root, path)
}

// buildS3PublicURL constructs a public S3 URL from the given parameters.
func buildS3PublicURL(bucket, region, endpoint, key string, forcePathStyle, useSSL bool) string {
	if endpoint != "" {
		scheme := "https"
		if !useSSL {
			scheme = "http"
		}
		if forcePathStyle {
			return scheme + "://" + endpoint + "/" + bucket + "/" + key
		}
		return scheme + "://" + bucket + "." + endpoint + "/" + key
	}
	return "https://" + bucket + ".s3." + region + ".amazonaws.com/" + key
}

// wrapError creates a StorageError with the given operation, path, backend,
// and underlying error. It is the canonical way to wrap errors in the storage
// package so that every call site uses the same pattern.
func wrapError(op, path, backend string, err error) error {
	return &StorageError{
		Op:      op,
		Path:    path,
		Backend: backend,
		Err:     err,
	}
}
