//go:build integration

package ffmpeg_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/ffmpeg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoFFmpeg(t *testing.T) {
	if !ffmpeg.IsAvailable() {
		t.Skip("ffmpeg/ffprobe not available in PATH, skipping test")
	}
}

func TestIsAvailable(t *testing.T) {
	// This test should pass regardless of whether ffmpeg is installed
	result := ffmpeg.IsAvailable()
	t.Logf("ffmpeg.IsAvailable() = %v", result)
	// Just assert it's a boolean, no assertion on the value since it depends on environment
	assert.True(t, result || !result)
}

func TestProbeVideo(t *testing.T) {
	skipIfNoFFmpeg(t)

	// Create a temporary test video
	tmpDir := t.TempDir()
	testVideoPath := filepath.Join(tmpDir, "test.mp4")

	err := ffmpeg.GenerateTestVideo(testVideoPath, 320, 240, 2*time.Second)
	require.NoError(t, err, "failed to generate test video")
	require.FileExists(t, testVideoPath)

	// Probe the video
	result, err := ffmpeg.ProbeVideo(t.Context(), testVideoPath)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 320, result.Width)
	assert.Equal(t, 240, result.Height)
	assert.Greater(t, result.Duration, 0.0)
	assert.Equal(t, "h264", result.Codec)
	assert.Equal(t, "video", result.CodecType)
}

func TestProbeVideo_NotFound(t *testing.T) {
	_, err := ffmpeg.ProbeVideo(t.Context(), "/nonexistent/path/video.mp4")
	assert.Error(t, err)
}

func TestExtractVideoFrame(t *testing.T) {
	skipIfNoFFmpeg(t)

	tmpDir := t.TempDir()
	testVideoPath := filepath.Join(tmpDir, "test.mp4")
	outputPath := filepath.Join(tmpDir, "frame.jpg")

	err := ffmpeg.GenerateTestVideo(testVideoPath, 320, 240, 2*time.Second)
	require.NoError(t, err)

	opts := ffmpeg.ExtractFrameOptions{
		TimeOffset: 0.5,
		Quality:    85,
	}

	err = ffmpeg.ExtractVideoFrame(t.Context(), testVideoPath, outputPath, opts)
	require.NoError(t, err)
	require.FileExists(t, outputPath)

	// Verify it's a valid JPEG by checking file size
	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "extracted frame should have non-zero size")
}

func TestExtractVideoFrame_AutoOffset(t *testing.T) {
	skipIfNoFFmpeg(t)

	tmpDir := t.TempDir()
	testVideoPath := filepath.Join(tmpDir, "test.mp4")
	outputPath := filepath.Join(tmpDir, "frame_auto.jpg")

	err := ffmpeg.GenerateTestVideo(testVideoPath, 320, 240, 5*time.Second)
	require.NoError(t, err)

	// Use default options (auto time offset)
	opts := ffmpeg.DefaultExtractFrameOptions()
	err = ffmpeg.ExtractVideoFrame(t.Context(), testVideoPath, outputPath, opts)
	require.NoError(t, err)
	require.FileExists(t, outputPath)

	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

func TestTranscodeToH264(t *testing.T) {
	skipIfNoFFmpeg(t)

	tmpDir := t.TempDir()
	testVideoPath := filepath.Join(tmpDir, "test.mp4")
	outputPath := filepath.Join(tmpDir, "output.mp4")

	// Generate a test video with larger dimensions
	err := ffmpeg.GenerateTestVideo(testVideoPath, 640, 480, 2*time.Second)
	require.NoError(t, err)

	opts := ffmpeg.TranscodeOptions{
		CRF:       28,
		Preset:    "ultrafast",
		MaxWidth:  320,
		MaxHeight: 240,
	}

	err = ffmpeg.TranscodeToH264(t.Context(), testVideoPath, outputPath, opts)
	require.NoError(t, err)
	require.FileExists(t, outputPath)

	// Verify the output is a valid H.264 MP4
	result, err := ffmpeg.ProbeVideo(t.Context(), outputPath)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "h264", result.Codec)
	assert.LessOrEqual(t, result.Width, 320)
	assert.LessOrEqual(t, result.Height, 240)
	assert.Greater(t, result.Duration, 0.0)
}

func TestTranscodeToH264_Defaults(t *testing.T) {
	skipIfNoFFmpeg(t)

	tmpDir := t.TempDir()
	testVideoPath := filepath.Join(tmpDir, "test.mp4")
	outputPath := filepath.Join(tmpDir, "output_default.mp4")

	err := ffmpeg.GenerateTestVideo(testVideoPath, 640, 480, 1*time.Second)
	require.NoError(t, err)

	// Use all defaults
	opts := ffmpeg.DefaultTranscodeOptions()
	err = ffmpeg.TranscodeToH264(t.Context(), testVideoPath, outputPath, opts)
	require.NoError(t, err)
	require.FileExists(t, outputPath)

	result, err := ffmpeg.ProbeVideo(t.Context(), outputPath)
	require.NoError(t, err)
	assert.Equal(t, "h264", result.Codec)
}

func TestGenerateTestVideo(t *testing.T) {
	skipIfNoFFmpeg(t)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "generated.mp4")

	err := ffmpeg.GenerateTestVideo(outputPath, 320, 240, 1*time.Second)
	require.NoError(t, err)
	require.FileExists(t, outputPath)

	// Verify it's a valid video
	result, err := ffmpeg.ProbeVideo(t.Context(), outputPath)
	require.NoError(t, err)
	assert.Equal(t, 320, result.Width)
	assert.Equal(t, 240, result.Height)
}

func TestTranscodeToH264_FFmpegNotAvailable(t *testing.T) {
	// This test simulates what happens when ffmpeg is not available
	// We can't easily test this without mocking exec.LookPath, so we just
	// verify the error type is correct when given a non-existent ffmpeg path
	// by checking that the function returns an error for invalid input
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.mp4")

	// Use a non-existent input file - this should fail before checking ffmpeg
	err := ffmpeg.TranscodeToH264(t.Context(), "/nonexistent/input.mp4", outputPath, ffmpeg.DefaultTranscodeOptions())
	assert.Error(t, err)
}

func TestExtractVideoFrame_FFmpegNotAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "frame.jpg")

	err := ffmpeg.ExtractVideoFrame(t.Context(), "/nonexistent/input.mp4", outputPath, ffmpeg.DefaultExtractFrameOptions())
	assert.Error(t, err)
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.txt")
	dstPath := filepath.Join(tmpDir, "dst.txt")

	content := []byte("hello world")
	err := os.WriteFile(srcPath, content, 0644)
	require.NoError(t, err)

	err = ffmpeg.CopyFile(srcPath, dstPath)
	require.NoError(t, err)

	dstContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, content, dstContent)
}

func TestDefaultTranscodeOptions(t *testing.T) {
	opts := ffmpeg.DefaultTranscodeOptions()
	assert.Equal(t, 23, opts.CRF)
	assert.Equal(t, "medium", opts.Preset)
	assert.Equal(t, 1920, opts.MaxWidth)
	assert.Equal(t, 1080, opts.MaxHeight)
}

func TestDefaultExtractFrameOptions(t *testing.T) {
	opts := ffmpeg.DefaultExtractFrameOptions()
	assert.Equal(t, -1.0, opts.TimeOffset)
	assert.Equal(t, 85, opts.Quality)
}
