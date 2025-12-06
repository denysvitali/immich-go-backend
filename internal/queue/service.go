package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("immich-go-backend/queue")

// QueueName represents the name of a job queue
type QueueName string

const (
	QueueThumbnailGeneration QueueName = "thumbnailGeneration"
	QueueMetadataExtraction  QueueName = "metadataExtraction"
	QueueVideoConversion     QueueName = "videoConversion"
	QueueFaceDetection       QueueName = "faceDetection"
	QueueFacialRecognition   QueueName = "facialRecognition"
	QueueSmartSearch         QueueName = "smartSearch"
	QueueDuplicateDetection  QueueName = "duplicateDetection"
	QueueBackgroundTask      QueueName = "backgroundTask"
	QueueStorageMigration    QueueName = "storageMigration"
	QueueSearch              QueueName = "search"
	QueueSidecar             QueueName = "sidecar"
	QueueLibrary             QueueName = "library"
	QueueNotification        QueueName = "notification"
)

// AllQueues returns all available queue names
func AllQueues() []QueueName {
	return []QueueName{
		QueueThumbnailGeneration,
		QueueMetadataExtraction,
		QueueVideoConversion,
		QueueFaceDetection,
		QueueFacialRecognition,
		QueueSmartSearch,
		QueueDuplicateDetection,
		QueueBackgroundTask,
		QueueStorageMigration,
		QueueSearch,
		QueueSidecar,
		QueueLibrary,
		QueueNotification,
	}
}

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusActive    JobStatus = "active"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusDelayed   JobStatus = "delayed"
	JobStatusWaiting   JobStatus = "waiting"
	JobStatusPaused    JobStatus = "paused"
)

// QueueStatistics represents statistics for a queue
type QueueStatistics struct {
	Active    int `json:"active"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Delayed   int `json:"delayed"`
	Waiting   int `json:"waiting"`
	Paused    int `json:"paused"`
}

// QueueInfo represents information about a queue
type QueueInfo struct {
	Name       QueueName       `json:"name"`
	IsPaused   bool            `json:"isPaused"`
	Statistics QueueStatistics `json:"statistics"`
}

// Job represents a job in a queue
type Job struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	Status    JobStatus   `json:"status"`
	Error     string      `json:"error,omitempty"`
	Attempts  int         `json:"attempts"`
}

// Service handles job queue management
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// In-memory queue state (in production, this would be backed by Redis/BullMQ)
	mu       sync.RWMutex
	queues   map[QueueName]*queueState
	jobs     map[string]*Job
	jobQueue map[QueueName][]*Job
}

// queueState represents the state of a queue
type queueState struct {
	IsPaused   bool
	Statistics QueueStatistics
}

// NewService creates a new queue service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	s := &Service{
		db:       queries,
		config:   cfg,
		queues:   make(map[QueueName]*queueState),
		jobs:     make(map[string]*Job),
		jobQueue: make(map[QueueName][]*Job),
	}

	// Initialize all queues
	for _, name := range AllQueues() {
		s.queues[name] = &queueState{
			IsPaused: false,
			Statistics: QueueStatistics{
				Active:    0,
				Completed: 0,
				Failed:    0,
				Delayed:   0,
				Waiting:   0,
				Paused:    0,
			},
		}
		s.jobQueue[name] = make([]*Job, 0)
	}

	return s, nil
}

// GetAllQueues returns information about all queues
func (s *Service) GetAllQueues(ctx context.Context) ([]QueueInfo, error) {
	ctx, span := tracer.Start(ctx, "queue.get_all")
	defer span.End()

	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []QueueInfo
	for _, name := range AllQueues() {
		state := s.queues[name]
		result = append(result, QueueInfo{
			Name:       name,
			IsPaused:   state.IsPaused,
			Statistics: state.Statistics,
		})
	}

	span.SetAttributes(attribute.Int("queue_count", len(result)))

	return result, nil
}

// GetQueue returns information about a specific queue
func (s *Service) GetQueue(ctx context.Context, name string) (*QueueInfo, error) {
	ctx, span := tracer.Start(ctx, "queue.get",
		trace.WithAttributes(attribute.String("queue_name", name)))
	defer span.End()

	s.mu.RLock()
	defer s.mu.RUnlock()

	queueName := QueueName(name)
	state, ok := s.queues[queueName]
	if !ok {
		return nil, fmt.Errorf("queue not found: %s", name)
	}

	return &QueueInfo{
		Name:       queueName,
		IsPaused:   state.IsPaused,
		Statistics: state.Statistics,
	}, nil
}

// UpdateQueue updates a queue (pause/resume)
func (s *Service) UpdateQueue(ctx context.Context, name string, isPaused *bool) (*QueueInfo, error) {
	ctx, span := tracer.Start(ctx, "queue.update",
		trace.WithAttributes(attribute.String("queue_name", name)))
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	queueName := QueueName(name)
	state, ok := s.queues[queueName]
	if !ok {
		return nil, fmt.Errorf("queue not found: %s", name)
	}

	if isPaused != nil {
		state.IsPaused = *isPaused
		span.SetAttributes(attribute.Bool("is_paused", *isPaused))
	}

	return &QueueInfo{
		Name:       queueName,
		IsPaused:   state.IsPaused,
		Statistics: state.Statistics,
	}, nil
}

// GetQueueJobs returns jobs from a specific queue
func (s *Service) GetQueueJobs(ctx context.Context, name string, statuses []JobStatus, limit, offset int) ([]*Job, int, error) {
	ctx, span := tracer.Start(ctx, "queue.get_jobs",
		trace.WithAttributes(
			attribute.String("queue_name", name),
			attribute.Int("limit", limit),
			attribute.Int("offset", offset),
		))
	defer span.End()

	s.mu.RLock()
	defer s.mu.RUnlock()

	queueName := QueueName(name)
	jobs, ok := s.jobQueue[queueName]
	if !ok {
		return nil, 0, fmt.Errorf("queue not found: %s", name)
	}

	// Filter by status if specified
	var filtered []*Job
	for _, job := range jobs {
		if len(statuses) == 0 {
			filtered = append(filtered, job)
		} else {
			for _, status := range statuses {
				if job.Status == status {
					filtered = append(filtered, job)
					break
				}
			}
		}
	}

	total := len(filtered)

	// Apply pagination
	if offset >= len(filtered) {
		return []*Job{}, total, nil
	}

	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	result := filtered[offset:end]
	span.SetAttributes(attribute.Int("result_count", len(result)))
	span.SetAttributes(attribute.Int("total_count", total))

	return result, total, nil
}

// ClearQueueJobs clears jobs from a queue
func (s *Service) ClearQueueJobs(ctx context.Context, name string, includeFailed bool) error {
	ctx, span := tracer.Start(ctx, "queue.clear_jobs",
		trace.WithAttributes(
			attribute.String("queue_name", name),
			attribute.Bool("include_failed", includeFailed),
		))
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	queueName := QueueName(name)
	_, ok := s.queues[queueName]
	if !ok {
		return fmt.Errorf("queue not found: %s", name)
	}

	if includeFailed {
		// Clear all jobs
		s.jobQueue[queueName] = make([]*Job, 0)
		s.queues[queueName].Statistics = QueueStatistics{}
	} else {
		// Keep only failed jobs
		var failedJobs []*Job
		for _, job := range s.jobQueue[queueName] {
			if job.Status == JobStatusFailed {
				failedJobs = append(failedJobs, job)
			}
		}
		s.jobQueue[queueName] = failedJobs

		// Update statistics
		s.queues[queueName].Statistics = QueueStatistics{
			Failed: len(failedJobs),
		}
	}

	span.SetAttributes(attribute.Bool("jobs_cleared", true))

	return nil
}

// AddJob adds a job to a queue
func (s *Service) AddJob(ctx context.Context, queueName QueueName, jobName string, data interface{}) (string, error) {
	ctx, span := tracer.Start(ctx, "queue.add_job",
		trace.WithAttributes(
			attribute.String("queue_name", string(queueName)),
			attribute.String("job_name", jobName),
		))
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.queues[queueName]
	if !ok {
		return "", fmt.Errorf("queue not found: %s", queueName)
	}

	jobID := uuid.New().String()

	job := &Job{
		ID:        jobID,
		Name:      jobName,
		Data:      data,
		Timestamp: time.Now(),
		Status:    JobStatusWaiting,
		Attempts:  0,
	}

	s.jobs[jobID] = job
	s.jobQueue[queueName] = append(s.jobQueue[queueName], job)
	state.Statistics.Waiting++

	span.SetAttributes(attribute.String("job_id", jobID))

	return jobID, nil
}

// GetJob returns a job by ID
func (s *Service) GetJob(ctx context.Context, jobID string) (*Job, error) {
	ctx, span := tracer.Start(ctx, "queue.get_job",
		trace.WithAttributes(attribute.String("job_id", jobID)))
	defer span.End()

	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	return job, nil
}

// UpdateJobStatus updates the status of a job
func (s *Service) UpdateJobStatus(ctx context.Context, jobID string, status JobStatus, errorMsg string) error {
	ctx, span := tracer.Start(ctx, "queue.update_job_status",
		trace.WithAttributes(
			attribute.String("job_id", jobID),
			attribute.String("status", string(status)),
		))
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	oldStatus := job.Status
	job.Status = status
	if errorMsg != "" {
		job.Error = errorMsg
	}

	// Update queue statistics
	for queueName, jobs := range s.jobQueue {
		for _, j := range jobs {
			if j.ID == jobID {
				state := s.queues[queueName]
				s.updateStatistics(state, oldStatus, status)
				break
			}
		}
	}

	return nil
}

// updateStatistics updates queue statistics when job status changes
func (s *Service) updateStatistics(state *queueState, oldStatus, newStatus JobStatus) {
	// Decrement old status count
	switch oldStatus {
	case JobStatusActive:
		state.Statistics.Active--
	case JobStatusCompleted:
		state.Statistics.Completed--
	case JobStatusFailed:
		state.Statistics.Failed--
	case JobStatusDelayed:
		state.Statistics.Delayed--
	case JobStatusWaiting:
		state.Statistics.Waiting--
	case JobStatusPaused:
		state.Statistics.Paused--
	}

	// Increment new status count
	switch newStatus {
	case JobStatusActive:
		state.Statistics.Active++
	case JobStatusCompleted:
		state.Statistics.Completed++
	case JobStatusFailed:
		state.Statistics.Failed++
	case JobStatusDelayed:
		state.Statistics.Delayed++
	case JobStatusWaiting:
		state.Statistics.Waiting++
	case JobStatusPaused:
		state.Statistics.Paused++
	}
}

// SerializeJobData serializes job data to JSON string
func SerializeJobData(data interface{}) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
