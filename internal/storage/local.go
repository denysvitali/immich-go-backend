package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// LocalBackend implements StorageBackend using local filesystem
type LocalBackend struct {
	config   LocalConfig
	rootPath string
	fileMode os.FileMode
	dirMode  os.FileMode
}

// NewLocalBackend creates a new local filesystem storage backend
func NewLocalBackend(config LocalConfig) (*LocalBackend, error) {
	// Parse file mode
	fileMode, err := parseFileMode(config.FileMode, 0644)
	if err != nil {
		return nil, &StorageError{
			Op:      "create local backend",
			Backend: "local",
			Err:     fmt.Errorf("invalid file mode %s: %w", config.FileMode, err),
		}
	}

	// Parse directory mode
	dirMode, err := parseFileMode(config.DirMode, 0755)
	if err != nil {
		return nil, &StorageError{
			Op:      "create local backend",
			Backend: "local",
			Err:     fmt.Errorf("invalid directory mode %s: %w", config.DirMode, err),
		}
	}

	// Ensure root path is absolute
	rootPath, err := filepath.Abs(config.RootPath)
	if err != nil {
		return nil, &StorageError{
			Op:      "create local backend",
			Backend: "local",
			Err:     fmt.Errorf("failed to get absolute path: %w", err),
		}
	}

	// Create root directory if it doesn't exist
	if err := os.MkdirAll(rootPath, dirMode); err != nil {
		return nil, &StorageError{
			Op:      "create local backend",
			Backend: "local",
			Err:     fmt.Errorf("failed to create root directory: %w", err),
		}
	}

	return &LocalBackend{
		config:   config,
		rootPath: rootPath,
		fileMode: fileMode,
		dirMode:  dirMode,
	}, nil
}

// parseFileMode parses a file mode string (octal) into os.FileMode
func parseFileMode(modeStr string, defaultMode os.FileMode) (os.FileMode, error) {
	if modeStr == "" {
		return defaultMode, nil
	}

	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return 0, err
	}

	return os.FileMode(mode), nil
}

// getFullPath returns the full filesystem path for a given storage path
func (l *LocalBackend) getFullPath(path string) string {
	// Clean the path and remove leading slash
	path = filepath.Clean(strings.TrimPrefix(path, "/"))
	
	// Join with root path
	return filepath.Join(l.rootPath, path)
}

// Upload uploads a file to the local filesystem
func (l *LocalBackend) Upload(ctx context.Context, path string, reader io.Reader, size int64, contentType string) error {
	ctx, span := tracer.Start(ctx, "local.Upload",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.Int64("storage.size", size),
			attribute.String("storage.content_type", contentType),
		))
	defer span.End()

	fullPath := l.getFullPath(path)
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, l.dirMode); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload",
			Path:    path,
			Backend: "local",
			Err:     fmt.Errorf("failed to create directory: %w", err),
		}
	}

	// Create the file
	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, l.fileMode)
	if err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload",
			Path:    path,
			Backend: "local",
			Err:     fmt.Errorf("failed to create file: %w", err),
		}
	}
	defer file.Close()

	// Copy data to file
	if _, err := io.Copy(file, reader); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload",
			Path:    path,
			Backend: "local",
			Err:     fmt.Errorf("failed to write file: %w", err),
		}
	}

	return nil
}

// UploadBytes uploads byte data to the local filesystem
func (l *LocalBackend) UploadBytes(ctx context.Context, path string, data []byte, contentType string) error {
	ctx, span := tracer.Start(ctx, "local.UploadBytes",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.String("storage.content_type", contentType),
			attribute.Int("storage.size", len(data)),
		))
	defer span.End()

	fullPath := l.getFullPath(path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, l.dirMode); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload bytes",
			Path:    path,
			Backend: "local",
			Err:     fmt.Errorf("failed to create directory: %w", err),
		}
	}

	// Write the file
	if err := os.WriteFile(fullPath, data, l.fileMode); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload bytes",
			Path:    path,
			Backend: "local",
			Err:     fmt.Errorf("failed to write file: %w", err),
		}
	}

	return nil
}

// Download downloads a file from the local filesystem
func (l *LocalBackend) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	ctx, span := tracer.Start(ctx, "local.Download",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	fullPath := l.getFullPath(path)
	
	file, err := os.Open(fullPath)
	if err != nil {
		span.RecordError(err)
		if os.IsNotExist(err) {
			return nil, &StorageError{
				Op:      "download",
				Path:    path,
				Backend: "local",
				Err:     fmt.Errorf("file not found"),
			}
		}
		return nil, &StorageError{
			Op:      "download",
			Path:    path,
			Backend: "local",
			Err:     fmt.Errorf("failed to open file: %w", err),
		}
	}

	return file, nil
}

// Delete deletes a file from the local filesystem
func (l *LocalBackend) Delete(ctx context.Context, path string) error {
	ctx, span := tracer.Start(ctx, "local.Delete",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	fullPath := l.getFullPath(path)
	
	if err := os.Remove(fullPath); err != nil {
		span.RecordError(err)
		if os.IsNotExist(err) {
			return &StorageError{
				Op:      "delete",
				Path:    path,
				Backend: "local",
				Err:     fmt.Errorf("file not found"),
			}
		}
		return &StorageError{
			Op:      "delete",
			Path:    path,
			Backend: "local",
			Err:     fmt.Errorf("failed to delete file: %w", err),
		}
	}

	return nil
}

// Exists checks if a file exists in the local filesystem
func (l *LocalBackend) Exists(ctx context.Context, path string) (bool, error) {
	ctx, span := tracer.Start(ctx, "local.Exists",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	fullPath := l.getFullPath(path)
	
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		span.RecordError(err)
		return false, &StorageError{
			Op:      "exists",
			Path:    path,
			Backend: "local",
			Err:     fmt.Errorf("failed to stat file: %w", err),
		}
	}

	return true, nil
}

// GetSize returns the size of a file
func (l *LocalBackend) GetSize(ctx context.Context, path string) (int64, error) {
	ctx, span := tracer.Start(ctx, "local.GetSize",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	fullPath := l.getFullPath(path)
	
	info, err := os.Stat(fullPath)
	if err != nil {
		span.RecordError(err)
		if os.IsNotExist(err) {
			return 0, &StorageError{
				Op:      "get size",
				Path:    path,
				Backend: "local",
				Err:     fmt.Errorf("file not found"),
			}
		}
		return 0, &StorageError{
			Op:      "get size",
			Path:    path,
			Backend: "local",
			Err:     fmt.Errorf("failed to stat file: %w", err),
		}
	}

	return info.Size(), nil
}

// GetPresignedUploadURL is not supported by local backend
func (l *LocalBackend) GetPresignedUploadURL(ctx context.Context, path string, contentType string, expiry time.Duration) (*PresignedURL, error) {
	return nil, &StorageError{
		Op:      "get presigned upload URL",
		Path:    path,
		Backend: "local",
		Err:     fmt.Errorf("presigned URLs not supported by local backend"),
	}
}

// GetPresignedDownloadURL is not supported by local backend
func (l *LocalBackend) GetPresignedDownloadURL(ctx context.Context, path string, expiry time.Duration) (*PresignedURL, error) {
	return nil, &StorageError{
		Op:      "get presigned download URL",
		Path:    path,
		Backend: "local",
		Err:     fmt.Errorf("presigned URLs not supported by local backend"),
	}
}

// SupportsPresignedURLs returns false for local backend
func (l *LocalBackend) SupportsPresignedURLs() bool {
	return false
}

// GetPublicURL is not supported by local backend
func (l *LocalBackend) GetPublicURL(ctx context.Context, path string) (string, error) {
	return "", &StorageError{
		Op:      "get public URL",
		Path:    path,
		Backend: "local",
		Err:     fmt.Errorf("public URLs not supported by local backend"),
	}
}

// Copy copies a file within the local filesystem
func (l *LocalBackend) Copy(ctx context.Context, srcPath, dstPath string) error {
	ctx, span := tracer.Start(ctx, "local.Copy",
		trace.WithAttributes(
			attribute.String("storage.src_path", srcPath),
			attribute.String("storage.dst_path", dstPath),
		))
	defer span.End()

	srcFullPath := l.getFullPath(srcPath)
	dstFullPath := l.getFullPath(dstPath)
	
	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dstFullPath)
	if err := os.MkdirAll(dstDir, l.dirMode); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "copy",
			Path:    srcPath,
			Backend: "local",
			Err:     fmt.Errorf("failed to create destination directory: %w", err),
		}
	}

	// Open source file
	srcFile, err := os.Open(srcFullPath)
	if err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "copy",
			Path:    srcPath,
			Backend: "local",
			Err:     fmt.Errorf("failed to open source file: %w", err),
		}
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.OpenFile(dstFullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, l.fileMode)
	if err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "copy",
			Path:    srcPath,
			Backend: "local",
			Err:     fmt.Errorf("failed to create destination file: %w", err),
		}
	}
	defer dstFile.Close()

	// Copy data
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "copy",
			Path:    srcPath,
			Backend: "local",
			Err:     fmt.Errorf("failed to copy file data: %w", err),
		}
	}

	return nil
}

// Move moves a file within the local filesystem
func (l *LocalBackend) Move(ctx context.Context, srcPath, dstPath string) error {
	ctx, span := tracer.Start(ctx, "local.Move",
		trace.WithAttributes(
			attribute.String("storage.src_path", srcPath),
			attribute.String("storage.dst_path", dstPath),
		))
	defer span.End()

	srcFullPath := l.getFullPath(srcPath)
	dstFullPath := l.getFullPath(dstPath)
	
	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dstFullPath)
	if err := os.MkdirAll(dstDir, l.dirMode); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "move",
			Path:    srcPath,
			Backend: "local",
			Err:     fmt.Errorf("failed to create destination directory: %w", err),
		}
	}

	// Try to rename first (fastest if on same filesystem)
	if err := os.Rename(srcFullPath, dstFullPath); err != nil {
		// If rename fails, fall back to copy + delete
		if err := l.Copy(ctx, srcPath, dstPath); err != nil {
			span.RecordError(err)
			return err
		}
		
		if err := l.Delete(ctx, srcPath); err != nil {
			span.RecordError(err)
			return err
		}
	}

	return nil
}

// List lists files in a directory
func (l *LocalBackend) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	ctx, span := tracer.Start(ctx, "local.List",
		trace.WithAttributes(
			attribute.String("storage.prefix", prefix),
			attribute.Bool("storage.recursive", recursive),
		))
	defer span.End()

	fullPath := l.getFullPath(prefix)
	
	var files []FileInfo
	
	if recursive {
		err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			
			// Skip the root directory itself
			if path == fullPath {
				return nil
			}
			
			// Get relative path from root
			relPath, err := filepath.Rel(l.rootPath, path)
			if err != nil {
				return err
			}
			
			// Convert to forward slashes for consistency
			relPath = filepath.ToSlash(relPath)
			
			files = append(files, FileInfo{
				Path:        relPath,
				Size:        info.Size(),
				ModTime:     info.ModTime(),
				IsDir:       info.IsDir(),
				ContentType: mime.TypeByExtension(filepath.Ext(path)),
			})
			
			return nil
		})
		
		if err != nil {
			span.RecordError(err)
			return nil, &StorageError{
				Op:      "list",
				Path:    prefix,
				Backend: "local",
				Err:     fmt.Errorf("failed to walk directory: %w", err),
			}
		}
	} else {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			span.RecordError(err)
			return nil, &StorageError{
				Op:      "list",
				Path:    prefix,
				Backend: "local",
				Err:     fmt.Errorf("failed to read directory: %w", err),
			}
		}
		
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			
			// Get relative path from root
			entryPath := filepath.Join(fullPath, entry.Name())
			relPath, err := filepath.Rel(l.rootPath, entryPath)
			if err != nil {
				continue
			}
			
			// Convert to forward slashes for consistency
			relPath = filepath.ToSlash(relPath)
			
			files = append(files, FileInfo{
				Path:        relPath,
				Size:        info.Size(),
				ModTime:     info.ModTime(),
				IsDir:       info.IsDir(),
				ContentType: mime.TypeByExtension(filepath.Ext(entry.Name())),
			})
		}
	}

	return files, nil
}

// GetMetadata returns metadata about a file
func (l *LocalBackend) GetMetadata(ctx context.Context, path string) (*FileMetadata, error) {
	ctx, span := tracer.Start(ctx, "local.GetMetadata",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	fullPath := l.getFullPath(path)
	
	info, err := os.Stat(fullPath)
	if err != nil {
		span.RecordError(err)
		if os.IsNotExist(err) {
			return nil, &StorageError{
				Op:      "get metadata",
				Path:    path,
				Backend: "local",
				Err:     fmt.Errorf("file not found"),
			}
		}
		return nil, &StorageError{
			Op:      "get metadata",
			Path:    path,
			Backend: "local",
			Err:     fmt.Errorf("failed to stat file: %w", err),
		}
	}

	return &FileMetadata{
		Path:        path,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		ContentType: mime.TypeByExtension(filepath.Ext(path)),
		Metadata:    make(map[string]string),
	}, nil
}

// Close closes the local backend
func (l *LocalBackend) Close() error {
	// Nothing to close for local filesystem
	return nil
}