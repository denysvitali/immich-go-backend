package server

import (
	"context"
	"encoding/json"

	"github.com/denysvitali/immich-go-backend/internal/queue"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ListQueues returns a list of all queues
func (s *Server) ListQueues(ctx context.Context, _ *emptypb.Empty) (*immichv1.ListQueuesResponse, error) {
	// Verify user is authenticated and is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	queues, err := s.queueService.GetAllQueues(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get queues: %v", err)
	}

	var protoQueues []*immichv1.QueueInfo
	for _, q := range queues {
		protoQueues = append(protoQueues, queueToProto(q))
	}

	return &immichv1.ListQueuesResponse{
		Queues: protoQueues,
	}, nil
}

// GetQueue returns information about a specific queue
func (s *Server) GetQueue(ctx context.Context, req *immichv1.GetQueueRequest) (*immichv1.QueueInfo, error) {
	// Verify user is authenticated and is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	queueInfo, err := s.queueService.GetQueue(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "queue not found: %v", err)
	}

	return queueToProto(*queueInfo), nil
}

// UpdateQueue updates a queue (pause/resume)
func (s *Server) UpdateQueue(ctx context.Context, req *immichv1.UpdateQueueRequest) (*immichv1.QueueInfo, error) {
	// Verify user is authenticated and is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	var isPaused *bool
	if req.IsPaused != nil {
		isPaused = req.IsPaused
	}

	queueInfo, err := s.queueService.UpdateQueue(ctx, req.Name, isPaused)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "queue not found: %v", err)
	}

	return queueToProto(*queueInfo), nil
}

// GetQueueJobs returns jobs from a specific queue
func (s *Server) GetQueueJobs(ctx context.Context, req *immichv1.GetQueueJobsRequest) (*immichv1.ListQueueJobsResponse, error) {
	// Verify user is authenticated and is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	// Convert proto status to internal status
	var statuses []queue.JobStatus
	for _, s := range req.Status {
		statuses = append(statuses, protoStatusToInternal(s))
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	offset := int(req.Offset)

	jobs, total, err := s.queueService.GetQueueJobs(ctx, req.Name, statuses, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "queue not found: %v", err)
	}

	var protoJobs []*immichv1.QueueJob
	for _, job := range jobs {
		protoJobs = append(protoJobs, jobToProto(job))
	}

	return &immichv1.ListQueueJobsResponse{
		Jobs:  protoJobs,
		Total: int32(total),
	}, nil
}

// ClearQueueJobs clears jobs from a queue
func (s *Server) ClearQueueJobs(ctx context.Context, req *immichv1.ClearQueueJobsRequest) (*emptypb.Empty, error) {
	// Verify user is authenticated and is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	err = s.queueService.ClearQueueJobs(ctx, req.Name, req.IncludeFailed)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "queue not found: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// Helper functions

func queueToProto(q queue.QueueInfo) *immichv1.QueueInfo {
	return &immichv1.QueueInfo{
		Name:     queueNameToProto(q.Name),
		IsPaused: q.IsPaused,
		Statistics: &immichv1.QueueStatistics{
			Active:    int32(q.Statistics.Active),
			Completed: int32(q.Statistics.Completed),
			Failed:    int32(q.Statistics.Failed),
			Delayed:   int32(q.Statistics.Delayed),
			Waiting:   int32(q.Statistics.Waiting),
			Paused:    int32(q.Statistics.Paused),
		},
	}
}

func queueNameToProto(name queue.QueueName) immichv1.QueueName {
	switch name {
	case queue.QueueThumbnailGeneration:
		return immichv1.QueueName_QUEUE_NAME_THUMBNAIL_GENERATION
	case queue.QueueMetadataExtraction:
		return immichv1.QueueName_QUEUE_NAME_METADATA_EXTRACTION
	case queue.QueueVideoConversion:
		return immichv1.QueueName_QUEUE_NAME_VIDEO_CONVERSION
	case queue.QueueFaceDetection:
		return immichv1.QueueName_QUEUE_NAME_FACE_DETECTION
	case queue.QueueFacialRecognition:
		return immichv1.QueueName_QUEUE_NAME_FACIAL_RECOGNITION
	case queue.QueueSmartSearch:
		return immichv1.QueueName_QUEUE_NAME_SMART_SEARCH
	case queue.QueueDuplicateDetection:
		return immichv1.QueueName_QUEUE_NAME_DUPLICATE_DETECTION
	case queue.QueueBackgroundTask:
		return immichv1.QueueName_QUEUE_NAME_BACKGROUND_TASK
	case queue.QueueStorageMigration:
		return immichv1.QueueName_QUEUE_NAME_STORAGE_MIGRATION
	case queue.QueueSearch:
		return immichv1.QueueName_QUEUE_NAME_SEARCH
	case queue.QueueSidecar:
		return immichv1.QueueName_QUEUE_NAME_SIDECAR
	case queue.QueueLibrary:
		return immichv1.QueueName_QUEUE_NAME_LIBRARY
	case queue.QueueNotification:
		return immichv1.QueueName_QUEUE_NAME_NOTIFICATION
	default:
		return immichv1.QueueName_QUEUE_NAME_UNSPECIFIED
	}
}

func protoStatusToInternal(s immichv1.QueueJobStatus) queue.JobStatus {
	switch s {
	case immichv1.QueueJobStatus_QUEUE_JOB_STATUS_ACTIVE:
		return queue.JobStatusActive
	case immichv1.QueueJobStatus_QUEUE_JOB_STATUS_COMPLETED:
		return queue.JobStatusCompleted
	case immichv1.QueueJobStatus_QUEUE_JOB_STATUS_FAILED:
		return queue.JobStatusFailed
	case immichv1.QueueJobStatus_QUEUE_JOB_STATUS_DELAYED:
		return queue.JobStatusDelayed
	case immichv1.QueueJobStatus_QUEUE_JOB_STATUS_WAITING:
		return queue.JobStatusWaiting
	case immichv1.QueueJobStatus_QUEUE_JOB_STATUS_PAUSED:
		return queue.JobStatusPaused
	default:
		return queue.JobStatusWaiting
	}
}

func internalStatusToProto(s queue.JobStatus) immichv1.QueueJobStatus {
	switch s {
	case queue.JobStatusActive:
		return immichv1.QueueJobStatus_QUEUE_JOB_STATUS_ACTIVE
	case queue.JobStatusCompleted:
		return immichv1.QueueJobStatus_QUEUE_JOB_STATUS_COMPLETED
	case queue.JobStatusFailed:
		return immichv1.QueueJobStatus_QUEUE_JOB_STATUS_FAILED
	case queue.JobStatusDelayed:
		return immichv1.QueueJobStatus_QUEUE_JOB_STATUS_DELAYED
	case queue.JobStatusWaiting:
		return immichv1.QueueJobStatus_QUEUE_JOB_STATUS_WAITING
	case queue.JobStatusPaused:
		return immichv1.QueueJobStatus_QUEUE_JOB_STATUS_PAUSED
	default:
		return immichv1.QueueJobStatus_QUEUE_JOB_STATUS_UNSPECIFIED
	}
}

func jobToProto(job *queue.Job) *immichv1.QueueJob {
	dataBytes, _ := json.Marshal(job.Data)
	dataStr := string(dataBytes)

	protoJob := &immichv1.QueueJob{
		Id:        job.ID,
		Name:      job.Name,
		Data:      dataStr,
		Timestamp: job.Timestamp.Unix(),
		Status:    internalStatusToProto(job.Status),
	}

	if job.Error != "" {
		protoJob.Error = &job.Error
	}

	if job.Attempts > 0 {
		attempts := int32(job.Attempts)
		protoJob.Attempts = &attempts
	}

	return protoJob
}
