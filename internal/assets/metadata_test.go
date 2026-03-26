package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestJPEG creates a synthetic JPEG image for testing purposes.
func createTestJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		panic(fmt.Sprintf("createTestJPEG: failed to encode: %v", err))
	}
	return buf.Bytes()
}

// TestExtractMetadata_PlainJPEG verifies that ExtractMetadata succeeds on a
// plain JPEG with no embedded EXIF data and populates only the basic fields.
func TestExtractMetadata_PlainJPEG(t *testing.T) {
	imgBytes := createTestJPEG(64, 48)

	extractor := NewMetadataExtractor()
	ctx := context.Background()

	const filename = "test.jpg"
	const contentType = "image/jpeg"
	size := int64(len(imgBytes))

	meta, err := extractor.ExtractMetadata(ctx, bytes.NewReader(imgBytes), filename, contentType, size)
	require.NoError(t, err, "ExtractMetadata must not return an error for a plain JPEG")
	require.NotNil(t, meta)

	// Basic fields must be populated.
	assert.Equal(t, filename, meta.Filename)
	assert.Equal(t, contentType, meta.ContentType)
	assert.Equal(t, size, meta.Size)
	assert.False(t, meta.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, meta.ModifiedAt.IsZero(), "ModifiedAt should be set")

	// EXIF fields must remain nil because the image has no EXIF data.
	assert.Nil(t, meta.Make, "Make should be nil when no EXIF is present")
	assert.Nil(t, meta.Model, "Model should be nil when no EXIF is present")
	assert.Nil(t, meta.DateTaken, "DateTaken should be nil when no EXIF is present")
	assert.Nil(t, meta.Width, "Width should be nil when no EXIF PixelXDimension tag is present")
	assert.Nil(t, meta.Height, "Height should be nil when no EXIF PixelYDimension tag is present")
	assert.Nil(t, meta.LensModel, "LensModel should be nil when no EXIF is present")
	assert.Nil(t, meta.FNumber, "FNumber should be nil when no EXIF is present")
	assert.Nil(t, meta.FocalLength, "FocalLength should be nil when no EXIF is present")
	assert.Nil(t, meta.ISO, "ISO should be nil when no EXIF is present")
	assert.Nil(t, meta.ExposureTime, "ExposureTime should be nil when no EXIF is present")
	assert.Nil(t, meta.Latitude, "Latitude should be nil when no EXIF is present")
	assert.Nil(t, meta.Longitude, "Longitude should be nil when no EXIF is present")
}

// TestExtractMetadata_EmptyReader verifies that an empty reader does not cause
// ExtractMetadata to return an error (missing EXIF is not fatal).
func TestExtractMetadata_EmptyReader(t *testing.T) {
	extractor := NewMetadataExtractor()
	ctx := context.Background()

	meta, err := extractor.ExtractMetadata(ctx, strings.NewReader(""), "empty.jpg", "image/jpeg", 0)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, "empty.jpg", meta.Filename)
	assert.Equal(t, int64(0), meta.Size)

	// No EXIF fields should be set.
	assert.Nil(t, meta.Make)
	assert.Nil(t, meta.Model)
	assert.Nil(t, meta.DateTaken)
}

// TestExtractMetadata_VideoContentType verifies that video files are handled
// without error even when the content is not a real video stream (ffprobe
// simply won't be invoked or will gracefully fail).
func TestExtractMetadata_VideoContentType(t *testing.T) {
	extractor := NewMetadataExtractor()
	ctx := context.Background()

	// Use arbitrary bytes – the extractor gracefully handles ffprobe failures.
	meta, err := extractor.ExtractMetadata(ctx, strings.NewReader("fake video data"), "clip.mp4", "video/mp4", 15)
	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.Equal(t, "clip.mp4", meta.Filename)
	assert.Equal(t, "video/mp4", meta.ContentType)
	assert.Equal(t, int64(15), meta.Size)
}

// TestExtractMetadata_UnknownContentType verifies that unknown content types
// are handled without error and return the basic fields.
func TestExtractMetadata_UnknownContentType(t *testing.T) {
	extractor := NewMetadataExtractor()
	ctx := context.Background()

	meta, err := extractor.ExtractMetadata(ctx, strings.NewReader("binary blob"), "archive.bin", "application/octet-stream", 11)
	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.Equal(t, "archive.bin", meta.Filename)
	assert.Equal(t, "application/octet-stream", meta.ContentType)
	assert.Equal(t, int64(11), meta.Size)
}

// TestGetAssetTypeFromContentType verifies that MIME types are classified
// correctly into the four asset type buckets.
func TestGetAssetTypeFromContentType(t *testing.T) {
	extractor := NewMetadataExtractor()

	tests := []struct {
		contentType string
		want        AssetType
	}{
		// Image variants
		{"image/jpeg", AssetTypeImage},
		{"image/png", AssetTypeImage},
		{"image/gif", AssetTypeImage},
		{"image/webp", AssetTypeImage},
		{"image/heic", AssetTypeImage},
		{"IMAGE/JPEG", AssetTypeImage}, // case-insensitive

		// Video variants
		{"video/mp4", AssetTypeVideo},
		{"video/quicktime", AssetTypeVideo},
		{"video/x-msvideo", AssetTypeVideo},
		{"VIDEO/MP4", AssetTypeVideo}, // case-insensitive

		// Audio variants
		{"audio/mpeg", AssetTypeAudio},
		{"audio/ogg", AssetTypeAudio},
		{"AUDIO/MPEG", AssetTypeAudio}, // case-insensitive

		// Anything else
		{"application/octet-stream", AssetTypeOther},
		{"text/plain", AssetTypeOther},
		{"", AssetTypeOther},
	}

	for _, tc := range tests {
		t.Run(tc.contentType, func(t *testing.T) {
			got := extractor.getAssetTypeFromContentType(tc.contentType)
			assert.Equal(t, tc.want, got, "unexpected asset type for content type %q", tc.contentType)
		})
	}
}

// TestParseISO6709Location verifies parsing of ISO 6709 location strings.
func TestParseISO6709Location(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLat float64
		wantLon float64
	}{
		{
			name:    "San Francisco",
			input:   "+37.7749-122.4194/",
			wantLat: 37.7749,
			wantLon: -122.4194,
		},
		{
			name:    "without trailing slash",
			input:   "+37.7749-122.4194",
			wantLat: 37.7749,
			wantLon: -122.4194,
		},
		{
			name:    "negative latitude positive longitude (e.g. Sydney area)",
			input:   "-33.8688+151.2093/",
			wantLat: -33.8688,
			wantLon: 151.2093,
		},
		{
			name:    "both positive (e.g. Berlin area)",
			input:   "+52.5200+013.4050/",
			wantLat: 52.5200,
			wantLon: 13.4050,
		},
		{
			name:    "empty string returns zeros",
			input:   "",
			wantLat: 0,
			wantLon: 0,
		},
		{
			name:    "missing longitude sign returns zeros",
			input:   "+37.7749",
			wantLat: 0,
			wantLon: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotLat, gotLon := parseISO6709Location(tc.input)
			assert.InDelta(t, tc.wantLat, gotLat, 1e-6, "latitude mismatch")
			assert.InDelta(t, tc.wantLon, gotLon, 1e-6, "longitude mismatch")
		})
	}
}

// TestCalculateChecksum verifies that CalculateChecksum returns the expected
// SHA-256 hex digest for a known byte sequence.
func TestCalculateChecksum(t *testing.T) {
	extractor := NewMetadataExtractor()
	ctx := context.Background()

	data := []byte("hello, immich")
	expected := fmt.Sprintf("%x", sha256.Sum256(data))

	checksum, err := extractor.CalculateChecksum(ctx, bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, expected, checksum)
}

// TestCalculateChecksum_Empty verifies that the checksum of an empty reader
// equals the SHA-256 of an empty byte slice.
func TestCalculateChecksum_Empty(t *testing.T) {
	extractor := NewMetadataExtractor()
	ctx := context.Background()

	expected := fmt.Sprintf("%x", sha256.Sum256([]byte{}))

	checksum, err := extractor.CalculateChecksum(ctx, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, expected, checksum)
}

// TestCalculateChecksum_LargeInput verifies that the checksum is consistent
// for larger inputs (the JPEG created in other tests).
func TestCalculateChecksum_LargeInput(t *testing.T) {
	extractor := NewMetadataExtractor()
	ctx := context.Background()

	data := createTestJPEG(128, 128)
	expected := fmt.Sprintf("%x", sha256.Sum256(data))

	checksum, err := extractor.CalculateChecksum(ctx, bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, expected, checksum)
}
