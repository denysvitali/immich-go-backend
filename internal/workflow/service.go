package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var tracer = telemetry.GetTracer("workflow")

// TriggerType represents the type of workflow trigger
type TriggerType string

const (
	TriggerTypeAssetUploaded  TriggerType = "asset_uploaded"
	TriggerTypeAssetDeleted   TriggerType = "asset_deleted"
	TriggerTypeAlbumCreated   TriggerType = "album_created"
	TriggerTypeAlbumUpdated   TriggerType = "album_updated"
	TriggerTypeUserCreated    TriggerType = "user_created"
	TriggerTypeScheduled      TriggerType = "scheduled"
	TriggerTypeManual         TriggerType = "manual"
	TriggerTypeFaceDetected   TriggerType = "face_detected"
	TriggerTypeDuplicateFound TriggerType = "duplicate_found"
)

// ActionType represents the type of workflow action
type ActionType string

const (
	ActionTypeAddTag            ActionType = "add_tag"
	ActionTypeRemoveTag         ActionType = "remove_tag"
	ActionTypeMoveToAlbum       ActionType = "move_to_album"
	ActionTypeSetVisibility     ActionType = "set_visibility"
	ActionTypeSendNotification  ActionType = "send_notification"
	ActionTypeWebhook           ActionType = "webhook"
	ActionTypeRunPlugin         ActionType = "run_plugin"
	ActionTypeSetMetadata       ActionType = "set_metadata"
	ActionTypeGenerateThumbnail ActionType = "generate_thumbnail"
)

// WorkflowStatus represents the status of a workflow
type WorkflowStatus string

const (
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusDisabled WorkflowStatus = "disabled"
	WorkflowStatusError    WorkflowStatus = "error"
)

// ExecutionStatus represents the status of a workflow execution
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// Trigger represents a workflow trigger configuration
type Trigger struct {
	Type           TriggerType            `json:"type"`
	CronExpression string                 `json:"cronExpression,omitempty"`
	Conditions     map[string]interface{} `json:"conditions,omitempty"`
}

// Action represents a workflow action configuration
type Action struct {
	Type   ActionType             `json:"type"`
	Params map[string]interface{} `json:"params"`
	Order  int                    `json:"order"`
}

// ActionResult represents the result of a workflow action execution
type ActionResult struct {
	Type         ActionType `json:"type"`
	Success      bool       `json:"success"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	DurationMs   int64      `json:"durationMs"`
}

// WorkflowInfo contains information about a workflow
type WorkflowInfo struct {
	ID              string
	Name            string
	Description     string
	Status          WorkflowStatus
	Enabled         bool
	Trigger         Trigger
	Actions         []Action
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CreatedBy       string
	ErrorMessage    string
	ExecutionCount  int
	LastExecutionAt *time.Time
}

// ExecutionInfo contains information about a workflow execution
type ExecutionInfo struct {
	ID            string
	WorkflowID    string
	Status        ExecutionStatus
	StartedAt     time.Time
	CompletedAt   *time.Time
	ErrorMessage  string
	TriggerData   map[string]interface{}
	ActionResults []ActionResult
}

// Service handles workflow management operations
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// In-memory workflow registry (in production, would use database)
	mu         sync.RWMutex
	workflows  map[string]*WorkflowInfo
	executions map[string]*ExecutionInfo

	// Metrics
	workflowCounter   metric.Int64UpDownCounter
	executionCounter  metric.Int64Counter
	operationDuration metric.Float64Histogram
}

// NewService creates a new workflow management service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	meter := telemetry.GetMeter()

	workflowCounter, err := meter.Int64UpDownCounter(
		"workflows_total",
		metric.WithDescription("Total number of workflows"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow counter: %w", err)
	}

	executionCounter, err := meter.Int64Counter(
		"workflow_executions_total",
		metric.WithDescription("Total number of workflow executions"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution counter: %w", err)
	}

	operationDuration, err := meter.Float64Histogram(
		"workflow_operation_duration_seconds",
		metric.WithDescription("Time spent on workflow operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation duration histogram: %w", err)
	}

	s := &Service{
		db:                queries,
		config:            cfg,
		workflows:         make(map[string]*WorkflowInfo),
		executions:        make(map[string]*ExecutionInfo),
		workflowCounter:   workflowCounter,
		executionCounter:  executionCounter,
		operationDuration: operationDuration,
	}

	// Initialize with sample workflows
	s.initializeSampleWorkflows()

	return s, nil
}

// initializeSampleWorkflows creates sample workflows for demonstration
func (s *Service) initializeSampleWorkflows() {
	now := time.Now()

	// Auto-tag workflow
	s.workflows["auto-tag-screenshots"] = &WorkflowInfo{
		ID:          "auto-tag-screenshots",
		Name:        "Auto-tag Screenshots",
		Description: "Automatically tag screenshots based on filename pattern",
		Status:      WorkflowStatusActive,
		Enabled:     true,
		Trigger: Trigger{
			Type: TriggerTypeAssetUploaded,
			Conditions: map[string]interface{}{
				"filenamePattern": "(?i)screenshot.*",
			},
		},
		Actions: []Action{
			{
				Type: ActionTypeAddTag,
				Params: map[string]interface{}{
					"tagName": "Screenshots",
				},
				Order: 1,
			},
		},
		CreatedAt:      now,
		UpdatedAt:      now,
		ExecutionCount: 0,
	}

	// Archive old photos workflow
	s.workflows["archive-old-photos"] = &WorkflowInfo{
		ID:          "archive-old-photos",
		Name:        "Archive Old Photos",
		Description: "Archive photos older than 5 years",
		Status:      WorkflowStatusDisabled,
		Enabled:     false,
		Trigger: Trigger{
			Type:           TriggerTypeScheduled,
			CronExpression: "0 0 * * 0", // Every Sunday at midnight
			Conditions: map[string]interface{}{
				"olderThanDays": 1825, // 5 years
			},
		},
		Actions: []Action{
			{
				Type: ActionTypeSetVisibility,
				Params: map[string]interface{}{
					"visibility": "archive",
				},
				Order: 1,
			},
		},
		CreatedAt:      now,
		UpdatedAt:      now,
		ExecutionCount: 0,
	}

	// Webhook on duplicate workflow
	s.workflows["duplicate-webhook"] = &WorkflowInfo{
		ID:          "duplicate-webhook",
		Name:        "Duplicate Detection Webhook",
		Description: "Send webhook notification when duplicate is found",
		Status:      WorkflowStatusActive,
		Enabled:     true,
		Trigger: Trigger{
			Type: TriggerTypeDuplicateFound,
		},
		Actions: []Action{
			{
				Type: ActionTypeWebhook,
				Params: map[string]interface{}{
					"url":    "https://example.com/webhook/duplicates",
					"method": "POST",
				},
				Order: 1,
			},
		},
		CreatedAt:      now,
		UpdatedAt:      now,
		ExecutionCount: 0,
	}
}

// ListWorkflows returns all workflows
func (s *Service) ListWorkflows(ctx context.Context) ([]*WorkflowInfo, error) {
	ctx, span := tracer.Start(ctx, "workflow.list_workflows")
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "list_workflows")))
	}()

	s.mu.RLock()
	defer s.mu.RUnlock()

	workflows := make([]*WorkflowInfo, 0, len(s.workflows))
	for _, workflow := range s.workflows {
		workflows = append(workflows, workflow)
	}

	return workflows, nil
}

// GetWorkflow returns a specific workflow by ID
func (s *Service) GetWorkflow(ctx context.Context, workflowID string) (*WorkflowInfo, error) {
	ctx, span := tracer.Start(ctx, "workflow.get_workflow",
		trace.WithAttributes(attribute.String("workflow_id", workflowID)))
	defer span.End()

	s.mu.RLock()
	defer s.mu.RUnlock()

	workflow, exists := s.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	return workflow, nil
}

// CreateWorkflow creates a new workflow
func (s *Service) CreateWorkflow(ctx context.Context, name, description string, trigger Trigger, actions []Action, enabled bool, createdBy string) (*WorkflowInfo, error) {
	ctx, span := tracer.Start(ctx, "workflow.create_workflow",
		trace.WithAttributes(attribute.String("name", name)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "create_workflow")))
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	workflowID := uuid.New().String()
	now := time.Now()

	workflow := &WorkflowInfo{
		ID:             workflowID,
		Name:           name,
		Description:    description,
		Status:         WorkflowStatusActive,
		Enabled:        enabled,
		Trigger:        trigger,
		Actions:        actions,
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      createdBy,
		ExecutionCount: 0,
	}

	if !enabled {
		workflow.Status = WorkflowStatusDisabled
	}

	s.workflows[workflowID] = workflow
	s.workflowCounter.Add(ctx, 1)

	return workflow, nil
}

// UpdateWorkflow updates an existing workflow
func (s *Service) UpdateWorkflow(ctx context.Context, workflowID string, name, description *string, trigger *Trigger, actions []Action) (*WorkflowInfo, error) {
	ctx, span := tracer.Start(ctx, "workflow.update_workflow",
		trace.WithAttributes(attribute.String("workflow_id", workflowID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "update_workflow")))
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	workflow, exists := s.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	if name != nil {
		workflow.Name = *name
	}
	if description != nil {
		workflow.Description = *description
	}
	if trigger != nil {
		workflow.Trigger = *trigger
	}
	if len(actions) > 0 {
		workflow.Actions = actions
	}
	workflow.UpdatedAt = time.Now()

	return workflow, nil
}

// DeleteWorkflow deletes a workflow
func (s *Service) DeleteWorkflow(ctx context.Context, workflowID string) error {
	ctx, span := tracer.Start(ctx, "workflow.delete_workflow",
		trace.WithAttributes(attribute.String("workflow_id", workflowID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "delete_workflow")))
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.workflows[workflowID]; !exists {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	delete(s.workflows, workflowID)
	s.workflowCounter.Add(ctx, -1)

	return nil
}

// TriggerWorkflow manually triggers a workflow execution
func (s *Service) TriggerWorkflow(ctx context.Context, workflowID string, triggerData map[string]interface{}) (*ExecutionInfo, error) {
	ctx, span := tracer.Start(ctx, "workflow.trigger_workflow",
		trace.WithAttributes(attribute.String("workflow_id", workflowID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "trigger_workflow")))
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	workflow, exists := s.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	if !workflow.Enabled {
		return nil, fmt.Errorf("workflow is disabled: %s", workflowID)
	}

	executionID := uuid.New().String()
	now := time.Now()

	// Simulate workflow execution
	actionResults := make([]ActionResult, len(workflow.Actions))
	for i, action := range workflow.Actions {
		actionResults[i] = ActionResult{
			Type:       action.Type,
			Success:    true,
			DurationMs: 50, // Simulated duration
		}
	}

	execution := &ExecutionInfo{
		ID:            executionID,
		WorkflowID:    workflowID,
		Status:        ExecutionStatusCompleted,
		StartedAt:     now,
		CompletedAt:   &now,
		TriggerData:   triggerData,
		ActionResults: actionResults,
	}

	s.executions[executionID] = execution
	workflow.ExecutionCount++
	workflow.LastExecutionAt = &now

	s.executionCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("workflow_id", workflowID),
			attribute.String("status", string(ExecutionStatusCompleted)),
		))

	return execution, nil
}

// GetWorkflowExecutions returns execution history for a workflow
func (s *Service) GetWorkflowExecutions(ctx context.Context, workflowID string, limit, offset int, statusFilter *ExecutionStatus) ([]*ExecutionInfo, int, error) {
	ctx, span := tracer.Start(ctx, "workflow.get_workflow_executions",
		trace.WithAttributes(attribute.String("workflow_id", workflowID)))
	defer span.End()

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Verify workflow exists
	if _, exists := s.workflows[workflowID]; !exists {
		return nil, 0, fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Filter executions
	var filtered []*ExecutionInfo
	for _, exec := range s.executions {
		if exec.WorkflowID != workflowID {
			continue
		}
		if statusFilter != nil && exec.Status != *statusFilter {
			continue
		}
		filtered = append(filtered, exec)
	}

	total := len(filtered)

	// Apply pagination
	if offset >= len(filtered) {
		return []*ExecutionInfo{}, total, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end], total, nil
}

// SetWorkflowEnabled enables or disables a workflow
func (s *Service) SetWorkflowEnabled(ctx context.Context, workflowID string, enabled bool) (*WorkflowInfo, error) {
	ctx, span := tracer.Start(ctx, "workflow.set_workflow_enabled",
		trace.WithAttributes(
			attribute.String("workflow_id", workflowID),
			attribute.Bool("enabled", enabled),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "set_workflow_enabled")))
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	workflow, exists := s.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	workflow.Enabled = enabled
	workflow.UpdatedAt = time.Now()

	if enabled {
		workflow.Status = WorkflowStatusActive
	} else {
		workflow.Status = WorkflowStatusDisabled
	}

	return workflow, nil
}
