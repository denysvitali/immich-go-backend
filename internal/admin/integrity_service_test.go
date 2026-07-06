package admin

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // Immich uses SHA-1 asset checksums; not for crypto.
	"encoding/hex"
	"fmt"
	"io"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeIntegrityStorage struct {
	files map[string][]byte
}

func (s fakeIntegrityStorage) Exists(_ context.Context, path string) (bool, error) {
	_, ok := s.files[path]
	return ok, nil
}

func (s fakeIntegrityStorage) Download(_ context.Context, path string) (io.ReadCloser, error) {
	data, ok := s.files[path]
	if !ok {
		return nil, fmt.Errorf("file not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s fakeIntegrityStorage) List(_ context.Context, _ string, _ bool) ([]storage.FileInfo, error) {
	items := make([]storage.FileInfo, 0, len(s.files))
	for path := range s.files {
		items = append(items, storage.FileInfo{Path: path})
	}
	return items, nil
}

func TestBuildIntegrityReportDetectsMissingChecksumAndUntrackedFiles(t *testing.T) {
	ctx := context.Background()
	matchPayload := []byte("matching content")
	mismatchPayload := []byte("actual content")

	report, err := buildIntegrityReport(ctx, []integrityOriginalAsset{
		{
			ID:       "00000000-0000-4000-8000-000000000001",
			Path:     "library/match.jpg",
			Checksum: []byte(sha1Hex(matchPayload)),
		},
		{
			ID:       "00000000-0000-4000-8000-000000000002",
			Path:     "library/missing.jpg",
			Checksum: []byte("missing-checksum"),
		},
		{
			ID:       "00000000-0000-4000-8000-000000000003",
			Path:     "library/mismatch.jpg",
			Checksum: []byte("wrong-checksum"),
		},
	}, []string{
		"library/match.jpg",
		"library/missing.jpg",
		"library/mismatch.jpg",
		"thumbs/tracked.webp",
	}, fakeIntegrityStorage{files: map[string][]byte{
		".immich":                       []byte("install marker"),
		"library/match.jpg":             matchPayload,
		"library/mismatch.jpg":          mismatchPayload,
		"thumbs/tracked.webp":           []byte("tracked generated file"),
		"upload/.immich-go-write-check": []byte("ok"),
		"orphan/untracked.jpg":          []byte("orphan"),
	}})
	require.NoError(t, err)

	summary := report.summary()
	assert.EqualValues(t, 1, summary[integrityTypeMissingFile])
	assert.EqualValues(t, 1, summary[integrityTypeChecksumMismatch])
	assert.EqualValues(t, 1, summary[integrityTypeUntrackedFile])

	missing := report.itemsByType(integrityTypeMissingFile)
	require.Len(t, missing, 1)
	assert.Equal(t, "00000000-0000-4000-8000-000000000002", missing[0].ID)
	assert.Equal(t, "library/missing.jpg", missing[0].Path)

	mismatch := report.itemsByType(integrityTypeChecksumMismatch)
	require.Len(t, mismatch, 1)
	assert.Equal(t, "00000000-0000-4000-8000-000000000003", mismatch[0].ID)
	assert.Equal(t, "library/mismatch.jpg", mismatch[0].Path)

	untracked := report.itemsByType(integrityTypeUntrackedFile)
	require.Len(t, untracked, 1)
	assert.Equal(t, "orphan/untracked.jpg", untracked[0].Path)
	assert.NotEmpty(t, untracked[0].ID)

	csv := string(report.csvData(integrityTypeUntrackedFile))
	assert.Contains(t, csv, "id,type,path\n")
	assert.Contains(t, csv, "orphan/untracked.jpg")
	assert.NotContains(t, csv, "thumbs/tracked.webp")
	assert.NotContains(t, csv, ".immich")
	assert.NotContains(t, csv, ".immich-go-write-check")
}

func TestBuildIntegrityReportAcceptsRawChecksumBytes(t *testing.T) {
	ctx := context.Background()
	payload := []byte("raw checksum content")
	sum := sha1.Sum(payload) //nolint:gosec // Immich uses SHA-1 asset checksums; not for crypto.

	report, err := buildIntegrityReport(ctx, []integrityOriginalAsset{
		{
			ID:       "00000000-0000-4000-8000-000000000001",
			Path:     "library/raw.jpg",
			Checksum: sum[:],
		},
	}, []string{"library/raw.jpg"}, fakeIntegrityStorage{files: map[string][]byte{
		"library/raw.jpg": payload,
	}})
	require.NoError(t, err)

	summary := report.summary()
	assert.EqualValues(t, 0, summary[integrityTypeMissingFile])
	assert.EqualValues(t, 0, summary[integrityTypeChecksumMismatch])
	assert.EqualValues(t, 0, summary[integrityTypeUntrackedFile])
}

func TestPaginateIntegrityItems(t *testing.T) {
	report := newIntegrityReport([]integrityReportItem{
		{ID: "1", Type: integrityTypeMissingFile, Path: "a.jpg"},
		{ID: "2", Type: integrityTypeMissingFile, Path: "b.jpg"},
		{ID: "3", Type: integrityTypeMissingFile, Path: "c.jpg"},
	})

	first, next, err := paginateIntegrityItems(report.itemsByType(integrityTypeMissingFile), "", 2)
	require.NoError(t, err)
	require.NotNil(t, next)
	assert.Equal(t, "2", *next)
	assert.Equal(t, []string{"a.jpg", "b.jpg"}, integrityItemPaths(first))

	second, next, err := paginateIntegrityItems(report.itemsByType(integrityTypeMissingFile), *next, 2)
	require.NoError(t, err)
	assert.Nil(t, next)
	assert.Equal(t, []string{"c.jpg"}, integrityItemPaths(second))

	_, _, err = paginateIntegrityItems(report.itemsByType(integrityTypeMissingFile), "not-a-number", 2)
	require.Error(t, err)
}

func sha1Hex(data []byte) string {
	sum := sha1.Sum(data) //nolint:gosec // Immich uses SHA-1 asset checksums; not for crypto.
	return hex.EncodeToString(sum[:])
}

func integrityItemPaths(items []integrityReportItem) []string {
	paths := make([]string, 0, len(items))
	for _, item := range items {
		paths = append(paths, item.Path)
	}
	return paths
}
