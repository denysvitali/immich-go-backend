package assets

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestPNG creates a synthetic PNG image in memory for use in tests.
func createTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 200, 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img) //nolint:errcheck
	return buf.Bytes()
}

func TestGenerateThumbnails_FromJPEG(t *testing.T) {
	g := NewThumbnailGenerator()

	// Create a large enough JPEG so all thumbnails are reduced in dimension.
	const origW, origH = 2000, 1500
	original := createTestJPEG(origW, origH)
	require.NotEmpty(t, original)

	reader := bytes.NewReader(original)
	thumbnails, err := g.GenerateThumbnails(context.Background(), reader, "test.jpg")
	require.NoError(t, err)

	// All three thumbnail types must be present.
	require.Contains(t, thumbnails, ThumbnailTypePreview, "preview thumbnail missing")
	require.Contains(t, thumbnails, ThumbnailTypeWebp, "webp thumbnail missing")
	require.Contains(t, thumbnails, ThumbnailTypeThumb, "thumb thumbnail missing")

	// Expected max dimensions per thumbnail type (from NewThumbnailGenerator config).
	maxDims := map[ThumbnailType][2]int{
		ThumbnailTypePreview: {1440, 1440},
		ThumbnailTypeWebp:    {250, 250},
		ThumbnailTypeThumb:   {160, 160},
	}

	for _, thumbType := range []ThumbnailType{ThumbnailTypePreview, ThumbnailTypeWebp, ThumbnailTypeThumb} {
		data := thumbnails[thumbType]

		// Data must be non-empty.
		assert.NotEmpty(t, data, "thumbnail %s data is empty", thumbType)

		// Must start with JPEG SOI marker (0xFF 0xD8).
		require.GreaterOrEqual(t, len(data), 2, "thumbnail %s too short to check header", thumbType)
		assert.Equal(t, byte(0xFF), data[0], "thumbnail %s missing JPEG 0xFF header byte", thumbType)
		assert.Equal(t, byte(0xD8), data[1], "thumbnail %s missing JPEG 0xD8 header byte", thumbType)

		// Decode thumbnail and check pixel dimensions.
		thumbImg, _, err := image.Decode(bytes.NewReader(data))
		require.NoError(t, err, "thumbnail %s is not a valid image", thumbType)

		maxW, maxH := maxDims[thumbType][0], maxDims[thumbType][1]
		gotW := thumbImg.Bounds().Dx()
		gotH := thumbImg.Bounds().Dy()

		assert.LessOrEqual(t, gotW, maxW, "thumbnail %s width %d exceeds max %d", thumbType, gotW, maxW)
		assert.LessOrEqual(t, gotH, maxH, "thumbnail %s height %d exceeds max %d", thumbType, gotH, maxH)

		// All thumbnails must be smaller in dimension than the original.
		assert.Less(t, gotW, origW, "thumbnail %s width should be less than original", thumbType)
		assert.Less(t, gotH, origH, "thumbnail %s height should be less than original", thumbType)

		// Byte size must also be smaller (JPEG->JPEG at lower quality/size is always smaller).
		assert.Less(t, len(data), len(original),
			"thumbnail %s (%d bytes) is not smaller than original (%d bytes)", thumbType, len(data), len(original))
	}
}

func TestGenerateThumbnails_FromPNG(t *testing.T) {
	g := NewThumbnailGenerator()

	const origW, origH = 1800, 1200
	original := createTestPNG(origW, origH)
	require.NotEmpty(t, original)

	reader := bytes.NewReader(original)
	thumbnails, err := g.GenerateThumbnails(context.Background(), reader, "test.png")
	require.NoError(t, err)

	// All three thumbnail types must be present.
	require.Contains(t, thumbnails, ThumbnailTypePreview, "preview thumbnail missing")
	require.Contains(t, thumbnails, ThumbnailTypeWebp, "webp thumbnail missing")
	require.Contains(t, thumbnails, ThumbnailTypeThumb, "thumb thumbnail missing")

	// Expected max dimensions per thumbnail type (from NewThumbnailGenerator config).
	maxDims := map[ThumbnailType][2]int{
		ThumbnailTypePreview: {1440, 1440},
		ThumbnailTypeWebp:    {250, 250},
		ThumbnailTypeThumb:   {160, 160},
	}

	for _, thumbType := range []ThumbnailType{ThumbnailTypePreview, ThumbnailTypeWebp, ThumbnailTypeThumb} {
		data := thumbnails[thumbType]

		assert.NotEmpty(t, data, "thumbnail %s data is empty", thumbType)

		// All output formats fall back to JPEG encoding.
		require.GreaterOrEqual(t, len(data), 2, "thumbnail %s too short to check header", thumbType)
		assert.Equal(t, byte(0xFF), data[0], "thumbnail %s missing JPEG 0xFF header byte", thumbType)
		assert.Equal(t, byte(0xD8), data[1], "thumbnail %s missing JPEG 0xD8 header byte", thumbType)

		// Decode the thumbnail to verify its pixel dimensions are within the max bounds.
		thumbImg, _, err := image.Decode(bytes.NewReader(data))
		require.NoError(t, err, "thumbnail %s is not a valid image", thumbType)

		maxW, maxH := maxDims[thumbType][0], maxDims[thumbType][1]
		gotW := thumbImg.Bounds().Dx()
		gotH := thumbImg.Bounds().Dy()

		assert.LessOrEqual(t, gotW, maxW,
			"thumbnail %s width %d exceeds max %d", thumbType, gotW, maxW)
		assert.LessOrEqual(t, gotH, maxH,
			"thumbnail %s height %d exceeds max %d", thumbType, gotH, maxH)

		// Dimensions must be smaller than the original (since origW/origH both exceed all maxes).
		assert.Less(t, gotW, origW, "thumbnail %s width should be less than original", thumbType)
		assert.Less(t, gotH, origH, "thumbnail %s height should be less than original", thumbType)
	}
}

func TestGenerateThumbnails_InvalidInput(t *testing.T) {
	g := NewThumbnailGenerator()

	reader := bytes.NewReader([]byte("this is not an image"))
	thumbnails, err := g.GenerateThumbnails(context.Background(), reader, "bad.jpg")

	assert.Error(t, err)
	assert.Nil(t, thumbnails)
}

func TestGetThumbnailPath(t *testing.T) {
	g := NewThumbnailGenerator()

	tests := []struct {
		name         string
		originalPath string
		thumbType    ThumbnailType
		wantSuffix   string // expected path suffix
		wantDir      string // expected parent directory name
	}{
		{
			name:         "preview thumbnail for JPEG",
			originalPath: "/uploads/users/abc/photo.jpg",
			thumbType:    ThumbnailTypePreview,
			wantDir:      "thumbnails",
			wantSuffix:   "photo_preview.jpg",
		},
		{
			name:         "webp thumbnail for JPEG",
			originalPath: "/uploads/users/abc/photo.jpg",
			thumbType:    ThumbnailTypeWebp,
			wantDir:      "thumbnails",
			wantSuffix:   "photo_webp.webp",
		},
		{
			name:         "thumb thumbnail for JPEG",
			originalPath: "/uploads/users/abc/photo.jpg",
			thumbType:    ThumbnailTypeThumb,
			wantDir:      "thumbnails",
			wantSuffix:   "photo_thumb.jpg",
		},
		{
			name:         "preview thumbnail for PNG",
			originalPath: "images/vacation.png",
			thumbType:    ThumbnailTypePreview,
			wantDir:      "thumbnails",
			wantSuffix:   "vacation_preview.jpg",
		},
		{
			name:         "path with nested directories",
			originalPath: "/a/b/c/d/file.jpeg",
			thumbType:    ThumbnailTypeThumb,
			wantDir:      "thumbnails",
			wantSuffix:   "file_thumb.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.GetThumbnailPath(tt.originalPath, tt.thumbType)

			// The immediate parent of the thumbnail file must be "thumbnails".
			assert.Equal(t, tt.wantDir, filepath.Base(filepath.Dir(got)))

			// The filename must match the expected suffix.
			assert.Equal(t, tt.wantSuffix, filepath.Base(got))
		})
	}
}

func TestCanGenerateThumbnail(t *testing.T) {
	g := NewThumbnailGenerator()

	supported := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/bmp",
		"image/tiff",
		"image/webp",
		// Case insensitivity.
		"IMAGE/JPEG",
		"Image/PNG",
	}

	for _, ct := range supported {
		assert.True(t, g.CanGenerateThumbnail(ct), "expected %s to be supported", ct)
	}

	unsupported := []string{
		"video/mp4",
		"application/pdf",
		"text/plain",
		"audio/mpeg",
		"",
	}

	for _, ct := range unsupported {
		assert.False(t, g.CanGenerateThumbnail(ct), "expected %s to be unsupported", ct)
	}
}

func TestCalculateDimensions(t *testing.T) {
	g := NewThumbnailGenerator()

	tests := []struct {
		name           string
		origW, origH   int
		maxW, maxH     int
		expectW        int
		expectH        int
		expectUnchanged bool
	}{
		{
			name:            "image smaller than max — returned unchanged",
			origW:           100, origH: 80,
			maxW: 1440, maxH: 1440,
			expectW: 100, expectH: 80,
			expectUnchanged: true,
		},
		{
			name:  "landscape image wider than max",
			origW: 3000, origH: 2000,
			maxW: 1440, maxH: 1440,
			expectW: 1440, expectH: 960,
		},
		{
			name:  "portrait image taller than max",
			origW: 1000, origH: 3000,
			maxW: 1440, maxH: 1440,
			expectW: 480, expectH: 1440,
		},
		{
			name:  "square image larger than max",
			origW: 2000, origH: 2000,
			maxW: 250, maxH: 250,
			expectW: 250, expectH: 250,
		},
		{
			name:  "wide aspect ratio constrained by height",
			origW: 4000, origH: 1000,
			maxW: 1440, maxH: 1440,
			expectW: 1440, expectH: 360,
		},
		{
			name:  "exactly at max — returned unchanged",
			origW: 1440, origH: 1440,
			maxW: 1440, maxH: 1440,
			expectW:         1440, expectH: 1440,
			expectUnchanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := g.calculateDimensions(tt.origW, tt.origH, tt.maxW, tt.maxH)

			assert.Equal(t, tt.expectW, gotW, "width mismatch")
			assert.Equal(t, tt.expectH, gotH, "height mismatch")

			if !tt.expectUnchanged {
				// The output must fit within the requested bounds.
				assert.LessOrEqual(t, gotW, tt.maxW, "output width exceeds max")
				assert.LessOrEqual(t, gotH, tt.maxH, "output height exceeds max")

				// Aspect ratio must be preserved (within rounding error of 1 pixel).
				origRatio := float64(tt.origW) / float64(tt.origH)
				gotRatio := float64(gotW) / float64(gotH)
				assert.InDelta(t, origRatio, gotRatio, 0.02,
					"aspect ratio not preserved (orig %.4f, got %.4f)", origRatio, gotRatio)
			}
		})
	}
}
