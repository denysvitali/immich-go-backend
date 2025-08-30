package assets

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ThumbnailGenerator handles generation of thumbnails for assets
type ThumbnailGenerator struct {
	// Configuration for different thumbnail sizes
	sizes map[ThumbnailType]ThumbnailConfig
}

// ThumbnailConfig represents configuration for a thumbnail type
type ThumbnailConfig struct {
	MaxWidth  int
	MaxHeight int
	Quality   int    // JPEG quality (1-100)
	Format    string // "jpeg", "webp", "png"
}

// NewThumbnailGenerator creates a new thumbnail generator
func NewThumbnailGenerator() *ThumbnailGenerator {
	return &ThumbnailGenerator{
		sizes: map[ThumbnailType]ThumbnailConfig{
			ThumbnailTypePreview: {
				MaxWidth:  1440,
				MaxHeight: 1440,
				Quality:   80,
				Format:    "jpeg",
			},
			ThumbnailTypeWebp: {
				MaxWidth:  250,
				MaxHeight: 250,
				Quality:   75,
				Format:    "webp",
			},
			ThumbnailTypeThumb: {
				MaxWidth:  160,
				MaxHeight: 160,
				Quality:   70,
				Format:    "jpeg",
			},
		},
	}
}

// GenerateThumbnails generates all required thumbnails for an asset
func (g *ThumbnailGenerator) GenerateThumbnails(ctx context.Context, reader io.Reader, originalFilename string) (map[ThumbnailType][]byte, error) {
	ctx, span := tracer.Start(ctx, "thumbnails.generate_all",
		trace.WithAttributes(
			attribute.String("filename", originalFilename),
		))
	defer span.End()

	// Decode the original image
	img, format, err := image.Decode(reader)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	span.SetAttributes(
		attribute.String("original_format", format),
		attribute.Int("original_width", img.Bounds().Dx()),
		attribute.Int("original_height", img.Bounds().Dy()),
	)

	thumbnails := make(map[ThumbnailType][]byte)

	// Generate each thumbnail type
	for thumbType, config := range g.sizes {
		thumbData, err := g.generateThumbnail(ctx, img, thumbType, config)
		if err != nil {
			span.RecordError(err)
			// Continue with other thumbnails even if one fails
			continue
		}
		thumbnails[thumbType] = thumbData
	}

	span.SetAttributes(attribute.Int("thumbnails_generated", len(thumbnails)))
	return thumbnails, nil
}

// generateThumbnail generates a single thumbnail
func (g *ThumbnailGenerator) generateThumbnail(ctx context.Context, img image.Image, thumbType ThumbnailType, config ThumbnailConfig) ([]byte, error) {
	_, span := tracer.Start(ctx, "thumbnails.generate_single",
		trace.WithAttributes(
			attribute.String("thumbnail_type", string(thumbType)),
			attribute.Int("max_width", config.MaxWidth),
			attribute.Int("max_height", config.MaxHeight),
		))
	defer span.End()

	// Calculate new dimensions while maintaining aspect ratio
	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	newWidth, newHeight := g.calculateDimensions(originalWidth, originalHeight, config.MaxWidth, config.MaxHeight)

	span.SetAttributes(
		attribute.Int("new_width", newWidth),
		attribute.Int("new_height", newHeight),
	)

	// Resize the image
	resized := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	// Encode to bytes
	var buf bytes.Buffer

	switch config.Format {
	case "jpeg":
		err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: config.Quality})
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to encode JPEG: %w", err)
		}
	case "png":
		err := png.Encode(&buf, resized)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to encode PNG: %w", err)
		}
	case "webp":
		// For WebP, we'd need a WebP encoder library
		// For now, fall back to JPEG
		err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: config.Quality})
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to encode WebP (fallback JPEG): %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported thumbnail format: %s", config.Format)
	}

	data := buf.Bytes()
	span.SetAttributes(attribute.Int("thumbnail_size", len(data)))

	return data, nil
}

// calculateDimensions calculates new dimensions while maintaining aspect ratio
func (g *ThumbnailGenerator) calculateDimensions(originalWidth, originalHeight, maxWidth, maxHeight int) (int, int) {
	if originalWidth <= maxWidth && originalHeight <= maxHeight {
		return originalWidth, originalHeight
	}

	aspectRatio := float64(originalWidth) / float64(originalHeight)

	var newWidth, newHeight int

	if float64(maxWidth)/aspectRatio <= float64(maxHeight) {
		newWidth = maxWidth
		newHeight = int(float64(maxWidth) / aspectRatio)
	} else {
		newHeight = maxHeight
		newWidth = int(float64(maxHeight) * aspectRatio)
	}

	return newWidth, newHeight
}

// GetThumbnailPath generates a path for a thumbnail
func (g *ThumbnailGenerator) GetThumbnailPath(originalPath string, thumbType ThumbnailType) string {
	dir := filepath.Dir(originalPath)
	filename := filepath.Base(originalPath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	config := g.sizes[thumbType]
	var thumbExt string
	switch config.Format {
	case "webp":
		thumbExt = ".webp"
	case "png":
		thumbExt = ".png"
	default:
		thumbExt = ".jpg"
	}

	return filepath.Join(dir, "thumbnails", fmt.Sprintf("%s_%s%s", nameWithoutExt, thumbType, thumbExt))
}

// CanGenerateThumbnail checks if thumbnails can be generated for a file type
func (g *ThumbnailGenerator) CanGenerateThumbnail(contentType string) bool {
	contentType = strings.ToLower(contentType)

	supportedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
		"image/bmp":  true,
		"image/tiff": true,
		"image/webp": true,
		// Add more supported types as needed
	}

	return supportedTypes[contentType]
}

// GetThumbnailInfo returns information about a generated thumbnail
func (g *ThumbnailGenerator) GetThumbnailInfo(thumbType ThumbnailType, data []byte, path string) ThumbnailInfo {
	config := g.sizes[thumbType]

	// For a more accurate implementation, you'd decode the thumbnail
	// to get actual dimensions. For now, we'll use the max dimensions.
	return ThumbnailInfo{
		Type:   thumbType,
		Path:   path,
		Width:  int32(config.MaxWidth),
		Height: int32(config.MaxHeight),
		Size:   int64(len(data)),
	}
}
