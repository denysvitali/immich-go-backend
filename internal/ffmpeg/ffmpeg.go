// Package ffmpeg provides video processing utilities using the ffmpeg/ffprobe
// command-line tools. All functions gracefully handle the case where ffmpeg is
// not installed by returning descriptive errors that callers can check with
// errors.Is(err, ErrFFmpegNotFound).
package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("immich-go-backend/ffmpeg")

// ErrFFmpegNotFound is returned when ffmpeg or ffprobe is not available.
var ErrFFmpegNotFound = errors.New("ffmpeg/ffprobe not found in PATH")

// ProbeResult holds metadata extracted from a video file.
type ProbeResult struct {
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Duration   float64 `json:"duration"`
	Codec      string  `json:"codec"`
	CodecType  string  `json:"codec_type"`
	BitRate    int64   `json:"bit_rate,omitempty"`
	FormatName string  `json:"format_name,omitempty"`
}

// TranscodeOptions configures the H.264 transcoding process.
type TranscodeOptions struct {
	CRF       int    // Constant Rate Factor (0-51, lower = better quality, default 23)
	Preset    string // x264 preset: ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow (default "medium")
	MaxWidth  int    // Maximum output width (default 1920)
	MaxHeight int    // Maximum output height (default 1080)
}

// HLSOptions configures HLS playlist and segment generation.
type HLSOptions struct {
	CRF             int
	Preset          string
	MaxWidth        int
	MaxHeight       int
	SegmentDuration int
}

// DefaultTranscodeOptions returns sensible defaults for transcoding.
func DefaultTranscodeOptions() TranscodeOptions {
	return TranscodeOptions{
		CRF:       23,
		Preset:    "medium",
		MaxWidth:  1920,
		MaxHeight: 1080,
	}
}

// DefaultHLSOptions returns conservative defaults for on-demand HLS generation.
func DefaultHLSOptions() HLSOptions {
	return HLSOptions{
		CRF:             23,
		Preset:          "veryfast",
		MaxWidth:        1920,
		MaxHeight:       1080,
		SegmentDuration: 2,
	}
}

// IsAvailable checks whether both ffmpeg and ffprobe are present in PATH.
func IsAvailable() bool {
	_, ffmpegErr := exec.LookPath("ffmpeg")
	_, ffprobeErr := exec.LookPath("ffprobe")
	return ffmpegErr == nil && ffprobeErr == nil
}

// ProbeVideo runs ffprobe on the given file and returns structured metadata.
func ProbeVideo(ctx context.Context, inputPath string) (*ProbeResult, error) {
	ctx, span := tracer.Start(ctx, "ffmpeg.probe_video",
		trace.WithAttributes(attribute.String("input_path", inputPath)))
	defer span.End()

	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		span.SetAttributes(attribute.String("error", "ffprobe_not_found"))
		return nil, fmt.Errorf("%w: ffprobe", ErrFFmpegNotFound)
	}

	cmd := exec.CommandContext(
		ctx, ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)

	stdout, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			span.SetAttributes(attribute.String("stderr", string(exitErr.Stderr)))
		}
		span.RecordError(err)
		return nil, fmt.Errorf("ffprobe failed for %s: %w", inputPath, err)
	}

	var probeData struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Duration  string `json:"duration"`
			BitRate   string `json:"bit_rate"`
		} `json:"streams"`
		Format struct {
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
			FormatName string `json:"format_name"`
		} `json:"format"`
	}

	if err := json.Unmarshal(stdout, &probeData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	result := &ProbeResult{}

	// Find first video stream
	for _, stream := range probeData.Streams {
		if stream.CodecType == "video" {
			result.Width = stream.Width
			result.Height = stream.Height
			result.Codec = stream.CodecName
			result.CodecType = stream.CodecType
			if stream.Duration != "" {
				if dur, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
					result.Duration = dur
				}
			}
			if stream.BitRate != "" {
				if br, err := strconv.ParseInt(stream.BitRate, 10, 64); err == nil {
					result.BitRate = br
				}
			}
			break
		}
	}

	// Fall back to format-level duration
	if result.Duration == 0 && probeData.Format.Duration != "" {
		if dur, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
			result.Duration = dur
		}
	}

	if probeData.Format.BitRate != "" {
		if br, err := strconv.ParseInt(probeData.Format.BitRate, 10, 64); err == nil {
			if result.BitRate == 0 {
				result.BitRate = br
			}
		}
	}

	result.FormatName = probeData.Format.FormatName

	span.SetAttributes(
		attribute.Int("width", result.Width),
		attribute.Int("height", result.Height),
		attribute.String("codec", result.Codec),
		attribute.Float64("duration", result.Duration),
	)

	return result, nil
}

// ExtractVideoFrame extracts a single frame from a video to a JPEG image.
// By default it extracts at 10% of the video duration or 1 second, whichever
// is appropriate. Use options to customize.
type ExtractFrameOptions struct {
	// TimeOffset is the position in seconds to extract the frame at.
	// If <= 0, defaults to 10% of duration or 1 second.
	TimeOffset float64
	// Quality is the JPEG quality (1-100, default 85).
	Quality int
}

// DefaultExtractFrameOptions returns sensible defaults for frame extraction.
func DefaultExtractFrameOptions() ExtractFrameOptions {
	return ExtractFrameOptions{
		TimeOffset: -1, // auto
		Quality:    85,
	}
}

// ExtractVideoFrame extracts a single frame from a video file to a JPEG.
func ExtractVideoFrame(ctx context.Context, inputPath, outputPath string, opts ExtractFrameOptions) error {
	ctx, span := tracer.Start(ctx, "ffmpeg.extract_frame",
		trace.WithAttributes(
			attribute.String("input_path", inputPath),
			attribute.String("output_path", outputPath),
		))
	defer span.End()

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		span.SetAttributes(attribute.String("error", "ffmpeg_not_found"))
		return fmt.Errorf("%w: ffmpeg", ErrFFmpegNotFound)
	}

	// Determine time offset
	timeOffset := opts.TimeOffset
	if timeOffset <= 0 {
		// Probe video to get duration, then use 10% or 1s
		probe, probeErr := ProbeVideo(ctx, inputPath)
		if probeErr == nil && probe.Duration > 0 {
			timeOffset = probe.Duration * 0.1
			if timeOffset < 1 {
				timeOffset = 1
			}
		} else {
			timeOffset = 1
		}
	}

	if opts.Quality <= 0 || opts.Quality > 100 {
		opts.Quality = 85
	}

	span.SetAttributes(attribute.Float64("time_offset", timeOffset))

	cmd := exec.CommandContext(
		ctx, ffmpegPath,
		"-ss", fmt.Sprintf("%.3f", timeOffset),
		"-i", inputPath,
		"-frames:v", "1",
		"-q:v", fmt.Sprintf("%d", opts.Quality),
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		span.SetAttributes(attribute.String("ffmpeg_output", string(output)))
		span.RecordError(err)
		return fmt.Errorf("ffmpeg frame extraction failed: %w", err)
	}

	span.SetAttributes(attribute.String("status", "success"))
	return nil
}

// TranscodeToH264 transcodes a video to H.264 + AAC in an MP4 container.
// Output is capped at MaxWidth x MaxHeight while preserving aspect ratio.
func TranscodeToH264(ctx context.Context, inputPath, outputPath string, opts TranscodeOptions) error {
	ctx, span := tracer.Start(ctx, "ffmpeg.transcode_h264",
		trace.WithAttributes(
			attribute.String("input_path", inputPath),
			attribute.String("output_path", outputPath),
		))
	defer span.End()

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		span.SetAttributes(attribute.String("error", "ffmpeg_not_found"))
		return fmt.Errorf("%w: ffmpeg", ErrFFmpegNotFound)
	}

	// Set defaults
	if opts.CRF <= 0 {
		opts.CRF = 23
	}
	if opts.Preset == "" {
		opts.Preset = "medium"
	}
	if opts.MaxWidth <= 0 {
		opts.MaxWidth = 1920
	}
	if opts.MaxHeight <= 0 {
		opts.MaxHeight = 1080
	}

	span.SetAttributes(
		attribute.Int("crf", opts.CRF),
		attribute.String("preset", opts.Preset),
		attribute.Int("max_width", opts.MaxWidth),
		attribute.Int("max_height", opts.MaxHeight),
	)

	// Probe to determine if audio is already AAC
	probe, _ := ProbeVideo(ctx, inputPath)

	scaleFilter := fmt.Sprintf(
		"scale=w=%d:h=%d:force_original_aspect_ratio=decrease",
		opts.MaxWidth, opts.MaxHeight,
	)

	args := []string{
		"-i", inputPath,
		"-c:v", "libx264",
		"-crf", strconv.Itoa(opts.CRF),
		"-preset", opts.Preset,
		"-vf", scaleFilter,
		"-movflags", "+faststart",
		"-pix_fmt", "yuv420p",
		"-y",
	}

	// Audio handling: copy if AAC, otherwise encode to AAC
	audioCodec := "aac"
	audioBitrate := "128k"
	if probe != nil {
		// Check if audio stream is AAC
		audioIsAAC := checkAudioIsAAC(ctx, inputPath)
		if audioIsAAC {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", audioCodec, "-b:a", audioBitrate)
		}
	} else {
		args = append(args, "-c:a", audioCodec, "-b:a", audioBitrate)
	}

	args = append(args, outputPath)

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		span.SetAttributes(attribute.String("ffmpeg_output", string(output)))
		span.RecordError(err)
		return fmt.Errorf("ffmpeg transcode failed: %w", err)
	}

	span.SetAttributes(attribute.String("status", "success"))
	return nil
}

// GenerateHLS transcodes a video into a single-variant fMP4 HLS rendition.
func GenerateHLS(ctx context.Context, inputPath, outputDir string, opts HLSOptions) error {
	ctx, span := tracer.Start(ctx, "ffmpeg.generate_hls",
		trace.WithAttributes(
			attribute.String("input_path", inputPath),
			attribute.String("output_dir", outputDir),
		))
	defer span.End()

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		span.SetAttributes(attribute.String("error", "ffmpeg_not_found"))
		return fmt.Errorf("%w: ffmpeg", ErrFFmpegNotFound)
	}

	if opts.CRF <= 0 {
		opts.CRF = 23
	}
	if opts.Preset == "" {
		opts.Preset = "veryfast"
	}
	if opts.MaxWidth <= 0 {
		opts.MaxWidth = 1920
	}
	if opts.MaxHeight <= 0 {
		opts.MaxHeight = 1080
	}
	if opts.SegmentDuration <= 0 {
		opts.SegmentDuration = 2
	}

	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("failed to create HLS output directory: %w", err)
	}

	scaleFilter := fmt.Sprintf(
		"scale=w=%d:h=%d:force_original_aspect_ratio=decrease",
		opts.MaxWidth,
		opts.MaxHeight,
	)
	playlistPath := filepath.Join(outputDir, "playlist.m3u8")
	segmentPattern := filepath.Join(outputDir, "seg_%d.m4s")

	args := []string{
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-crf", strconv.Itoa(opts.CRF),
		"-preset", opts.Preset,
		"-vf", scaleFilter,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-f", "hls",
		"-hls_time", strconv.Itoa(opts.SegmentDuration),
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_segment_filename", segmentPattern,
		"-hls_flags", "independent_segments",
		"-y",
		playlistPath,
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		span.SetAttributes(attribute.String("ffmpeg_output", string(output)))
		span.RecordError(err)
		return fmt.Errorf("ffmpeg HLS generation failed: %w", err)
	}

	span.SetAttributes(attribute.String("status", "success"))
	return nil
}

// checkAudioIsAAC probes the input file and returns true if the first audio
// stream uses AAC codec.
func checkAudioIsAAC(ctx context.Context, inputPath string) bool {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return false
	}

	cmd := exec.CommandContext(
		ctx, ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		inputPath,
	)

	stdout, err := cmd.Output()
	if err != nil {
		return false
	}

	var probeData struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(stdout, &probeData); err != nil {
		return false
	}

	for _, stream := range probeData.Streams {
		if stream.CodecType == "audio" {
			return strings.EqualFold(stream.CodecName, "aac")
		}
	}
	return false
}

// GenerateTestVideo creates a tiny test MP4 file using ffmpeg.
// Useful for unit tests. Returns the path to the generated file.
func GenerateTestVideo(outputPath string, width, height int, duration time.Duration) error {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ErrFFmpegNotFound
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	cmd := exec.Command(
		ffmpegPath,
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=duration=%.0f:size=%dx%d:rate=1", duration.Seconds(), width, height),
		"-f", "lavfi",
		"-i", "sine=frequency=1000:duration=1",
		"-pix_fmt", "yuv420p",
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg test video generation failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
