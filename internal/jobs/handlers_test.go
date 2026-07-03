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
