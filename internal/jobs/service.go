package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/sirupsen/logrus"
)

// JobType represents the type of job to be processed
type JobType string

const (
	// Asset processing jobs
	JobTypeThumbnailGeneration JobType = "thumbnail_generation"
	JobTypeMetadataExtraction  JobType = "metadata_extraction"
	JobTypeVideoTranscode      JobType = "video_transcode"
	JobTypeAssetOptimization   JobType = "asset_optimization"

	// Machine learning jobs
	JobTypeFaceDetection   JobType = "face_detection"
	JobTypeFaceRecognition JobType = "face_recognition"
	JobTypeSmartSearch     JobType = "smart_search_indexing"
	JobTypeObjectDetection JobType = "object_detection"

	// Library jobs
	JobTypeLibraryScan     JobType = "library_scan"
	JobTypeLibraryWatch    JobType = "library_watch"
	JobTypeDuplicateDetect JobType = "duplicate_detection"
	JobTypeSidecarProcess  JobType = "sidecar_processing"

	// System jobs
	JobTypeStorageMigration JobType = "storage_migration"
	JobTypeCleanup          JobType = "cleanup"
	JobTypeBackup           JobType = "backup"
)

// JobPriority represents the priority level of a job
type JobPriority int

const (
	PriorityLow      JobPriority = 1
	PriorityNormal   JobPriority = 5
	PriorityHigh     JobPriority = 10
	PriorityCritical JobPriority = 20
)

// Service handles job queue management
type Service struct {
	client    *asynq.Client
	server    *asynq.Server
	inspector *asynq.Inspector
	logger    *logrus.Logger
	handlers  map[string]func(context.Context, *asynq.Task) error
}

// Config holds job queue configuration
type Config struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Concurrency   int
	QueueName     string
}

// NewService creates a new job queue service
func NewService(cfg *Config) (*Service, error) {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	client := asynq.NewClient(redisOpt)

	serverCfg := asynq.Config{
		Concurrency: cfg.Concurrency,
		Queues: map[string]int{
			"critical": 6,
			"high":     3,
			"normal":   2,
			"low":      1,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			logrus.WithFields(logrus.Fields{
				"type":    task.Type(),
				"payload": string(task.Payload()),
				"error":   err,
			}).Error("Job processing failed")
		}),
	}

	server := asynq.NewServer(redisOpt, serverCfg)
	inspector := asynq.NewInspector(redisOpt)

	return &Service{
		client:    client,
		server:    server,
		inspector: inspector,
		logger:    logrus.StandardLogger(),
		handlers:  make(map[string]func(context.Context, *asynq.Task) error),
	}, nil
}

// JobPayload represents the data for a job
type JobPayload struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at"`
}

// EnqueueJob adds a new job to the queue
func (s *Service) EnqueueJob(ctx context.Context, jobType JobType, payload *JobPayload, opts ...asynq.Option) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask(string(jobType), data)

	info, err := s.client.EnqueueContext(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"job_id":   info.ID,
		"job_type": jobType,
		"queue":    info.Queue,
	}).Info("Job enqueued successfully")

	return nil
}

// EnqueueJobWithPriority adds a job with specific priority
func (s *Service) EnqueueJobWithPriority(ctx context.Context, jobType JobType, payload *JobPayload, priority JobPriority) error {
	queue := s.getQueueByPriority(priority)
	opts := []asynq.Option{
		asynq.Queue(queue),
		asynq.MaxRetry(3),
		asynq.Timeout(30 * time.Minute),
	}

	return s.EnqueueJob(ctx, jobType, payload, opts...)
}

// ScheduleJob schedules a job to run at a specific time
func (s *Service) ScheduleJob(ctx context.Context, jobType JobType, payload *JobPayload, processAt time.Time) error {
	opts := []asynq.Option{
		asynq.ProcessAt(processAt),
		asynq.MaxRetry(3),
	}

	return s.EnqueueJob(ctx, jobType, payload, opts...)
}

// GetJobStatus retrieves the status of a job
func (s *Service) GetJobStatus(ctx context.Context, jobID string) (*JobStatus, error) {
	// Check pending jobs
	pending, err := s.inspector.GetTaskInfo("normal", jobID)
	if err == nil && pending != nil {
		return &JobStatus{
			ID:        pending.ID,
			Type:      pending.Type,
			State:     "pending",
			CreatedAt: pending.NextProcessAt,
		}, nil
	}

	// Check active jobs
	active, err := s.inspector.ListActiveTasks("normal")
	if err == nil {
		for _, task := range active {
			if task.ID == jobID {
				return &JobStatus{
					ID:        task.ID,
					Type:      task.Type,
					State:     "processing",
					CreatedAt: task.NextProcessAt,
				}, nil
			}
		}
	}

	// Check completed jobs would require additional tracking
	return nil, fmt.Errorf("job not found: %s", jobID)
}

// JobStatus represents the current status of a job
type JobStatus struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	State       string     `json:"state"`
	Progress    int        `json:"progress"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// GetQueueStats returns statistics for all queues
func (s *Service) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	queues := []string{"critical", "high", "normal", "low"}
	stats := &QueueStats{
		Queues: make(map[string]*QueueInfo),
	}

	for _, queue := range queues {
		info, err := s.inspector.GetQueueInfo(queue)
		if err != nil {
			s.logger.WithError(err).Warnf("Failed to get stats for queue: %s", queue)
			continue
		}

		stats.Queues[queue] = &QueueInfo{
			Name:      queue,
			Size:      info.Size,
			Active:    info.Active,
			Pending:   info.Pending,
			Scheduled: info.Scheduled,
			Retry:     info.Retry,
			Archived:  info.Archived,
			Completed: info.Completed,
		}
	}

	return stats, nil
}

// QueueStats represents statistics for all queues
type QueueStats struct {
	Queues map[string]*QueueInfo `json:"queues"`
}

// QueueInfo represents statistics for a single queue
type QueueInfo struct {
	Name      string `json:"name"`
	Size      int    `json:"size"`
	Active    int    `json:"active"`
	Pending   int    `json:"pending"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Archived  int    `json:"archived"`
	Completed int    `json:"completed"`
}

// PauseQueue pauses processing of a specific queue
func (s *Service) PauseQueue(ctx context.Context, queue string) error {
	return s.inspector.PauseQueue(queue)
}

// ResumeQueue resumes processing of a paused queue
func (s *Service) ResumeQueue(ctx context.Context, queue string) error {
	return s.inspector.UnpauseQueue(queue)
}

// ClearQueue removes all jobs from a queue
func (s *Service) ClearQueue(ctx context.Context, queue string) error {
	// Archive all pending tasks
	_, err := s.inspector.ArchiveAllPendingTasks(queue)
	if err != nil {
		return fmt.Errorf("failed to clear queue: %w", err)
	}

	// Delete archived tasks
	_, err = s.inspector.DeleteAllArchivedTasks(queue)
	return err
}

// RegisterHandler registers a job handler for a specific job type
func (s *Service) RegisterHandler(jobType JobType, handler func(context.Context, *asynq.Task) error) {
	s.handlers[string(jobType)] = handler
}

// Start begins processing jobs
func (s *Service) Start() error {
	s.logger.Info("Starting job queue server")
	mux := asynq.NewServeMux()
	for jobType, handler := range s.handlers {
		mux.HandleFunc(jobType, handler)
	}
	// Add default handler for unregistered job types
	mux.HandleFunc("*", s.processJob)
	return s.server.Start(mux)
}

// Stop gracefully stops the job queue server
func (s *Service) Stop() {
	s.logger.Info("Stopping job queue server")
	s.server.Stop()
	s.server.Shutdown()
	s.client.Close()
}

// processJob is the main job processor
func (s *Service) processJob(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"job_type": task.Type(),
		"job_id":   payload.ID,
		"user_id":  payload.UserID,
	}).Info("Processing job")

	// Job processing logic would be implemented here based on job type
	// For now, we'll just log that we're processing it

	return nil
}

// getQueueByPriority returns the queue name based on priority
func (s *Service) getQueueByPriority(priority JobPriority) string {
	switch {
	case priority >= PriorityCritical:
		return "critical"
	case priority >= PriorityHigh:
		return "high"
	case priority >= PriorityNormal:
		return "normal"
	default:
		return "low"
	}
}

// JobRequest represents a request to create a job
type JobRequest struct {
	Type     JobType                `json:"type"`
	Priority JobPriority            `json:"priority"`
	Data     map[string]interface{} `json:"data"`
	Delay    *time.Duration         `json:"delay,omitempty"`
}

// CreateJob creates and enqueues a new job
func (s *Service) CreateJob(ctx context.Context, userID string, req *JobRequest) (string, error) {
	payload := &JobPayload{
		ID:        fmt.Sprintf("%s-%d", req.Type, time.Now().UnixNano()),
		UserID:    userID,
		Data:      req.Data,
		CreatedAt: time.Now(),
	}

	if req.Delay != nil {
		// Schedule the job with delay
		processAt := time.Now().Add(*req.Delay)
		return payload.ID, s.ScheduleJob(ctx, req.Type, payload, processAt)
	}

	// Enqueue immediately with priority
	return payload.ID, s.EnqueueJobWithPriority(ctx, req.Type, payload, req.Priority)
}
