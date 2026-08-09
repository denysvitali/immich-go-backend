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

// normalizePathFS resolves a storage path below root. A leading separator is
// accepted for parity with object-store keys, but parent traversal and paths
// that otherwise escape root are rejected.
func normalizePathFS(path, root string) (string, error) {
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("storage root must be absolute")
	}

	// Storage paths use a leading slash as an optional key delimiter. Remove
	// all leading platform separators before cleaning so they cannot turn the
	// joined result into an absolute filesystem path.
	path = strings.TrimLeft(path, string(filepath.Separator))
	cleanPath := filepath.Clean(path)
	if volume := filepath.VolumeName(cleanPath); volume != "" {
		return "", fmt.Errorf("storage path must not contain a volume name")
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage path escapes root")
	}

	fullPath := filepath.Join(root, cleanPath)
	relPath, err := filepath.Rel(root, fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve storage path: %w", err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("storage path escapes root")
	}

	return fullPath, nil
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
