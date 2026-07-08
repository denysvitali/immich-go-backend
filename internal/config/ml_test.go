package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMLActiveDefaultsOff(t *testing.T) {
	cfg := &Config{}
	setDefaults(cfg)

	assert.False(t, cfg.MLActive())
	assert.False(t, cfg.CLIPActive())
	assert.False(t, cfg.FaceRecognitionActive())
	assert.False(t, cfg.Features.MachineLearningEnabled)
	assert.False(t, cfg.Features.FaceRecognitionEnabled)
	assert.False(t, cfg.Features.CLIPSearchEnabled)
	assert.False(t, cfg.MachineLearning.Enabled)
}

func TestMLActiveRequiresFeatureAndURL(t *testing.T) {
	cfg := &Config{}
	setDefaults(cfg)

	cfg.Features.MachineLearningEnabled = true
	cfg.MachineLearning.Enabled = true
	// still missing URL
	assert.False(t, cfg.MLActive())

	cfg.MachineLearning.URL = "http://localhost:3003"
	assert.True(t, cfg.MLActive())

	cfg.Features.CLIPSearchEnabled = true
	assert.True(t, cfg.CLIPActive())

	cfg.Features.FaceRecognitionEnabled = true
	assert.True(t, cfg.FaceRecognitionActive())

	// Sub-feature can still disable
	cfg.MachineLearning.Clip.Enabled = false
	assert.False(t, cfg.CLIPActive())
	cfg.MachineLearning.FacialRecognition.Enabled = false
	assert.False(t, cfg.FaceRecognitionActive())
}

func TestDuplicateDetectionActive(t *testing.T) {
	cfg := &Config{}
	setDefaults(cfg)
	assert.False(t, cfg.DuplicateDetectionActive())

	cfg.Features.DuplicateDetectionEnabled = true
	assert.True(t, cfg.DuplicateDetectionActive())
}
