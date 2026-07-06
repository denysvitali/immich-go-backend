package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceUploadPassesKnownReaderSize(t *testing.T) {
	backend := &recordingStorageBackend{}
	service := &Service{backend: backend}
	content := []byte("sized upload")

	err := service.Upload(context.Background(), "assets/file.txt", bytes.NewReader(content), "text/plain")
	require.NoError(t, err)

	assert.Equal(t, "assets/file.txt", backend.uploadPath)
	assert.Equal(t, "text/plain", backend.uploadContentType)
	assert.Equal(t, int64(len(content)), backend.uploadSize)
	assert.Equal(t, content, backend.uploadData)
}

func TestServiceUploadPassesRemainingSeekableReaderSize(t *testing.T) {
	backend := &recordingStorageBackend{}
	service := &Service{backend: backend}
	reader := &seekOnlyReader{reader: bytes.NewReader([]byte("skip-remaining"))}
	_, err := reader.Seek(5, io.SeekStart)
	require.NoError(t, err)

	err = service.Upload(context.Background(), "assets/file.txt", reader, "text/plain")
	require.NoError(t, err)

	assert.Equal(t, int64(len("remaining")), backend.uploadSize)
	assert.Equal(t, []byte("remaining"), backend.uploadData)
}

func TestServiceUploadUsesUnknownSizeForStreamingReader(t *testing.T) {
	backend := &recordingStorageBackend{}
	service := &Service{backend: backend}
	content := []byte("streamed upload")
	reader := io.LimitReader(bytes.NewReader(content), int64(len(content)))

	err := service.Upload(context.Background(), "assets/file.txt", reader, "text/plain")
	require.NoError(t, err)

	assert.Equal(t, int64(-1), backend.uploadSize)
	assert.Equal(t, content, backend.uploadData)
}

type recordingStorageBackend struct {
	uploadPath        string
	uploadSize        int64
	uploadContentType string
	uploadData        []byte
}

type seekOnlyReader struct {
	reader *bytes.Reader
}

func (r *seekOnlyReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *seekOnlyReader) Seek(offset int64, whence int) (int64, error) {
	return r.reader.Seek(offset, whence)
}

func (b *recordingStorageBackend) Upload(_ context.Context, path string, reader io.Reader, size int64, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	b.uploadPath = path
	b.uploadSize = size
	b.uploadContentType = contentType
	b.uploadData = data
	return nil
}

func (b *recordingStorageBackend) UploadBytes(context.Context, string, []byte, string) error {
	return nil
}

func (b *recordingStorageBackend) Download(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (b *recordingStorageBackend) Delete(context.Context, string) error {
	return nil
}

func (b *recordingStorageBackend) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (b *recordingStorageBackend) GetSize(context.Context, string) (int64, error) {
	return 0, nil
}

func (b *recordingStorageBackend) GetPresignedUploadURL(context.Context, string, string, time.Duration) (*PresignedURL, error) {
	return nil, ErrPresignedURLsNotSupported
}

func (b *recordingStorageBackend) GetPresignedDownloadURL(context.Context, string, time.Duration) (*PresignedURL, error) {
	return nil, ErrPresignedURLsNotSupported
}

func (b *recordingStorageBackend) SupportsPresignedURLs() bool {
	return false
}

func (b *recordingStorageBackend) GetPublicURL(context.Context, string) (string, error) {
	return "", ErrPublicURLNotSupported
}

func (b *recordingStorageBackend) Copy(context.Context, string, string) error {
	return nil
}

func (b *recordingStorageBackend) Move(context.Context, string, string) error {
	return nil
}

func (b *recordingStorageBackend) List(context.Context, string, bool) ([]FileInfo, error) {
	return nil, nil
}

func (b *recordingStorageBackend) GetMetadata(context.Context, string) (*FileMetadata, error) {
	return nil, nil
}

func (b *recordingStorageBackend) Close() error {
	return nil
}
