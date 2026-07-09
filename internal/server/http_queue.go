package server

import (
	"net/http"
	"strings"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// handleQueueListShape adapts the generated gateway response to Immich's
// REST contract. The protobuf response is an object containing `queues` and
// enum names, while the web client expects a lowercase-name array.
func (s *Server) handleQueueListShape(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}

	var response *immichv1.ListQueuesResponse
	var err error
	switch {
	case r.URL.Path == "/api/queues":
		ctx, ok := s.frontendGatewayContext(w, r)
		if !ok {
			return true
		}
		response, err = s.ListQueues(ctx, nil)
	case strings.HasPrefix(r.URL.Path, "/api/queues/") && !strings.Contains(strings.TrimPrefix(r.URL.Path, "/api/queues/"), "/"):
		ctx, ok := s.frontendGatewayContext(w, r)
		if !ok {
			return true
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/queues/")
		queue, queueErr := s.GetQueue(ctx, &immichv1.GetQueueRequest{Name: name})
		if queueErr != nil {
			writeGRPCErrorJSON(w, r, queueErr)
			return true
		}
		writeJSON(w, http.StatusOK, queueHTTPResponseFromProto(queue))
		return true
	default:
		return false
	}

	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return true
	}
	queues := make([]queueHTTPResponse, 0, len(response.Queues))
	for _, queue := range response.Queues {
		queues = append(queues, queueHTTPResponseFromProto(queue))
	}
	writeJSON(w, http.StatusOK, queues)
	return true
}

type queueHTTPResponse struct {
	Name       string              `json:"name"`
	IsPaused   bool                `json:"isPaused"`
	Statistics queueHTTPStatistics `json:"statistics"`
}

type queueHTTPStatistics struct {
	Active    int32 `json:"active"`
	Completed int32 `json:"completed"`
	Failed    int32 `json:"failed"`
	Delayed   int32 `json:"delayed"`
	Waiting   int32 `json:"waiting"`
	Paused    int32 `json:"paused"`
}

func queueHTTPResponseFromProto(queue *immichv1.QueueInfo) queueHTTPResponse {
	return queueHTTPResponse{
		Name:     queueNameHTTP(queue.Name),
		IsPaused: queue.IsPaused,
		Statistics: queueHTTPStatistics{
			Active:    queue.GetStatistics().GetActive(),
			Completed: queue.GetStatistics().GetCompleted(),
			Failed:    queue.GetStatistics().GetFailed(),
			Delayed:   queue.GetStatistics().GetDelayed(),
			Waiting:   queue.GetStatistics().GetWaiting(),
			Paused:    queue.GetStatistics().GetPaused(),
		},
	}
}

func queueNameHTTP(name immichv1.QueueName) string {
	values := map[immichv1.QueueName]string{
		immichv1.QueueName_QUEUE_NAME_THUMBNAIL_GENERATION: "thumbnailGeneration",
		immichv1.QueueName_QUEUE_NAME_METADATA_EXTRACTION:  "metadataExtraction",
		immichv1.QueueName_QUEUE_NAME_VIDEO_CONVERSION:     "videoConversion",
		immichv1.QueueName_QUEUE_NAME_FACE_DETECTION:       "faceDetection",
		immichv1.QueueName_QUEUE_NAME_FACIAL_RECOGNITION:   "facialRecognition",
		immichv1.QueueName_QUEUE_NAME_SMART_SEARCH:         "smartSearch",
		immichv1.QueueName_QUEUE_NAME_DUPLICATE_DETECTION:  "duplicateDetection",
		immichv1.QueueName_QUEUE_NAME_BACKGROUND_TASK:      "backgroundTask",
		immichv1.QueueName_QUEUE_NAME_STORAGE_MIGRATION:    "storageMigration",
		immichv1.QueueName_QUEUE_NAME_SEARCH:               "search",
		immichv1.QueueName_QUEUE_NAME_SIDECAR:              "sidecar",
		immichv1.QueueName_QUEUE_NAME_LIBRARY:              "library",
		immichv1.QueueName_QUEUE_NAME_NOTIFICATION:         "notification",
	}
	return values[name]
}
