package jobs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetIDFromTaskUsesTypedPayload(t *testing.T) {
	assetID := uuid.New()
	task := newTask(t, JobTypeThumbnailGeneration, ThumbnailGenerationPayload{
		AssetID: assetID.String(),
	})

	got, err := assetIDFromTask(task, assetID.String())

	require.NoError(t, err)
	assert.Equal(t, assetID, got)
}

func TestAssetIDFromTaskFallsBackToLegacyPayload(t *testing.T) {
	assetID := uuid.New()
	task := newTask(t, JobTypeThumbnailGeneration, JobPayload{
		ID:     "legacy-job",
		UserID: uuid.NewString(),
		Data: map[string]interface{}{
			"asset_id": assetID.String(),
		},
		CreatedAt: time.Now(),
	})

	got, err := assetIDFromTask(task, "")

	require.NoError(t, err)
	assert.Equal(t, assetID, got)
}

func TestAssetIDFromTaskRejectsLegacyPayloadWithoutAssetID(t *testing.T) {
	task := newTask(t, JobTypeThumbnailGeneration, JobPayload{
		ID:        "legacy-job",
		UserID:    uuid.NewString(),
		Data:      map[string]interface{}{},
		CreatedAt: time.Now(),
	})

	got, err := assetIDFromTask(task, "")

	require.Error(t, err)
	assert.Equal(t, uuid.Nil, got)
	assert.Contains(t, err.Error(), "invalid asset_id")
}

func newTask(t *testing.T, jobType JobType, payload any) *asynq.Task {
	t.Helper()

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	return asynq.NewTask(string(jobType), payloadBytes)
}

func TestMLJobsSkipWhenDisabled(t *testing.T) {
	// Handlers with nil ML client / config must succeed (skip), not hard-fail.
	h := NewHandlers(nil, nil, nil, nil, nil, nil)

	faceTask := newTask(t, JobTypeFaceDetection, FaceDetectionPayload{AssetID: uuid.NewString()})
	// GetAsset will not be reached because we skip before DB access when disabled...
	// actually we parse UUID then check config — nil config → skip. No DB needed.
	require.NoError(t, h.HandleFaceDetection(t.Context(), faceTask))

	smartTask := newTask(t, JobTypeSmartSearch, SmartSearchIndexPayload{AssetID: uuid.NewString()})
	require.NoError(t, h.HandleSmartSearchIndex(t.Context(), smartTask))

	recogTask := newTask(t, JobTypeFaceRecognition, FaceRecognitionPayload{FaceID: uuid.NewString()})
	require.NoError(t, h.HandleFaceRecognition(t.Context(), recogTask))
}

func TestCosineDistance(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0}
	assert.InDelta(t, 0, cosineDistance(a, b), 1e-6)

	c := []float32{0, 1}
	assert.InDelta(t, 1, cosineDistance(a, c), 1e-6)

	assert.Equal(t, float64(1), cosineDistance(nil, a))
}

func TestParseVectorString(t *testing.T) {
	emb, err := parseVectorString("[0.1,0.2]")
	require.NoError(t, err)
	require.Len(t, emb, 2)
	assert.InDelta(t, 0.1, emb[0], 1e-6)

	emb, err = parseVectorString("{0.5,0.6}")
	require.NoError(t, err)
	require.Len(t, emb, 2)
}
