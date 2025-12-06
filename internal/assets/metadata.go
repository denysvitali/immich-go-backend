package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
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
	_, span := tracer.Start(ctx, "metadata.extract_image")
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
			// Ensure value fits in int32
			if w > 2147483647 {
				w = 2147483647
			}
			w32 := int32(w) // Safe after bounds check
			metadata.Width = &w32
		}
	}

	if height, err := x.Get(exif.PixelYDimension); err == nil {
		if h, err := height.Int(0); err == nil {
			// Ensure value fits in int32
			if h > 2147483647 {
				h = 2147483647
			}
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
			// Ensure value fits in int32
			if isoVal > 2147483647 {
				isoVal = 2147483647
			}
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

// ffprobeOutput represents the JSON output from ffprobe
type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecType   string `json:"codec_type"`
	CodecName   string `json:"codec_name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Duration    string `json:"duration"`
	RFrameRate  string `json:"r_frame_rate"`
	AvgFrameRate string `json:"avg_frame_rate"`
}

type ffprobeFormat struct {
	Duration   string            `json:"duration"`
	Size       string            `json:"size"`
	BitRate    string            `json:"bit_rate"`
	FormatName string            `json:"format_name"`
	Tags       map[string]string `json:"tags"`
}

// extractVideoMetadata extracts metadata from video files using ffprobe
func (e *MetadataExtractor) extractVideoMetadata(ctx context.Context, reader io.Reader, metadata *AssetMetadata) error {
	_, span := tracer.Start(ctx, "metadata.extract_video")
	defer span.End()

	// Check if ffprobe is available
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		span.SetAttributes(attribute.String("status", "ffprobe_not_found"))
		// ffprobe not available, skip video metadata extraction
		return nil
	}

	// Create a temporary file to store the video data for ffprobe
	tmpFile, err := os.CreateTemp("", "video-metadata-*.tmp")
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy the reader content to the temp file
	bytesWritten, err := io.Copy(tmpFile, reader)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	span.SetAttributes(attribute.Int64("bytes_written", bytesWritten))

	// Close the file before running ffprobe
	tmpFile.Close()

	// Run ffprobe to extract metadata
	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		tmpFile.Name(),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("stderr", stderr.String()))
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse ffprobe output
	var probeData ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &probeData); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	span.SetAttributes(attribute.String("status", "success"))

	// Extract video stream info (first video stream)
	for _, stream := range probeData.Streams {
		if stream.CodecType == "video" {
			if stream.Width > 0 {
				w := int32(stream.Width)
				metadata.Width = &w
			}
			if stream.Height > 0 {
				h := int32(stream.Height)
				metadata.Height = &h
			}
			// Try stream duration first
			if stream.Duration != "" {
				if dur, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
					metadata.Duration = &dur
				}
			}
			span.SetAttributes(
				attribute.Int("width", stream.Width),
				attribute.Int("height", stream.Height),
				attribute.String("codec", stream.CodecName),
			)
			break
		}
	}

	// Extract format-level metadata
	if probeData.Format.Duration != "" && metadata.Duration == nil {
		if dur, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
			metadata.Duration = &dur
			span.SetAttributes(attribute.Float64("duration", dur))
		}
	}

	// Extract creation date from format tags
	if probeData.Format.Tags != nil {
		// Try various common date tags
		dateFields := []string{"creation_time", "date", "com.apple.quicktime.creationdate"}
		for _, field := range dateFields {
			if dateStr, ok := probeData.Format.Tags[field]; ok {
				// Try parsing ISO 8601 format
				for _, layout := range []string{
					time.RFC3339,
					"2006-01-02T15:04:05.000000Z",
					"2006-01-02T15:04:05Z",
					"2006-01-02 15:04:05",
				} {
					if dateTaken, err := time.Parse(layout, dateStr); err == nil {
						metadata.DateTaken = &dateTaken
						span.SetAttributes(attribute.String("date_taken", dateTaken.String()))
						break
					}
				}
				if metadata.DateTaken != nil {
					break
				}
			}
		}

		// Extract make/model if available
		if make, ok := probeData.Format.Tags["com.apple.quicktime.make"]; ok {
			metadata.Make = &make
		}
		if model, ok := probeData.Format.Tags["com.apple.quicktime.model"]; ok {
			metadata.Model = &model
		}

		// Extract GPS coordinates if available
		if latStr, ok := probeData.Format.Tags["com.apple.quicktime.location.ISO6709"]; ok {
			lat, lon := parseISO6709Location(latStr)
			if lat != 0 || lon != 0 {
				metadata.Latitude = &lat
				metadata.Longitude = &lon
			}
		}
	}

	return nil
}

// parseISO6709Location parses ISO 6709 location string (e.g., "+37.7749-122.4194/")
func parseISO6709Location(s string) (lat, lon float64) {
	s = strings.TrimSuffix(s, "/")

	// Find the second sign (start of longitude)
	secondSignIdx := -1
	for i := 1; i < len(s); i++ {
		if s[i] == '+' || s[i] == '-' {
			secondSignIdx = i
			break
		}
	}

	if secondSignIdx == -1 {
		return 0, 0
	}

	latStr := s[:secondSignIdx]
	lonStr := s[secondSignIdx:]

	lat, _ = strconv.ParseFloat(latStr, 64)
	lon, _ = strconv.ParseFloat(lonStr, 64)

	return lat, lon
}

// ExtractVideoMetadataFromFile extracts metadata from a video file on disk (more efficient)
func (e *MetadataExtractor) ExtractVideoMetadataFromFile(ctx context.Context, filePath string, metadata *AssetMetadata) error {
	_, span := tracer.Start(ctx, "metadata.extract_video_from_file",
		trace.WithAttributes(attribute.String("file_path", filePath)))
	defer span.End()

	// Check if ffprobe is available
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		span.SetAttributes(attribute.String("status", "ffprobe_not_found"))
		return nil
	}

	// Run ffprobe directly on the file
	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("stderr", stderr.String()))
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse ffprobe output
	var probeData ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &probeData); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	span.SetAttributes(attribute.String("status", "success"))

	// Extract video stream info (first video stream)
	for _, stream := range probeData.Streams {
		if stream.CodecType == "video" {
			if stream.Width > 0 {
				w := int32(stream.Width)
				metadata.Width = &w
			}
			if stream.Height > 0 {
				h := int32(stream.Height)
				metadata.Height = &h
			}
			if stream.Duration != "" {
				if dur, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
					metadata.Duration = &dur
				}
			}
			span.SetAttributes(
				attribute.Int("width", stream.Width),
				attribute.Int("height", stream.Height),
				attribute.String("codec", stream.CodecName),
			)
			break
		}
	}

	// Extract format-level metadata
	if probeData.Format.Duration != "" && metadata.Duration == nil {
		if dur, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
			metadata.Duration = &dur
			span.SetAttributes(attribute.Float64("duration", dur))
		}
	}

	// Extract creation date and other tags from format
	if probeData.Format.Tags != nil {
		dateFields := []string{"creation_time", "date", "com.apple.quicktime.creationdate"}
		for _, field := range dateFields {
			if dateStr, ok := probeData.Format.Tags[field]; ok {
				for _, layout := range []string{
					time.RFC3339,
					"2006-01-02T15:04:05.000000Z",
					"2006-01-02T15:04:05Z",
					"2006-01-02 15:04:05",
				} {
					if dateTaken, err := time.Parse(layout, dateStr); err == nil {
						metadata.DateTaken = &dateTaken
						break
					}
				}
				if metadata.DateTaken != nil {
					break
				}
			}
		}

		if make, ok := probeData.Format.Tags["com.apple.quicktime.make"]; ok {
			metadata.Make = &make
		}
		if model, ok := probeData.Format.Tags["com.apple.quicktime.model"]; ok {
			metadata.Model = &model
		}

		if latStr, ok := probeData.Format.Tags["com.apple.quicktime.location.ISO6709"]; ok {
			lat, lon := parseISO6709Location(latStr)
			if lat != 0 || lon != 0 {
				metadata.Latitude = &lat
				metadata.Longitude = &lon
			}
		}
	}

	return nil
}

// CalculateChecksum calculates a SHA256 checksum for the file content
func (e *MetadataExtractor) CalculateChecksum(ctx context.Context, reader io.Reader) (string, error) {
	_, span := tracer.Start(ctx, "metadata.calculate_checksum")
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
