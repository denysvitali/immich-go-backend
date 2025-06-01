package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("immich-go-backend/storage")

// RcloneBackend implements StorageBackend using rclone
type RcloneBackend struct {
	config RcloneConfig
	remote string // Full remote path (remote:path)
}

// NewRcloneBackend creates a new rclone storage backend
func NewRcloneBackend(config RcloneConfig) (*RcloneBackend, error) {
	if config.Remote == "" {
		return nil, &StorageError{
			Op:      "create rclone backend",
			Backend: "rclone",
			Err:     fmt.Errorf("remote name is required"),
		}
	}

	// Construct full remote path
	remote := config.Remote
	if config.Path != "" && config.Path != "/" {
		remote = config.Remote + ":" + strings.TrimPrefix(config.Path, "/")
	} else {
		remote = config.Remote + ":"
	}

	backend := &RcloneBackend{
		config: config,
		remote: remote,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	if err := backend.testConnection(ctx); err != nil {
		return nil, &StorageError{
			Op:      "test rclone connection",
			Backend: "rclone",
			Err:     err,
		}
	}

	return backend, nil
}

// testConnection tests if rclone can connect to the remote
func (r *RcloneBackend) testConnection(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "rclone.testConnection")
	defer span.End()

	cmd := r.buildCommand(ctx, "lsd", r.remote, "--max-depth", "1")
	if err := cmd.Run(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to connect to rclone remote %s: %w", r.config.Remote, err)
	}

	return nil
}

// buildCommand builds an rclone command with common flags
func (r *RcloneBackend) buildCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmdArgs := []string{}

	// Add config file if specified
	if r.config.ConfigFile != "" {
		cmdArgs = append(cmdArgs, "--config", r.config.ConfigFile)
	}

	// Add custom flags
	cmdArgs = append(cmdArgs, r.config.Flags...)

	// Add the actual command arguments
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, "rclone", cmdArgs...)
	return cmd
}

// Upload uploads a file to the rclone remote
func (r *RcloneBackend) Upload(ctx context.Context, path string, reader io.Reader, size int64, contentType string) error {
	ctx, span := tracer.Start(ctx, "rclone.Upload",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.Int64("storage.size", size),
			attribute.String("storage.content_type", contentType),
		))
	defer span.End()

	remotePath := r.getRemotePath(path)

	// Create a temporary file to write the data
	tempFile, err := os.CreateTemp("", "rclone-upload-*")
	if err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("failed to create temp file: %w", err),
		}
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Copy data to temp file
	if _, err := io.Copy(tempFile, reader); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("failed to write to temp file: %w", err),
		}
	}

	// Close temp file before rclone uses it
	tempFile.Close()

	// Use rclone copyto to upload the file
	cmd := r.buildCommand(ctx, "copyto", tempFile.Name(), remotePath)
	if err := cmd.Run(); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("rclone copyto failed: %w", err),
		}
	}

	return nil
}

// UploadBytes uploads byte data to the rclone remote
func (r *RcloneBackend) UploadBytes(ctx context.Context, path string, data []byte, contentType string) error {
	ctx, span := tracer.Start(ctx, "rclone.UploadBytes",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.String("storage.content_type", contentType),
			attribute.Int("storage.size", len(data)),
		))
	defer span.End()

	remotePath := r.getRemotePath(path)

	// Create a temporary file for the data
	tmpFile, err := os.CreateTemp("", "rclone-upload-*")
	if err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload bytes",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("failed to create temp file: %w", err),
		}
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload bytes",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("failed to write to temp file: %w", err),
		}
	}

	// Close the file before rclone uses it
	tmpFile.Close()

	// Use rclone copyto to upload the file
	cmd := exec.CommandContext(ctx, "rclone", "copyto", tmpFile.Name(), remotePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload bytes",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("rclone copyto failed: %s: %w", string(output), err),
		}
	}

	return nil
}

// Download downloads a file from the rclone remote
func (r *RcloneBackend) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	ctx, span := tracer.Start(ctx, "rclone.Download",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	remotePath := r.getRemotePath(path)

	// Use rclone cat to stream the file
	cmd := r.buildCommand(ctx, "cat", remotePath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		span.RecordError(err)
		return nil, &StorageError{
			Op:      "download",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("failed to create stdout pipe: %w", err),
		}
	}

	if err := cmd.Start(); err != nil {
		span.RecordError(err)
		return nil, &StorageError{
			Op:      "download",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("rclone cat failed to start: %w", err),
		}
	}

	// Return a ReadCloser that also waits for the command to finish
	return &rcloneReadCloser{
		ReadCloser: stdout,
		cmd:        cmd,
	}, nil
}

// rcloneReadCloser wraps the stdout pipe and ensures the command finishes
type rcloneReadCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (r *rcloneReadCloser) Close() error {
	// Close the pipe first
	if err := r.ReadCloser.Close(); err != nil {
		return err
	}

	// Wait for the command to finish
	return r.cmd.Wait()
}

// Delete deletes a file from the rclone remote
func (r *RcloneBackend) Delete(ctx context.Context, path string) error {
	ctx, span := tracer.Start(ctx, "rclone.Delete",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	remotePath := r.getRemotePath(path)

	cmd := r.buildCommand(ctx, "deletefile", remotePath)
	if err := cmd.Run(); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "delete",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("rclone deletefile failed: %w", err),
		}
	}

	return nil
}

// Exists checks if a file exists in the rclone remote
func (r *RcloneBackend) Exists(ctx context.Context, path string) (bool, error) {
	ctx, span := tracer.Start(ctx, "rclone.Exists",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	remotePath := r.getRemotePath(path)

	cmd := r.buildCommand(ctx, "lsf", remotePath)
	err := cmd.Run()

	if err != nil {
		// If the command fails, the file doesn't exist
		return false, nil
	}

	return true, nil
}

// GetSize returns the size of a file
func (r *RcloneBackend) GetSize(ctx context.Context, path string) (int64, error) {
	ctx, span := tracer.Start(ctx, "rclone.GetSize",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	metadata, err := r.GetMetadata(ctx, path)
	if err != nil {
		span.RecordError(err)
		return 0, err
	}

	return metadata.Size, nil
}

// GetPresignedUploadURL is not supported by rclone
func (r *RcloneBackend) GetPresignedUploadURL(ctx context.Context, path string, contentType string, expiry time.Duration) (*PresignedURL, error) {
	return nil, &StorageError{
		Op:      "get presigned upload URL",
		Path:    path,
		Backend: "rclone",
		Err:     fmt.Errorf("presigned URLs not supported by rclone backend"),
	}
}

// GetPresignedDownloadURL is not supported by rclone
func (r *RcloneBackend) GetPresignedDownloadURL(ctx context.Context, path string, expiry time.Duration) (*PresignedURL, error) {
	return nil, &StorageError{
		Op:      "get presigned download URL",
		Path:    path,
		Backend: "rclone",
		Err:     fmt.Errorf("presigned URLs not supported by rclone backend"),
	}
}

// SupportsPresignedURLs returns false for rclone
func (r *RcloneBackend) SupportsPresignedURLs() bool {
	return false
}

// GetPublicURL is not supported by rclone
func (r *RcloneBackend) GetPublicURL(ctx context.Context, path string) (string, error) {
	return "", &StorageError{
		Op:      "get public URL",
		Path:    path,
		Backend: "rclone",
		Err:     fmt.Errorf("public URLs not supported by rclone backend"),
	}
}

// Copy copies a file within the rclone remote
func (r *RcloneBackend) Copy(ctx context.Context, srcPath, dstPath string) error {
	ctx, span := tracer.Start(ctx, "rclone.Copy",
		trace.WithAttributes(
			attribute.String("storage.src_path", srcPath),
			attribute.String("storage.dst_path", dstPath),
		))
	defer span.End()

	srcRemotePath := r.getRemotePath(srcPath)
	dstRemotePath := r.getRemotePath(dstPath)

	cmd := r.buildCommand(ctx, "copyto", srcRemotePath, dstRemotePath)
	if err := cmd.Run(); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "copy",
			Path:    srcPath,
			Backend: "rclone",
			Err:     fmt.Errorf("rclone copyto failed: %w", err),
		}
	}

	return nil
}

// Move moves a file within the rclone remote
func (r *RcloneBackend) Move(ctx context.Context, srcPath, dstPath string) error {
	ctx, span := tracer.Start(ctx, "rclone.Move",
		trace.WithAttributes(
			attribute.String("storage.src_path", srcPath),
			attribute.String("storage.dst_path", dstPath),
		))
	defer span.End()

	srcRemotePath := r.getRemotePath(srcPath)
	dstRemotePath := r.getRemotePath(dstPath)

	cmd := r.buildCommand(ctx, "moveto", srcRemotePath, dstRemotePath)
	if err := cmd.Run(); err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "move",
			Path:    srcPath,
			Backend: "rclone",
			Err:     fmt.Errorf("rclone moveto failed: %w", err),
		}
	}

	return nil
}

// List lists files in a directory
func (r *RcloneBackend) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	ctx, span := tracer.Start(ctx, "rclone.List",
		trace.WithAttributes(
			attribute.String("storage.prefix", prefix),
			attribute.Bool("storage.recursive", recursive),
		))
	defer span.End()

	remotePath := r.getRemotePath(prefix)

	args := []string{"lsjson"}
	if recursive {
		args = append(args, "--recursive")
	}
	args = append(args, remotePath)

	cmd := r.buildCommand(ctx, args...)
	output, err := cmd.Output()
	if err != nil {
		span.RecordError(err)
		return nil, &StorageError{
			Op:      "list",
			Path:    prefix,
			Backend: "rclone",
			Err:     fmt.Errorf("rclone lsjson failed: %w", err),
		}
	}

	var rcloneFiles []struct {
		Path     string    `json:"Path"`
		Name     string    `json:"Name"`
		Size     int64     `json:"Size"`
		ModTime  time.Time `json:"ModTime"`
		IsDir    bool      `json:"IsDir"`
		MimeType string    `json:"MimeType"`
	}

	if err := json.Unmarshal(output, &rcloneFiles); err != nil {
		span.RecordError(err)
		return nil, &StorageError{
			Op:      "list",
			Path:    prefix,
			Backend: "rclone",
			Err:     fmt.Errorf("failed to parse rclone output: %w", err),
		}
	}

	files := make([]FileInfo, len(rcloneFiles))
	for i, f := range rcloneFiles {
		files[i] = FileInfo{
			Path:        f.Path,
			Size:        f.Size,
			ModTime:     f.ModTime,
			IsDir:       f.IsDir,
			ContentType: f.MimeType,
		}
	}

	return files, nil
}

// GetMetadata returns metadata about a file
func (r *RcloneBackend) GetMetadata(ctx context.Context, path string) (*FileMetadata, error) {
	ctx, span := tracer.Start(ctx, "rclone.GetMetadata",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	remotePath := r.getRemotePath(path)

	cmd := r.buildCommand(ctx, "lsjson", remotePath)
	output, err := cmd.Output()
	if err != nil {
		span.RecordError(err)
		return nil, &StorageError{
			Op:      "get metadata",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("rclone lsjson failed: %w", err),
		}
	}

	var rcloneFiles []struct {
		Path     string            `json:"Path"`
		Size     int64             `json:"Size"`
		ModTime  time.Time         `json:"ModTime"`
		MimeType string            `json:"MimeType"`
		Hashes   map[string]string `json:"Hashes"`
	}

	if err := json.Unmarshal(output, &rcloneFiles); err != nil {
		span.RecordError(err)
		return nil, &StorageError{
			Op:      "get metadata",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("failed to parse rclone output: %w", err),
		}
	}

	if len(rcloneFiles) == 0 {
		return nil, &StorageError{
			Op:      "get metadata",
			Path:    path,
			Backend: "rclone",
			Err:     fmt.Errorf("file not found"),
		}
	}

	f := rcloneFiles[0]
	metadata := &FileMetadata{
		Path:        f.Path,
		Size:        f.Size,
		ModTime:     f.ModTime,
		ContentType: f.MimeType,
		Metadata:    make(map[string]string),
	}

	// Add hashes to metadata
	for hashType, hash := range f.Hashes {
		metadata.Metadata[hashType] = hash
		if hashType == "md5" {
			metadata.Checksum = hash
		}
	}

	return metadata, nil
}

// Close closes the rclone backend
func (r *RcloneBackend) Close() error {
	// Nothing to close for rclone
	return nil
}

// getRemotePath constructs the full remote path
func (r *RcloneBackend) getRemotePath(path string) string {
	// Remove leading slash if present
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		return r.remote
	}

	return r.remote + "/" + path
}
