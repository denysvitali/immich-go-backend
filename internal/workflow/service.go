package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

// Service handles workflow management operations backed by PostgreSQL.
type Service struct {
	db     *sqlc.Queries
	config *config.Config

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

	return &Service{
		db:                queries,
		config:            cfg,
		workflowCounter:   workflowCounter,
		executionCounter:  executionCounter,
		operationDuration: operationDuration,
	}, nil
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

	rows, err := s.db.ListWorkflows(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}

	out := make([]*WorkflowInfo, 0, len(rows))
	for _, row := range rows {
		info, err := workflowFromDB(row)
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

// GetWorkflow returns a specific workflow by ID
func (s *Service) GetWorkflow(ctx context.Context, workflowID string) (*WorkflowInfo, error) {
	_, span := tracer.Start(ctx, "workflow.get_workflow",
		trace.WithAttributes(attribute.String("workflow_id", workflowID)))
	defer span.End()

	id, err := parseWorkflowUUID(workflowID)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	row, err := s.db.GetWorkflowByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}
	return workflowFromDB(row)
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

	ownerID, err := pgutil.StringToUUID(createdBy)
	if err != nil {
		return nil, fmt.Errorf("invalid owner id: %w", err)
	}

	status := WorkflowStatusActive
	if !enabled {
		status = WorkflowStatusDisabled
	}

	triggerJSON, err := json.Marshal(trigger)
	if err != nil {
		return nil, fmt.Errorf("marshal trigger: %w", err)
	}
	actionsJSON, err := json.Marshal(actions)
	if err != nil {
		return nil, fmt.Errorf("marshal actions: %w", err)
	}

	id := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	row, err := s.db.CreateWorkflow(ctx, sqlc.CreateWorkflowParams{
		ID:          id,
		OwnerId:     ownerID,
		Name:        name,
		Description: description,
		Enabled:     enabled,
		Status:      string(status),
		Trigger:     triggerJSON,
		Actions:     actionsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create workflow: %w", err)
	}

	s.workflowCounter.Add(ctx, 1)
	return workflowFromDB(row)
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

	id, err := parseWorkflowUUID(workflowID)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	params := sqlc.UpdateWorkflowParams{ID: id}
	if name != nil {
		params.Name = pgtype.Text{String: *name, Valid: true}
	}
	if description != nil {
		params.Description = pgtype.Text{String: *description, Valid: true}
	}
	if trigger != nil {
		b, err := json.Marshal(trigger)
		if err != nil {
			return nil, fmt.Errorf("marshal trigger: %w", err)
		}
		params.Trigger = b
	}
	if len(actions) > 0 {
		b, err := json.Marshal(actions)
		if err != nil {
			return nil, fmt.Errorf("marshal actions: %w", err)
		}
		params.Actions = b
	}

	row, err := s.db.UpdateWorkflow(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}
	return workflowFromDB(row)
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

	id, err := parseWorkflowUUID(workflowID)
	if err != nil {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Ensure exists first for a clear not-found error.
	if _, err := s.db.GetWorkflowByID(ctx, id); err != nil {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	if err := s.db.DeleteWorkflow(ctx, id); err != nil {
		return fmt.Errorf("delete workflow: %w", err)
	}
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

	workflow, err := s.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if !workflow.Enabled {
		return nil, fmt.Errorf("workflow is disabled: %s", workflowID)
	}

	now := time.Now()
	actionResults := make([]ActionResult, len(workflow.Actions))
	for i, action := range workflow.Actions {
		// Built-in actions run as successful no-ops for now; plugin host wires later.
		actionResults[i] = ActionResult{
			Type:       action.Type,
			Success:    true,
			DurationMs: 1,
		}
	}

	execID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	wfID, _ := parseWorkflowUUID(workflowID)

	triggerJSON, _ := json.Marshal(triggerData)
	resultsJSON, _ := json.Marshal(actionResults)

	row, err := s.db.CreateWorkflowExecution(ctx, sqlc.CreateWorkflowExecutionParams{
		ID:            execID,
		WorkflowId:    wfID,
		Status:        string(ExecutionStatusCompleted),
		StartedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		CompletedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		ErrorMessage:  pgtype.Text{},
		TriggerData:   triggerJSON,
		ActionResults: resultsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create execution: %w", err)
	}

	if _, err := s.db.IncrementWorkflowExecutionCount(ctx, wfID); err != nil {
		return nil, fmt.Errorf("increment execution count: %w", err)
	}

	s.executionCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("workflow_id", workflowID),
			attribute.String("status", string(ExecutionStatusCompleted)),
		))

	return executionFromDB(row)
}

// GetWorkflowExecutions returns execution history for a workflow
func (s *Service) GetWorkflowExecutions(ctx context.Context, workflowID string, limit, offset int, statusFilter *ExecutionStatus) ([]*ExecutionInfo, int, error) {
	_, span := tracer.Start(ctx, "workflow.get_workflow_executions",
		trace.WithAttributes(attribute.String("workflow_id", workflowID)))
	defer span.End()

	if _, err := s.GetWorkflow(ctx, workflowID); err != nil {
		return nil, 0, err
	}

	wfID, err := parseWorkflowUUID(workflowID)
	if err != nil {
		return nil, 0, fmt.Errorf("workflow not found: %s", workflowID)
	}

	if limit <= 0 {
		limit = 50
	}

	var statusText pgtype.Text
	if statusFilter != nil {
		statusText = pgtype.Text{String: string(*statusFilter), Valid: true}
	}

	rows, err := s.db.ListWorkflowExecutions(ctx, sqlc.ListWorkflowExecutionsParams{
		WorkflowId: wfID,
		Status:     statusText,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list executions: %w", err)
	}

	total, err := s.db.CountWorkflowExecutions(ctx, sqlc.CountWorkflowExecutionsParams{
		WorkflowId: wfID,
		Status:     statusText,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count executions: %w", err)
	}

	out := make([]*ExecutionInfo, 0, len(rows))
	for _, row := range rows {
		info, err := executionFromDB(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, info)
	}
	return out, int(total), nil
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

	id, err := parseWorkflowUUID(workflowID)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	status := WorkflowStatusActive
	if !enabled {
		status = WorkflowStatusDisabled
	}

	row, err := s.db.UpdateWorkflow(ctx, sqlc.UpdateWorkflowParams{
		ID:      id,
		Enabled: pgtype.Bool{Bool: enabled, Valid: true},
		Status:  pgtype.Text{String: string(status), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}
	return workflowFromDB(row)
}

func parseWorkflowUUID(id string) (pgtype.UUID, error) {
	return pgutil.StringToUUID(id)
}

func workflowFromDB(row sqlc.Workflow) (*WorkflowInfo, error) {
	var trigger Trigger
	if len(row.Trigger) > 0 {
		if err := json.Unmarshal(row.Trigger, &trigger); err != nil {
			return nil, fmt.Errorf("parse trigger: %w", err)
		}
	}
	var actions []Action
	if len(row.Actions) > 0 {
		if err := json.Unmarshal(row.Actions, &actions); err != nil {
			return nil, fmt.Errorf("parse actions: %w", err)
		}
	}

	info := &WorkflowInfo{
		ID:             pgutil.UUIDToString(row.ID),
		Name:           row.Name,
		Description:    row.Description,
		Status:         WorkflowStatus(row.Status),
		Enabled:        row.Enabled,
		Trigger:        trigger,
		Actions:        actions,
		CreatedAt:      pgutil.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:      pgutil.TimestamptzToTime(row.UpdatedAt),
		CreatedBy:      pgutil.UUIDToString(row.OwnerId),
		ExecutionCount: int(row.ExecutionCount),
	}
	if row.LastExecutionAt.Valid {
		t := row.LastExecutionAt.Time
		info.LastExecutionAt = &t
	}
	return info, nil
}

func executionFromDB(row sqlc.WorkflowExecution) (*ExecutionInfo, error) {
	info := &ExecutionInfo{
		ID:         pgutil.UUIDToString(row.ID),
		WorkflowID: pgutil.UUIDToString(row.WorkflowId),
		Status:     ExecutionStatus(row.Status),
		StartedAt:  pgutil.TimestamptzToTime(row.StartedAt),
	}
	if row.CompletedAt.Valid {
		t := row.CompletedAt.Time
		info.CompletedAt = &t
	}
	if row.ErrorMessage.Valid {
		info.ErrorMessage = row.ErrorMessage.String
	}
	if len(row.TriggerData) > 0 {
		_ = json.Unmarshal(row.TriggerData, &info.TriggerData)
	}
	if len(row.ActionResults) > 0 {
		_ = json.Unmarshal(row.ActionResults, &info.ActionResults)
	}
	return info, nil
}
