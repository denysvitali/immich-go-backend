package server

import (
	"context"
	"math"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/jobs"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// jobNameQueues maps each Immich job name to the asynq priority queue its work
// runs on. The backend multiplexes all job types onto four priority queues
// (critical/high/normal/low), so the reported counts and pause/resume/clear
// commands operate on the underlying priority queue shared by the job types in
// the same group.
var jobNameQueues = map[immichv1.JobName]string{
	immichv1.JobName_JOB_NAME_THUMBNAIL_GENERATION:       "high",
	immichv1.JobName_JOB_NAME_METADATA_EXTRACTION:        "high",
	immichv1.JobName_JOB_NAME_CLIP_ENCODING:              "normal",
	immichv1.JobName_JOB_NAME_DUPLICATE_DETECTION:        "normal",
	immichv1.JobName_JOB_NAME_FACE_DETECTION:             "normal",
	immichv1.JobName_JOB_NAME_FACIAL_RECOGNITION:         "normal",
	immichv1.JobName_JOB_NAME_LIBRARY:                    "normal",
	immichv1.JobName_JOB_NAME_MIGRATION:                  "normal",
	immichv1.JobName_JOB_NAME_NOTIFICATION:               "normal",
	immichv1.JobName_JOB_NAME_SEARCH:                     "normal",
	immichv1.JobName_JOB_NAME_SIDECAR:                    "normal",
	immichv1.JobName_JOB_NAME_SMART_SEARCH:               "normal",
	immichv1.JobName_JOB_NAME_STORAGE_TEMPLATE_MIGRATION: "normal",
	immichv1.JobName_JOB_NAME_BACKGROUND_TASK:            "low",
	immichv1.JobName_JOB_NAME_VIDEO_CONVERSION:           "low",
}

// jobIDNames maps the camelCase job queue identifiers used by the Immich REST
// API (PUT /api/jobs/{id}) to the proto job name enum.
var jobIDNames = map[string]immichv1.JobName{
	"backgroundTask":           immichv1.JobName_JOB_NAME_BACKGROUND_TASK,
	"clipEncoding":             immichv1.JobName_JOB_NAME_CLIP_ENCODING,
	"duplicateDetection":       immichv1.JobName_JOB_NAME_DUPLICATE_DETECTION,
	"faceDetection":            immichv1.JobName_JOB_NAME_FACE_DETECTION,
	"facialRecognition":        immichv1.JobName_JOB_NAME_FACIAL_RECOGNITION,
	"library":                  immichv1.JobName_JOB_NAME_LIBRARY,
	"metadataExtraction":       immichv1.JobName_JOB_NAME_METADATA_EXTRACTION,
	"migration":                immichv1.JobName_JOB_NAME_MIGRATION,
	"notification":             immichv1.JobName_JOB_NAME_NOTIFICATION,
	"notifications":            immichv1.JobName_JOB_NAME_NOTIFICATION,
	"search":                   immichv1.JobName_JOB_NAME_SEARCH,
	"sidecar":                  immichv1.JobName_JOB_NAME_SIDECAR,
	"smartSearch":              immichv1.JobName_JOB_NAME_SMART_SEARCH,
	"storageTemplateMigration": immichv1.JobName_JOB_NAME_STORAGE_TEMPLATE_MIGRATION,
	"thumbnailGeneration":      immichv1.JobName_JOB_NAME_THUMBNAIL_GENERATION,
	"videoConversion":          immichv1.JobName_JOB_NAME_VIDEO_CONVERSION,
}

// parseJobID resolves a job identifier from the REST path (camelCase, e.g.
// "thumbnailGeneration") or the proto enum name (e.g.
// "JOB_NAME_THUMBNAIL_GENERATION") to the proto job name enum.
func parseJobID(id string) (immichv1.JobName, bool) {
	if name, ok := jobIDNames[id]; ok {
		return name, true
	}
	if v, ok := immichv1.JobName_value[id]; ok && v != int32(immichv1.JobName_JOB_NAME_UNSPECIFIED) {
		return immichv1.JobName(v), true
	}
	return immichv1.JobName_JOB_NAME_UNSPECIFIED, false
}

// clampInt32 converts an int to int32, saturating at the int32 bounds.
func clampInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

// emptyJobStatus returns a job status DTO with all-zero counts, used when the
// job service is not configured (no Redis) or a queue has no recorded stats.
func emptyJobStatus() *immichv1.JobStatusDto {
	return &immichv1.JobStatusDto{
		IsActive:    false,
		IsPaused:    false,
		QueueStatus: &immichv1.QueueStatusDto{},
	}
}

// jobStatusFromQueueInfo converts asynq queue statistics into the Immich job
// status DTO shape. asynq semantics map as follows: pending -> waiting,
// scheduled + retry -> delayed, archived (retries exhausted) -> failed.
func jobStatusFromQueueInfo(info *jobs.QueueInfo) *immichv1.JobStatusDto {
	if info == nil {
		return emptyJobStatus()
	}
	return &immichv1.JobStatusDto{
		IsActive: info.Active > 0,
		IsPaused: info.Paused,
		QueueStatus: &immichv1.QueueStatusDto{
			Active:    clampInt32(info.Active),
			Completed: clampInt32(info.Completed),
			Delayed:   clampInt32(info.Scheduled + info.Retry),
			Failed:    clampInt32(info.Archived),
			Paused:    0,
			Waiting:   clampInt32(info.Pending),
		},
	}
}

// jobStatusForQueue fetches current statistics for a single asynq queue.
func (s *Server) jobStatusForQueue(ctx context.Context, queue string) (*immichv1.JobStatusDto, error) {
	stats, err := s.jobService.GetQueueStats(ctx)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get queue stats", err)
	}
	return jobStatusFromQueueInfo(stats.Queues[queue]), nil
}

// GetAllJobStatuses returns the status of every job queue. When the job
// service is not configured (no Redis), it returns all-zero counts so the
// admin UI can still poll this endpoint.
func (s *Server) GetAllJobStatuses(ctx context.Context, _ *emptypb.Empty) (*immichv1.AllJobStatusResponseDto, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	statuses := make(map[immichv1.JobName]*immichv1.JobStatusDto, len(jobNameQueues))
	if s.jobService == nil {
		for name := range jobNameQueues {
			statuses[name] = emptyJobStatus()
		}
	} else {
		stats, err := s.jobService.GetQueueStats(ctx)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to get queue stats", err)
		}
		for name, queue := range jobNameQueues {
			statuses[name] = jobStatusFromQueueInfo(stats.Queues[queue])
		}
	}

	return &immichv1.AllJobStatusResponseDto{
		BackgroundTask:           statuses[immichv1.JobName_JOB_NAME_BACKGROUND_TASK],
		ClipEncoding:             statuses[immichv1.JobName_JOB_NAME_CLIP_ENCODING],
		DuplicateDetection:       statuses[immichv1.JobName_JOB_NAME_DUPLICATE_DETECTION],
		FaceDetection:            statuses[immichv1.JobName_JOB_NAME_FACE_DETECTION],
		FacialRecognition:        statuses[immichv1.JobName_JOB_NAME_FACIAL_RECOGNITION],
		Library:                  statuses[immichv1.JobName_JOB_NAME_LIBRARY],
		MetadataExtraction:       statuses[immichv1.JobName_JOB_NAME_METADATA_EXTRACTION],
		Migration:                statuses[immichv1.JobName_JOB_NAME_MIGRATION],
		Notification:             statuses[immichv1.JobName_JOB_NAME_NOTIFICATION],
		Search:                   statuses[immichv1.JobName_JOB_NAME_SEARCH],
		Sidecar:                  statuses[immichv1.JobName_JOB_NAME_SIDECAR],
		SmartSearch:              statuses[immichv1.JobName_JOB_NAME_SMART_SEARCH],
		StorageTemplateMigration: statuses[immichv1.JobName_JOB_NAME_STORAGE_TEMPLATE_MIGRATION],
		ThumbnailGeneration:      statuses[immichv1.JobName_JOB_NAME_THUMBNAIL_GENERATION],
		VideoConversion:          statuses[immichv1.JobName_JOB_NAME_VIDEO_CONVERSION],
	}, nil
}

// SendJobCommand applies a command (start/pause/resume/empty/clear-failed) to
// the queue backing the given job name and returns the queue's status.
func (s *Server) SendJobCommand(ctx context.Context, request *immichv1.SendJobCommandRequest) (*immichv1.JobStatusDto, error) {
	claims, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if s.jobService == nil {
		return nil, status.Error(codes.Unavailable, "job service not configured")
	}

	jobName, ok := parseJobID(request.GetId())
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unknown job name: %q", request.GetId())
	}
	queue := jobNameQueues[jobName]

	switch cmd := request.GetCommand().GetCommand(); cmd {
	case "start":
		if err := s.startJob(ctx, claims, jobName); err != nil {
			return nil, err
		}
	case "pause":
		if err := s.jobService.PauseQueue(ctx, queue); err != nil {
			return nil, SanitizedInternal(ctx, "failed to pause queue", err)
		}
	case "resume":
		if err := s.jobService.ResumeQueue(ctx, queue); err != nil {
			return nil, SanitizedInternal(ctx, "failed to resume queue", err)
		}
	case "empty":
		if err := s.jobService.ClearQueue(ctx, queue); err != nil {
			return nil, SanitizedInternal(ctx, "failed to empty queue", err)
		}
	case "clear-failed":
		if err := s.jobService.ClearFailed(ctx, queue); err != nil {
			return nil, SanitizedInternal(ctx, "failed to clear failed jobs", err)
		}
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown job command: %q", cmd)
	}

	return s.jobStatusForQueue(ctx, queue)
}

// startJob enqueues a queue-wide run for job types whose payloads can be
// derived from the request context. Job types that require a per-asset payload
// have no queue-wide start capability in the job service.
func (s *Server) startJob(ctx context.Context, claims *auth.Claims, jobName immichv1.JobName) error {
	switch jobName {
	case immichv1.JobName_JOB_NAME_DUPLICATE_DETECTION:
		payload := jobs.DuplicateDetectionPayload{UserID: claims.UserID}
		if err := s.jobService.EnqueueJobWithPriority(ctx, jobs.JobTypeDuplicateDetect, payload, jobs.PriorityNormal); err != nil {
			return SanitizedInternal(ctx, "failed to enqueue duplicate detection job", err)
		}
		return nil
	default:
		return status.Errorf(codes.Unimplemented,
			"queue-wide start is not supported for job %q: its jobs require per-asset payloads and are enqueued automatically on upload", jobName)
	}
}

// ClearJob empties the queue backing the given job name and returns the
// queue's status.
func (s *Server) ClearJob(ctx context.Context, request *immichv1.ClearJobRequest) (*immichv1.JobStatusDto, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if s.jobService == nil {
		return nil, status.Error(codes.Unavailable, "job service not configured")
	}

	queue, ok := jobNameQueues[request.GetId()]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unknown job name: %v", request.GetId())
	}

	if err := s.jobService.ClearQueue(ctx, queue); err != nil {
		return nil, SanitizedInternal(ctx, "failed to clear queue", err)
	}

	return s.jobStatusForQueue(ctx, queue)
}
