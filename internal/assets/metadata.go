package assets

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("immich-go-backend/assets")

// MetadataExtractor handles extraction of metadata from various file types
type MetadataExtractor struct{}

// NewMetadataExtractor creates a new metadata extractor
func NewMetadataExtractor() *MetadataExtractor {
	return &MetadataExtractor{}
}

// ExtractMetadata extracts metadata from a file
func (e *MetadataExtractor) ExtractMetadata(ctx context.Context, reader io.Reader, filename string, contentType string, size int64) (*AssetMetadata, error) {
	ctx, span := tracer.Start(ctx, "metadata.extract",
		trace.WithAttributes(
			attribute.String("filename", filename),
			attribute.String("content_type", contentType),
			attribute.Int64("size", size),
		))
	defer span.End()

	metadata := &AssetMetadata{
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	}

	// Determine asset type from content type
	assetType := e.getAssetTypeFromContentType(contentType)
	span.SetAttributes(attribute.String("asset_type", string(assetType)))

	// Extract metadata based on file type
	switch assetType {
	case AssetTypeImage:
		if err := e.extractImageMetadata(ctx, reader, metadata); err != nil {
			span.RecordError(err)
			// Don't fail the entire operation for metadata extraction errors
			// Just log and continue
		}
	case AssetTypeVideo:
		if err := e.extractVideoMetadata(ctx, reader, metadata); err != nil {
			span.RecordError(err)
		}
	}

	return metadata, nil
}

// getAssetTypeFromContentType determines asset type from MIME type
func (e *MetadataExtractor) getAssetTypeFromContentType(contentType string) AssetType {
	contentType = strings.ToLower(contentType)

	switch {
	case strings.HasPrefix(contentType, "image/"):
		return AssetTypeImage
	case strings.HasPrefix(contentType, "video/"):
		return AssetTypeVideo
	case strings.HasPrefix(contentType, "audio/"):
		return AssetTypeAudio
	default:
		return AssetTypeOther
	}
}

// extractImageMetadata extracts EXIF data from images
func (e *MetadataExtractor) extractImageMetadata(ctx context.Context, reader io.Reader, metadata *AssetMetadata) error {
	ctx, span := tracer.Start(ctx, "metadata.extract_image")
	defer span.End()

	// Try to extract EXIF data
	x, err := exif.Decode(reader)
	if err != nil {
		// Not all images have EXIF data, this is not an error
		span.SetAttributes(attribute.Bool("has_exif", false))
		return nil
	}

	span.SetAttributes(attribute.Bool("has_exif", true))

	// Extract camera make and model
	if make, err := x.Get(exif.Make); err == nil {
		if makeStr, err := make.StringVal(); err == nil {
			metadata.Make = &makeStr
		}
	}

	if model, err := x.Get(exif.Model); err == nil {
		if modelStr, err := model.StringVal(); err == nil {
			metadata.Model = &modelStr
		}
	}

	// Extract lens model
	if lensModel, err := x.Get(exif.LensModel); err == nil {
		if lensStr, err := lensModel.StringVal(); err == nil {
			metadata.LensModel = &lensStr
		}
	}

	// Extract image dimensions
	if width, err := x.Get(exif.PixelXDimension); err == nil {
		if w, err := width.Int(0); err == nil {
			w32 := int32(w)
			metadata.Width = &w32
		}
	}

	if height, err := x.Get(exif.PixelYDimension); err == nil {
		if h, err := height.Int(0); err == nil {
			h32 := int32(h)
			metadata.Height = &h32
		}
	}

	// Extract camera settings
	if fNumber, err := x.Get(exif.FNumber); err == nil {
		if num, denom, err := fNumber.Rat2(0); err == nil && denom != 0 {
			fVal := float64(num) / float64(denom)
			metadata.FNumber = &fVal
		}
	}

	if focalLength, err := x.Get(exif.FocalLength); err == nil {
		if num, denom, err := focalLength.Rat2(0); err == nil && denom != 0 {
			flVal := float64(num) / float64(denom)
			metadata.FocalLength = &flVal
		}
	}

	if iso, err := x.Get(exif.ISOSpeedRatings); err == nil {
		if isoVal, err := iso.Int(0); err == nil {
			iso32 := int32(isoVal)
			metadata.ISO = &iso32
		}
	}

	if exposureTime, err := x.Get(exif.ExposureTime); err == nil {
		if expStr, err := exposureTime.StringVal(); err == nil {
			metadata.ExposureTime = &expStr
		}
	}

	// Extract GPS coordinates
	if lat, lon, err := x.LatLong(); err == nil {
		metadata.Latitude = &lat
		metadata.Longitude = &lon
	}

	// Extract date taken
	if dateTime, err := x.Get(exif.DateTimeOriginal); err == nil {
		if dateStr, err := dateTime.StringVal(); err == nil {
			if dateTaken, err := time.Parse("2006:01:02 15:04:05", dateStr); err == nil {
				metadata.DateTaken = &dateTaken
			}
		}
	}

	// Extract description/comment
	if desc, err := x.Get(exif.ImageDescription); err == nil {
		if descStr, err := desc.StringVal(); err == nil {
			metadata.Description = &descStr
		}
	}

	return nil
}

// extractVideoMetadata extracts metadata from video files
func (e *MetadataExtractor) extractVideoMetadata(ctx context.Context, reader io.Reader, metadata *AssetMetadata) error {
	ctx, span := tracer.Start(ctx, "metadata.extract_video")
	defer span.End()

	// For now, we'll implement basic video metadata extraction
	// In a production system, you'd use ffmpeg or similar
	// This is a placeholder for video metadata extraction

	// TODO: Implement video metadata extraction using ffmpeg
	// This would extract:
	// - Duration
	// - Resolution (width/height)
	// - Codec information
	// - Creation date
	// - GPS coordinates (if available)
	// - Camera make/model (if available)

	span.SetAttributes(attribute.String("status", "not_implemented"))
	return nil
}

// CalculateChecksum calculates a SHA256 checksum for the file content
func (e *MetadataExtractor) CalculateChecksum(ctx context.Context, reader io.Reader) (string, error) {
	ctx, span := tracer.Start(ctx, "metadata.calculate_checksum")
	defer span.End()

	hasher := sha256.New()

	// Copy data from reader to hasher
	bytesRead, err := io.Copy(hasher, reader)
	if err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("failed to read data for checksum: %w", err)
	}

	span.SetAttributes(attribute.Int64("bytes_read", bytesRead))

	// Calculate the checksum
	checksum := fmt.Sprintf("%x", hasher.Sum(nil))
	span.SetAttributes(attribute.String("checksum", checksum))

	return checksum, nil
}
