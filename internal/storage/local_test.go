package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalBackend_UploadAndDownload(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	backend, err := NewLocalBackend(LocalConfig{
		RootPath: tempDir,
	})
	require.NoError(t, err)

	ctx := context.Background()
	testPath := "test/file.txt"
	testContent := []byte("Hello, World!")

	// Test Upload
	err = backend.UploadBytes(ctx, testPath, testContent, "text/plain")
	assert.NoError(t, err)

	// Verify file exists on disk
	fullPath := filepath.Join(tempDir, testPath)
	assert.FileExists(t, fullPath)

	// Test Download
	reader, err := backend.Download(ctx, testPath)
	require.NoError(t, err)
	defer reader.Close()

	downloaded, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, testContent, downloaded)
}

func TestLocalBackend_Exists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	backend, err := NewLocalBackend(LocalConfig{
		RootPath: tempDir,
	})
	require.NoError(t, err)

	ctx := context.Background()
	testPath := "test/exists.txt"

	// Check non-existent file
	exists, err := backend.Exists(ctx, testPath)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Upload file
	err = backend.UploadBytes(ctx, testPath, []byte("test"), "text/plain")
	require.NoError(t, err)

	// Check existing file
	exists, err = backend.Exists(ctx, testPath)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestLocalBackend_Delete(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	backend, err := NewLocalBackend(LocalConfig{
		RootPath: tempDir,
	})
	require.NoError(t, err)

	ctx := context.Background()
	testPath := "test/delete.txt"

	// Upload file
	err = backend.UploadBytes(ctx, testPath, []byte("delete me"), "text/plain")
	require.NoError(t, err)

	// Verify it exists
	exists, err := backend.Exists(ctx, testPath)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete file
	err = backend.Delete(ctx, testPath)
	assert.NoError(t, err)

	// Verify it no longer exists
	exists, err = backend.Exists(ctx, testPath)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestLocalBackend_GetSize(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	backend, err := NewLocalBackend(LocalConfig{
		RootPath: tempDir,
	})
	require.NoError(t, err)

	ctx := context.Background()
	testPath := "test/sized.txt"
	testContent := []byte("This is a test file with known size")

	// Upload file
	err = backend.UploadBytes(ctx, testPath, testContent, "text/plain")
	require.NoError(t, err)

	// Get size
	size, err := backend.GetSize(ctx, testPath)
	assert.NoError(t, err)
	assert.Equal(t, int64(len(testContent)), size)
}

func TestLocalBackend_CopyAndMove(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	backend, err := NewLocalBackend(LocalConfig{
		RootPath: tempDir,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Copy", func(t *testing.T) {
		srcPath := "test/source.txt"
		dstPath := "test/destination.txt"
		testContent := []byte("content to copy")

		// Upload source file
		err := backend.UploadBytes(ctx, srcPath, testContent, "text/plain")
		require.NoError(t, err)

		// Copy file
		err = backend.Copy(ctx, srcPath, dstPath)
		assert.NoError(t, err)

		// Verify both files exist
		srcExists, _ := backend.Exists(ctx, srcPath)
		assert.True(t, srcExists)

		dstExists, _ := backend.Exists(ctx, dstPath)
		assert.True(t, dstExists)

		// Verify content matches
		reader, err := backend.Download(ctx, dstPath)
		require.NoError(t, err)
		defer reader.Close()

		copied, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, testContent, copied)
	})

	t.Run("Move", func(t *testing.T) {
		srcPath := "test/tomove.txt"
		dstPath := "test/moved.txt"
		testContent := []byte("content to move")

		// Upload source file
		err := backend.UploadBytes(ctx, srcPath, testContent, "text/plain")
		require.NoError(t, err)

		// Move file
		err = backend.Move(ctx, srcPath, dstPath)
		assert.NoError(t, err)

		// Verify source no longer exists
		srcExists, _ := backend.Exists(ctx, srcPath)
		assert.False(t, srcExists)

		// Verify destination exists
		dstExists, _ := backend.Exists(ctx, dstPath)
		assert.True(t, dstExists)
	})
}

func TestLocalBackend_List(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	backend, err := NewLocalBackend(LocalConfig{
		RootPath: tempDir,
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Create test files
	files := map[string][]byte{
		"list/file1.txt":        []byte("content1"),
		"list/file2.txt":        []byte("content2"),
		"list/subdir/file3.txt": []byte("content3"),
		"other/file4.txt":       []byte("content4"),
	}

	for path, content := range files {
		err := backend.UploadBytes(ctx, path, content, "text/plain")
		require.NoError(t, err)
	}

	// List non-recursive
	items, err := backend.List(ctx, "list/", false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 2) // At least file1.txt and file2.txt

	// List recursive
	items, err = backend.List(ctx, "list/", true)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 3) // Should include subdirectory files
}

func TestLocalBackend_UploadStream(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	backend, err := NewLocalBackend(LocalConfig{
		RootPath: tempDir,
	})
	require.NoError(t, err)

	ctx := context.Background()
	testPath := "test/stream.txt"
	testContent := []byte("Stream upload test")

	// Upload using io.Reader
	reader := bytes.NewReader(testContent)
	err = backend.Upload(ctx, testPath, reader, int64(len(testContent)), "text/plain")
	assert.NoError(t, err)

	// Verify content
	downloaded, err := backend.Download(ctx, testPath)
	require.NoError(t, err)
	defer downloaded.Close()

	content, err := io.ReadAll(downloaded)
	require.NoError(t, err)
	assert.Equal(t, testContent, content)
}

func TestLocalBackend_RejectsTraversal(t *testing.T) {
	parentDir := t.TempDir()
	rootDir := filepath.Join(parentDir, "storage")
	backend, err := NewLocalBackend(LocalConfig{RootPath: rootDir})
	require.NoError(t, err)

	outsidePath := filepath.Join(parentDir, "outside.txt")
	err = backend.UploadBytes(context.Background(), "../outside.txt", []byte("escaped"), "text/plain")
	require.ErrorContains(t, err, "escapes root")
	assert.NoFileExists(t, outsidePath)

	require.NoError(t, os.WriteFile(outsidePath, []byte("keep"), 0o600))
	err = backend.Delete(context.Background(), "../outside.txt")
	require.ErrorContains(t, err, "escapes root")
	assert.FileExists(t, outsidePath)

	err = backend.Delete(context.Background(), "")
	require.ErrorContains(t, err, "resolves to root")
	assert.DirExists(t, rootDir)
}

func TestLocalBackend_RejectsSymlinksBelowRoot(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	backend, err := NewLocalBackend(LocalConfig{RootPath: rootDir})
	require.NoError(t, err)

	require.NoError(t, os.Symlink(outsideDir, filepath.Join(rootDir, "escape")))
	err = backend.UploadBytes(context.Background(), "escape/file.txt", []byte("escaped"), "text/plain")
	require.ErrorContains(t, err, "symlink path component is not allowed")
	assert.NoFileExists(t, filepath.Join(outsideDir, "file.txt"))

	outsideFile := filepath.Join(outsideDir, "secret.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(outsideFile, filepath.Join(rootDir, "secret.txt")))

	_, err = backend.Download(context.Background(), "secret.txt")
	require.ErrorContains(t, err, "symlink path component is not allowed")
	err = backend.Delete(context.Background(), "secret.txt")
	require.ErrorContains(t, err, "symlink path component is not allowed")
	assert.FileExists(t, outsideFile)

	_, err = backend.List(context.Background(), "", false)
	require.ErrorContains(t, err, "symlink path component is not allowed")
}

func TestLocalBackend_UploadFailureIsAtomicAndCleansTemporaryFile(t *testing.T) {
	rootDir := t.TempDir()
	backend, err := NewLocalBackend(LocalConfig{RootPath: rootDir})
	require.NoError(t, err)

	ctx := context.Background()
	path := "assets/file.txt"
	require.NoError(t, backend.UploadBytes(ctx, path, []byte("original"), "text/plain"))

	writeErr := errors.New("reader failed")
	err = backend.Upload(ctx, path, &failingReader{
		reader: strings.NewReader("partial replacement"),
		err:    writeErr,
	}, -1, "text/plain")
	require.ErrorIs(t, err, writeErr)

	content, err := os.ReadFile(filepath.Join(rootDir, path))
	require.NoError(t, err)
	assert.Equal(t, []byte("original"), content)
	assertNoUploadTemporaryFiles(t, filepath.Join(rootDir, "assets"))
}

func TestLocalBackend_UploadVerifiesDeclaredSize(t *testing.T) {
	rootDir := t.TempDir()
	backend, err := NewLocalBackend(LocalConfig{RootPath: rootDir})
	require.NoError(t, err)

	ctx := context.Background()
	for _, tt := range []struct {
		name string
		data string
		size int64
	}{
		{name: "short reader", data: "abc", size: 4},
		{name: "long reader", data: "abcde", size: 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("assets", strings.ReplaceAll(tt.name, " ", "-")+".txt")
			err := backend.Upload(ctx, path, strings.NewReader(tt.data), tt.size, "text/plain")
			require.ErrorContains(t, err, "uploaded size mismatch")
			assert.NoFileExists(t, filepath.Join(rootDir, path))
			assertNoUploadTemporaryFiles(t, filepath.Join(rootDir, "assets"))
		})
	}
}

type failingReader struct {
	reader io.Reader
	err    error
}

func (r *failingReader) Read(data []byte) (int, error) {
	n, err := r.reader.Read(data)
	if errors.Is(err, io.EOF) {
		return 0, r.err
	}
	return n, err
}

func assertNoUploadTemporaryFiles(t *testing.T, dir string) {
	t.Helper()

	tempFiles, err := filepath.Glob(filepath.Join(dir, ".immich-go-upload-*"))
	require.NoError(t, err)
	assert.Empty(t, tempFiles)
}

func TestLocalBackend_GetPublicURL(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	backend, err := NewLocalBackend(LocalConfig{
		RootPath: tempDir,
	})
	require.NoError(t, err)

	// LocalBackend doesn't support public URLs
	url, err := backend.GetPublicURL(context.Background(), "test/file.txt")
	assert.Error(t, err)
	assert.Empty(t, url)
	assert.Contains(t, err.Error(), "not supported")
}

func TestLocalBackend_PresignedURLs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	backend, err := NewLocalBackend(LocalConfig{
		RootPath: tempDir,
	})
	require.NoError(t, err)

	// Local backend should not support presigned URLs
	assert.False(t, backend.SupportsPresignedURLs())
}
