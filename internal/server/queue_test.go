package server

import (
	"encoding/json"
	"testing"
	"time"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueueNameToProto(t *testing.T) {
	tests := []struct {
		name string
		in   queue.QueueName
		want immichv1.QueueName
	}{
		{"thumbnail generation", queue.QueueThumbnailGeneration, immichv1.QueueName_QUEUE_NAME_THUMBNAIL_GENERATION},
		{"metadata extraction", queue.QueueMetadataExtraction, immichv1.QueueName_QUEUE_NAME_METADATA_EXTRACTION},
		{"video conversion", queue.QueueVideoConversion, immichv1.QueueName_QUEUE_NAME_VIDEO_CONVERSION},
		{"face detection", queue.QueueFaceDetection, immichv1.QueueName_QUEUE_NAME_FACE_DETECTION},
		{"facial recognition", queue.QueueFacialRecognition, immichv1.QueueName_QUEUE_NAME_FACIAL_RECOGNITION},
		{"smart search", queue.QueueSmartSearch, immichv1.QueueName_QUEUE_NAME_SMART_SEARCH},
		{"duplicate detection", queue.QueueDuplicateDetection, immichv1.QueueName_QUEUE_NAME_DUPLICATE_DETECTION},
		{"background task", queue.QueueBackgroundTask, immichv1.QueueName_QUEUE_NAME_BACKGROUND_TASK},
		{"storage migration", queue.QueueStorageMigration, immichv1.QueueName_QUEUE_NAME_STORAGE_MIGRATION},
		{"search", queue.QueueSearch, immichv1.QueueName_QUEUE_NAME_SEARCH},
		{"sidecar", queue.QueueSidecar, immichv1.QueueName_QUEUE_NAME_SIDECAR},
		{"library", queue.QueueLibrary, immichv1.QueueName_QUEUE_NAME_LIBRARY},
		{"notification", queue.QueueNotification, immichv1.QueueName_QUEUE_NAME_NOTIFICATION},
		{"unknown", queue.QueueName("missing"), immichv1.QueueName_QUEUE_NAME_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, queueNameToProto(tt.in))
		})
	}
}

func TestQueueJobStatusMappings(t *testing.T) {
	tests := []struct {
		name     string
		internal queue.JobStatus
		proto    immichv1.QueueJobStatus
	}{
		{"active", queue.JobStatusActive, immichv1.QueueJobStatus_QUEUE_JOB_STATUS_ACTIVE},
		{"completed", queue.JobStatusCompleted, immichv1.QueueJobStatus_QUEUE_JOB_STATUS_COMPLETED},
		{"failed", queue.JobStatusFailed, immichv1.QueueJobStatus_QUEUE_JOB_STATUS_FAILED},
		{"delayed", queue.JobStatusDelayed, immichv1.QueueJobStatus_QUEUE_JOB_STATUS_DELAYED},
		{"waiting", queue.JobStatusWaiting, immichv1.QueueJobStatus_QUEUE_JOB_STATUS_WAITING},
		{"paused", queue.JobStatusPaused, immichv1.QueueJobStatus_QUEUE_JOB_STATUS_PAUSED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.proto, internalStatusToProto(tt.internal))
			assert.Equal(t, tt.internal, protoStatusToInternal(tt.proto))
		})
	}
}

func TestQueueJobStatusMappingDefaults(t *testing.T) {
	assert.Equal(t, queue.JobStatusWaiting, protoStatusToInternal(immichv1.QueueJobStatus_QUEUE_JOB_STATUS_UNSPECIFIED))
	assert.Equal(t, immichv1.QueueJobStatus_QUEUE_JOB_STATUS_UNSPECIFIED, internalStatusToProto(queue.JobStatus("missing")))
}

func TestQueueToProto(t *testing.T) {
	got := queueToProto(queue.QueueInfo{
		Name:     queue.QueueNotification,
		IsPaused: true,
		Statistics: queue.QueueStatistics{
			Active:    1,
			Completed: 2,
			Failed:    3,
			Delayed:   4,
			Waiting:   5,
			Paused:    6,
		},
	})

	require.NotNil(t, got.Statistics)
	assert.Equal(t, immichv1.QueueName_QUEUE_NAME_NOTIFICATION, got.Name)
	assert.True(t, got.IsPaused)
	assert.Equal(t, int32(1), got.Statistics.Active)
	assert.Equal(t, int32(2), got.Statistics.Completed)
	assert.Equal(t, int32(3), got.Statistics.Failed)
	assert.Equal(t, int32(4), got.Statistics.Delayed)
	assert.Equal(t, int32(5), got.Statistics.Waiting)
	assert.Equal(t, int32(6), got.Statistics.Paused)
}

func TestJobToProto(t *testing.T) {
	timestamp := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	job := &queue.Job{
		ID:        "job-1",
		Name:      "notify",
		Data:      map[string]string{"assetId": "asset-1"},
		Timestamp: timestamp,
		Status:    queue.JobStatusFailed,
		Error:     "worker failed",
		Attempts:  2,
	}

	got := jobToProto(job)

	assert.Equal(t, "job-1", got.Id)
	assert.Equal(t, "notify", got.Name)
	assert.Equal(t, timestamp.Unix(), got.Timestamp)
	assert.Equal(t, immichv1.QueueJobStatus_QUEUE_JOB_STATUS_FAILED, got.Status)
	require.NotNil(t, got.Error)
	assert.Equal(t, "worker failed", *got.Error)
	require.NotNil(t, got.Attempts)
	assert.Equal(t, int32(2), *got.Attempts)

	var data map[string]string
	require.NoError(t, json.Unmarshal([]byte(got.Data), &data))
	assert.Equal(t, map[string]string{"assetId": "asset-1"}, data)
}

func TestJobToProtoOmitsEmptyOptionals(t *testing.T) {
	got := jobToProto(&queue.Job{
		ID:        "job-1",
		Timestamp: time.Unix(42, 0),
		Status:    queue.JobStatusWaiting,
	})

	assert.Nil(t, got.Error)
	assert.Nil(t, got.Attempts)
}
