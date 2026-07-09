package server

import (
	"encoding/json"
	"testing"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/require"
)

func TestQueueHTTPResponseUsesUpstreamNamesAndShape(t *testing.T) {
	response := queueHTTPResponseFromProto(&immichv1.QueueInfo{
		Name:     immichv1.QueueName_QUEUE_NAME_THUMBNAIL_GENERATION,
		IsPaused: true,
		Statistics: &immichv1.QueueStatistics{
			Waiting: 3,
		},
	})

	body, err := json.Marshal(response)
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"thumbnailGeneration","isPaused":true,"statistics":{"active":0,"completed":0,"failed":0,"delayed":0,"waiting":3,"paused":0}}`, string(body))
}
