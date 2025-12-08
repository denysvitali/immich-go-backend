package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
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
