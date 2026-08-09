package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
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
	fileMode, err := parseFileMode(config.FileMode, 0o644)
	if err != nil {
		return nil, wrapError("create local backend", "", "local", fmt.Errorf("invalid file mode %s: %w", config.FileMode, err))
	}

	// Parse directory mode
	dirMode, err := parseFileMode(config.DirMode, 0o755)
	if err != nil {
		return nil, wrapError("create local backend", "", "local", fmt.Errorf("invalid directory mode %s: %w", config.DirMode, err))
	}

	// Ensure root path is absolute
	rootPath, err := filepath.Abs(config.RootPath)
	if err != nil {
		return nil, wrapError("create local backend", "", "local", fmt.Errorf("failed to get absolute path: %w", err))
	}

	// Create root directory if it doesn't exist
	if err := os.MkdirAll(rootPath, dirMode); err != nil {
		return nil, wrapError("create local backend", "", "local", fmt.Errorf("failed to create root directory: %w", err))
	}

	// Resolve symlinks in the configured root once. Symlinks below this
	// canonical root are rejected by getFullPath so a stored path can never be
	// redirected outside the backend after initialization.
	rootPath, err = filepath.EvalSymlinks(rootPath)
	if err != nil {
		return nil, wrapError("create local backend", "", "local", fmt.Errorf("failed to resolve root directory: %w", err))
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

// getFullPath returns a contained filesystem path and rejects every symlink
// below the canonical storage root. The local backend intentionally does not
// follow user-controlled symlinks.
func (l *LocalBackend) getFullPath(path string, allowRoot bool) (string, error) {
	fullPath, err := normalizePathFS(path, l.rootPath)
	if err != nil {
		return "", err
	}
	if !allowRoot && fullPath == l.rootPath {
		return "", fmt.Errorf("storage path resolves to root")
	}
	if err := l.rejectSymlinks(fullPath); err != nil {
		return "", err
	}
	return fullPath, nil
}

func (l *LocalBackend) rejectSymlinks(fullPath string) error {
	relPath, err := filepath.Rel(l.rootPath, fullPath)
	if err != nil {
		return fmt.Errorf("resolve path relative to storage root: %w", err)
	}
	if relPath == "." {
		return nil
	}

	currentPath := l.rootPath
	for _, part := range strings.Split(relPath, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		currentPath = filepath.Join(currentPath, part)
		info, err := os.Lstat(currentPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("inspect path component: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink path component is not allowed: %s", part)
		}
	}
	return nil
}

// atomicWrite writes and syncs a temporary file in the destination directory,
// then atomically renames it into place. A failed write never exposes partial
// content or replaces an existing destination.
func (l *LocalBackend) atomicWrite(path string, reader io.Reader, size int64) error {
	fullPath, err := l.getFullPath(path, false)
	if err != nil {
		return err
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, l.dirMode); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	// MkdirAll may have encountered a concurrently introduced path component;
	// validate the complete destination again before creating the temp file.
	if _, err := l.getFullPath(path, false); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(dir, ".immich-go-upload-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempPath := tempFile.Name()
	committed := false
	defer func() {
		_ = tempFile.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()

	if err := tempFile.Chmod(l.fileMode); err != nil {
		return fmt.Errorf("failed to set temporary file mode: %w", err)
	}

	var written int64
	if size >= 0 && size < math.MaxInt64 {
		limited := &io.LimitedReader{R: reader, N: size + 1}
		written, err = io.Copy(tempFile, limited)
	} else {
		written, err = io.Copy(tempFile, reader)
	}
	if err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if size >= 0 && written != size {
		if written > size {
			return fmt.Errorf("uploaded size mismatch: declared %d bytes, received more data", size)
		}
		return fmt.Errorf("uploaded size mismatch: declared %d bytes, received %d", size, written)
	}
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err := os.Rename(tempPath, fullPath); err != nil {
		return fmt.Errorf("failed to commit temporary file: %w", err)
	}
	committed = true

	// Persist the directory entry as well as the file contents so a successful
	// upload survives a host crash after rename.
	dirHandle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("failed to open destination directory for sync: %w", err)
	}
	if err := dirHandle.Sync(); err != nil {
		_ = dirHandle.Close()
		return fmt.Errorf("failed to sync destination directory: %w", err)
	}
	if err := dirHandle.Close(); err != nil {
		return fmt.Errorf("failed to close destination directory: %w", err)
	}
	return nil
}

// Upload uploads a file to the local filesystem
func (l *LocalBackend) Upload(ctx context.Context, path string, reader io.Reader, size int64, contentType string) error {
	_, span := tracer.Start(ctx, "local.Upload",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.Int64("storage.size", size),
			attribute.String("storage.content_type", contentType),
		))
	defer span.End()

	if err := l.atomicWrite(path, reader, size); err != nil {
		span.RecordError(err)
		return wrapError("upload", path, "local", err)
	}

	return nil
}

// UploadBytes uploads byte data to the local filesystem
func (l *LocalBackend) UploadBytes(ctx context.Context, path string, data []byte, contentType string) error {
	_, span := tracer.Start(ctx, "local.UploadBytes",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.String("storage.content_type", contentType),
			attribute.Int("storage.size", len(data)),
		))
	defer span.End()

	if err := l.atomicWrite(path, bytes.NewReader(data), int64(len(data))); err != nil {
		span.RecordError(err)
		return wrapError("upload bytes", path, "local", err)
	}

	return nil
}

// Download downloads a file from the local filesystem
func (l *LocalBackend) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	_, span := tracer.Start(ctx, "local.Download",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	fullPath, err := l.getFullPath(path, false)
	if err != nil {
		span.RecordError(err)
		return nil, wrapError("download", path, "local", err)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		span.RecordError(err)
		if os.IsNotExist(err) {
			return nil, wrapError("download", path, "local", fmt.Errorf("file not found"))
		}
		return nil, wrapError("download", path, "local", fmt.Errorf("failed to open file: %w", err))
	}

	return file, nil
}

// Delete deletes a file from the local filesystem
func (l *LocalBackend) Delete(ctx context.Context, path string) error {
	_, span := tracer.Start(ctx, "local.Delete",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	fullPath, err := l.getFullPath(path, false)
	if err != nil {
		span.RecordError(err)
		return wrapError("delete", path, "local", err)
	}

	if err := os.Remove(fullPath); err != nil {
		span.RecordError(err)
		if os.IsNotExist(err) {
			return wrapError("delete", path, "local", fmt.Errorf("file not found"))
		}
		return wrapError("delete", path, "local", fmt.Errorf("failed to delete file: %w", err))
	}

	return nil
}

// Exists checks if a file exists in the local filesystem
func (l *LocalBackend) Exists(ctx context.Context, path string) (bool, error) {
	_, span := tracer.Start(ctx, "local.Exists",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	fullPath, err := l.getFullPath(path, false)
	if err != nil {
		span.RecordError(err)
		return false, wrapError("exists", path, "local", err)
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		span.RecordError(err)
		return false, wrapError("exists", path, "local", fmt.Errorf("failed to stat file: %w", err))
	}

	return true, nil
}

// GetSize returns the size of a file
func (l *LocalBackend) GetSize(ctx context.Context, path string) (int64, error) {
	_, span := tracer.Start(ctx, "local.GetSize",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	fullPath, err := l.getFullPath(path, false)
	if err != nil {
		span.RecordError(err)
		return 0, wrapError("get size", path, "local", err)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		span.RecordError(err)
		if os.IsNotExist(err) {
			return 0, wrapError("get size", path, "local", fmt.Errorf("file not found"))
		}
		return 0, wrapError("get size", path, "local", fmt.Errorf("failed to stat file: %w", err))
	}

	return info.Size(), nil
}

// GetPresignedUploadURL is not supported by local backend
func (l *LocalBackend) GetPresignedUploadURL(ctx context.Context, path string, contentType string, expiry time.Duration) (*PresignedURL, error) {
	_ = ctx // Mark as intentionally unused
	return nil, wrapError("get presigned upload URL", path, "local", ErrPresignedURLsNotSupported)
}

// GetPresignedDownloadURL is not supported by local backend
func (l *LocalBackend) GetPresignedDownloadURL(ctx context.Context, path string, expiry time.Duration) (*PresignedURL, error) {
	_ = ctx // Mark as intentionally unused
	return nil, wrapError("get presigned download URL", path, "local", ErrPresignedURLsNotSupported)
}

// SupportsPresignedURLs returns false for local backend
func (l *LocalBackend) SupportsPresignedURLs() bool {
	return false
}

// GetPublicURL is not supported by local backend
func (l *LocalBackend) GetPublicURL(ctx context.Context, path string) (string, error) {
	_ = ctx // Mark as intentionally unused
	return "", wrapError("get public URL", path, "local", ErrPublicURLNotSupported)
}

// Copy copies a file within the local filesystem
func (l *LocalBackend) Copy(ctx context.Context, srcPath, dstPath string) error {
	_, span := tracer.Start(ctx, "local.Copy",
		trace.WithAttributes(
			attribute.String("storage.src_path", srcPath),
			attribute.String("storage.dst_path", dstPath),
		))
	defer span.End()

	srcFullPath, err := l.getFullPath(srcPath, false)
	if err != nil {
		span.RecordError(err)
		return wrapError("copy", srcPath, "local", err)
	}
	dstFullPath, err := l.getFullPath(dstPath, false)
	if err != nil {
		span.RecordError(err)
		return wrapError("copy", dstPath, "local", err)
	}

	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dstFullPath)
	if err := os.MkdirAll(dstDir, l.dirMode); err != nil {
		span.RecordError(err)
		return wrapError("copy", dstPath, "local", fmt.Errorf("failed to create destination directory: %w", err))
	}
	if _, err := l.getFullPath(dstPath, false); err != nil {
		span.RecordError(err)
		return wrapError("copy", dstPath, "local", err)
	}

	// Open source file
	srcFile, err := os.Open(srcFullPath)
	if err != nil {
		span.RecordError(err)
		return wrapError("copy", srcPath, "local", fmt.Errorf("failed to open source file: %w", err))
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		span.RecordError(err)
		return wrapError("copy", srcPath, "local", fmt.Errorf("failed to stat source file: %w", err))
	}

	// Use the same atomic commit path as uploads so a failed copy cannot expose
	// a truncated destination.
	if err := l.atomicWrite(dstPath, srcFile, srcInfo.Size()); err != nil {
		span.RecordError(err)
		return wrapError("copy", dstPath, "local", fmt.Errorf("failed to copy file data: %w", err))
	}

	return nil
}

// Move moves a file within the local filesystem
func (l *LocalBackend) Move(ctx context.Context, srcPath, dstPath string) error {
	_, span := tracer.Start(ctx, "local.Move",
		trace.WithAttributes(
			attribute.String("storage.src_path", srcPath),
			attribute.String("storage.dst_path", dstPath),
		))
	defer span.End()

	srcFullPath, err := l.getFullPath(srcPath, false)
	if err != nil {
		span.RecordError(err)
		return wrapError("move", srcPath, "local", err)
	}
	dstFullPath, err := l.getFullPath(dstPath, false)
	if err != nil {
		span.RecordError(err)
		return wrapError("move", dstPath, "local", err)
	}

	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dstFullPath)
	if err := os.MkdirAll(dstDir, l.dirMode); err != nil {
		span.RecordError(err)
		return wrapError("move", dstPath, "local", fmt.Errorf("failed to create destination directory: %w", err))
	}
	if _, err := l.getFullPath(dstPath, false); err != nil {
		span.RecordError(err)
		return wrapError("move", dstPath, "local", err)
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
	_, span := tracer.Start(ctx, "local.List",
		trace.WithAttributes(
			attribute.String("storage.prefix", prefix),
			attribute.Bool("storage.recursive", recursive),
		))
	defer span.End()

	fullPath, err := l.getFullPath(prefix, true)
	if err != nil {
		span.RecordError(err)
		return nil, wrapError("list", prefix, "local", err)
	}

	var files []FileInfo

	if recursive {
		err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink path component is not allowed: %s", info.Name())
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
			return nil, wrapError("list", prefix, "local", fmt.Errorf("failed to walk directory: %w", err))
		}
	} else {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			span.RecordError(err)
			return nil, wrapError("list", prefix, "local", fmt.Errorf("failed to read directory: %w", err))
		}

		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, wrapError("list", prefix, "local", fmt.Errorf("symlink path component is not allowed: %s", entry.Name()))
			}
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
	_, span := tracer.Start(ctx, "local.GetMetadata",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	fullPath, err := l.getFullPath(path, false)
	if err != nil {
		span.RecordError(err)
		return nil, wrapError("get metadata", path, "local", err)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		span.RecordError(err)
		if os.IsNotExist(err) {
			return nil, wrapError("get metadata", path, "local", fmt.Errorf("file not found"))
		}
		return nil, wrapError("get metadata", path, "local", fmt.Errorf("failed to stat file: %w", err))
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
