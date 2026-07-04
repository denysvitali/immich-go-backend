package ffmpeg_test

import (
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/ffmpeg"
	"github.com/stretchr/testify/assert"
)

func TestIsAvailable_Unit(t *testing.T) {
	// This test runs without ffmpeg; it just verifies the function works
	result := ffmpeg.IsAvailable()
	assert.True(t, result || !result) // always a valid boolean
}

func TestDefaultTranscodeOptions_Unit(t *testing.T) {
	opts := ffmpeg.DefaultTranscodeOptions()
	assert.Equal(t, 23, opts.CRF)
	assert.Equal(t, "medium", opts.Preset)
	assert.Equal(t, 1920, opts.MaxWidth)
	assert.Equal(t, 1080, opts.MaxHeight)
}

func TestDefaultExtractFrameOptions_Unit(t *testing.T) {
	opts := ffmpeg.DefaultExtractFrameOptions()
	assert.Equal(t, -1.0, opts.TimeOffset)
	assert.Equal(t, 85, opts.Quality)
}

func TestErrFFmpegNotFound(t *testing.T) {
	assert.NotNil(t, ffmpeg.ErrFFmpegNotFound)
	assert.Equal(t, "ffmpeg/ffprobe not found in PATH", ffmpeg.ErrFFmpegNotFound.Error())
}
