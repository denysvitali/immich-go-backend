package server

import (
	"context"
	"encoding/json"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/queue"
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
		return nil, SanitizedInternal(ctx, "failed to get queues", err)
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

var queueNameProtoValues = map[queue.QueueName]immichv1.QueueName{
	queue.QueueThumbnailGeneration: immichv1.QueueName_QUEUE_NAME_THUMBNAIL_GENERATION,
	queue.QueueMetadataExtraction:  immichv1.QueueName_QUEUE_NAME_METADATA_EXTRACTION,
	queue.QueueVideoConversion:     immichv1.QueueName_QUEUE_NAME_VIDEO_CONVERSION,
	queue.QueueFaceDetection:       immichv1.QueueName_QUEUE_NAME_FACE_DETECTION,
	queue.QueueFacialRecognition:   immichv1.QueueName_QUEUE_NAME_FACIAL_RECOGNITION,
	queue.QueueSmartSearch:         immichv1.QueueName_QUEUE_NAME_SMART_SEARCH,
	queue.QueueDuplicateDetection:  immichv1.QueueName_QUEUE_NAME_DUPLICATE_DETECTION,
	queue.QueueBackgroundTask:      immichv1.QueueName_QUEUE_NAME_BACKGROUND_TASK,
	queue.QueueStorageMigration:    immichv1.QueueName_QUEUE_NAME_STORAGE_MIGRATION,
	queue.QueueSearch:              immichv1.QueueName_QUEUE_NAME_SEARCH,
	queue.QueueSidecar:             immichv1.QueueName_QUEUE_NAME_SIDECAR,
	queue.QueueLibrary:             immichv1.QueueName_QUEUE_NAME_LIBRARY,
	queue.QueueNotification:        immichv1.QueueName_QUEUE_NAME_NOTIFICATION,
}

func queueNameToProto(name queue.QueueName) immichv1.QueueName {
	if protoName, ok := queueNameProtoValues[name]; ok {
		return protoName
	}
	return immichv1.QueueName_QUEUE_NAME_UNSPECIFIED
}

var protoStatusValues = map[immichv1.QueueJobStatus]queue.JobStatus{
	immichv1.QueueJobStatus_QUEUE_JOB_STATUS_ACTIVE:    queue.JobStatusActive,
	immichv1.QueueJobStatus_QUEUE_JOB_STATUS_COMPLETED: queue.JobStatusCompleted,
	immichv1.QueueJobStatus_QUEUE_JOB_STATUS_FAILED:    queue.JobStatusFailed,
	immichv1.QueueJobStatus_QUEUE_JOB_STATUS_DELAYED:   queue.JobStatusDelayed,
	immichv1.QueueJobStatus_QUEUE_JOB_STATUS_WAITING:   queue.JobStatusWaiting,
	immichv1.QueueJobStatus_QUEUE_JOB_STATUS_PAUSED:    queue.JobStatusPaused,
}

func protoStatusToInternal(s immichv1.QueueJobStatus) queue.JobStatus {
	if status, ok := protoStatusValues[s]; ok {
		return status
	}
	return queue.JobStatusWaiting
}

var internalStatusProtoValues = map[queue.JobStatus]immichv1.QueueJobStatus{
	queue.JobStatusActive:    immichv1.QueueJobStatus_QUEUE_JOB_STATUS_ACTIVE,
	queue.JobStatusCompleted: immichv1.QueueJobStatus_QUEUE_JOB_STATUS_COMPLETED,
	queue.JobStatusFailed:    immichv1.QueueJobStatus_QUEUE_JOB_STATUS_FAILED,
	queue.JobStatusDelayed:   immichv1.QueueJobStatus_QUEUE_JOB_STATUS_DELAYED,
	queue.JobStatusWaiting:   immichv1.QueueJobStatus_QUEUE_JOB_STATUS_WAITING,
	queue.JobStatusPaused:    immichv1.QueueJobStatus_QUEUE_JOB_STATUS_PAUSED,
}

func internalStatusToProto(s queue.JobStatus) immichv1.QueueJobStatus {
	if protoStatus, ok := internalStatusProtoValues[s]; ok {
		return protoStatus
	}
	return immichv1.QueueJobStatus_QUEUE_JOB_STATUS_UNSPECIFIED
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
