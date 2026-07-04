package server

import (
	"context"
	"maps"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/workflow"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	workflowTriggerAssetCreate             = "AssetCreate"
	workflowTriggerAssetMetadataExtraction = "AssetMetadataExtraction"
	workflowTypeAssetV1                    = "AssetV1"
)

var workflowTriggerResponses = []*immichv1.WorkflowTriggerResponseDto{
	{Trigger: workflowTriggerAssetCreate, Types: []string{workflowTypeAssetV1}},
	{Trigger: workflowTriggerAssetMetadataExtraction, Types: []string{workflowTypeAssetV1}},
}

// ListWorkflows returns all workflows
func (s *Server) ListWorkflows(ctx context.Context, _ *emptypb.Empty) (*immichv1.ListWorkflowsResponse, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	workflows, err := s.workflowService.ListWorkflows(ctx)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to list workflows", err)
	}

	protoWorkflows := make([]*immichv1.WorkflowInfo, 0, len(workflows))
	for _, w := range workflows {
		protoWorkflows = append(protoWorkflows, workflowToProto(w))
	}

	return &immichv1.ListWorkflowsResponse{
		Workflows: protoWorkflows,
	}, nil
}

func (s *Server) GetWorkflowTriggers(ctx context.Context, _ *emptypb.Empty) (*immichv1.GetWorkflowTriggersResponse, error) {
	if _, err := s.getUserFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	triggers := make([]*immichv1.WorkflowTriggerResponseDto, 0, len(workflowTriggerResponses))
	for _, trigger := range workflowTriggerResponses {
		triggers = append(triggers, &immichv1.WorkflowTriggerResponseDto{
			Trigger: trigger.Trigger,
			Types:   append([]string(nil), trigger.Types...),
		})
	}

	return &immichv1.GetWorkflowTriggersResponse{Triggers: triggers}, nil
}

// GetWorkflow returns a specific workflow by ID
func (s *Server) GetWorkflow(ctx context.Context, req *immichv1.GetWorkflowRequest) (*immichv1.WorkflowInfo, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	w, err := s.workflowService.GetWorkflow(ctx, req.WorkflowId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "workflow not found: %v", err)
	}

	return workflowToProto(w), nil
}

func (s *Server) GetWorkflowForShare(ctx context.Context, req *immichv1.GetWorkflowForShareRequest) (*immichv1.WorkflowShareResponseDto, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	w, err := s.workflowService.GetWorkflow(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "workflow not found: %v", err)
	}

	resp, err := workflowShareToProto(w)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to convert workflow share response", err)
	}
	return resp, nil
}

// CreateWorkflow creates a new workflow
func (s *Server) CreateWorkflow(ctx context.Context, req *immichv1.CreateWorkflowRequest) (*immichv1.WorkflowInfo, error) {
	claims, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	trigger := protoTriggerToInternal(req.Trigger)
	actions := make([]workflow.Action, len(req.Actions))
	for i, a := range req.Actions {
		actions[i] = protoActionToInternal(a)
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	description := ""
	if req.Description != nil {
		description = *req.Description
	}

	w, err := s.workflowService.CreateWorkflow(ctx, req.Name, description, trigger, actions, enabled, claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to create workflow", err)
	}

	return workflowToProto(w), nil
}

// UpdateWorkflow updates an existing workflow
func (s *Server) UpdateWorkflow(ctx context.Context, req *immichv1.UpdateWorkflowRequest) (*immichv1.WorkflowInfo, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	var trigger *workflow.Trigger
	if req.Trigger != nil {
		t := protoTriggerToInternal(req.Trigger)
		trigger = &t
	}

	var actions []workflow.Action
	if len(req.Actions) > 0 {
		actions = make([]workflow.Action, len(req.Actions))
		for i, a := range req.Actions {
			actions[i] = protoActionToInternal(a)
		}
	}

	w, err := s.workflowService.UpdateWorkflow(ctx, req.WorkflowId, req.Name, req.Description, trigger, actions)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to update workflow", err)
	}

	return workflowToProto(w), nil
}

// DeleteWorkflow deletes a workflow
func (s *Server) DeleteWorkflow(ctx context.Context, req *immichv1.DeleteWorkflowRequest) (*emptypb.Empty, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	err := s.workflowService.DeleteWorkflow(ctx, req.WorkflowId)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to delete workflow", err)
	}

	return &emptypb.Empty{}, nil
}

// TriggerWorkflow manually triggers a workflow execution
func (s *Server) TriggerWorkflow(ctx context.Context, req *immichv1.TriggerWorkflowRequest) (*immichv1.WorkflowExecutionInfo, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	var triggerData map[string]interface{}
	if req.TriggerData != nil {
		triggerData = req.TriggerData.AsMap()
	}

	exec, err := s.workflowService.TriggerWorkflow(ctx, req.WorkflowId, triggerData)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to trigger workflow", err)
	}

	return executionToProto(exec), nil
}

// GetWorkflowExecutions returns execution history for a workflow
func (s *Server) GetWorkflowExecutions(ctx context.Context, req *immichv1.GetWorkflowExecutionsRequest) (*immichv1.ListWorkflowExecutionsResponse, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	limit := 50
	if req.Limit != nil {
		limit = int(*req.Limit)
	}
	offset := 0
	if req.Offset != nil {
		offset = int(*req.Offset)
	}

	var statusFilter *workflow.ExecutionStatus
	if req.Status != nil && *req.Status != immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_UNSPECIFIED {
		s := protoExecutionStatusToInternal(*req.Status)
		statusFilter = &s
	}

	executions, total, err := s.workflowService.GetWorkflowExecutions(ctx, req.WorkflowId, limit, offset, statusFilter)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get workflow executions", err)
	}

	protoExecutions := make([]*immichv1.WorkflowExecutionInfo, 0, len(executions))
	for _, e := range executions {
		protoExecutions = append(protoExecutions, executionToProto(e))
	}

	return &immichv1.ListWorkflowExecutionsResponse{
		Executions: protoExecutions,
		Total:      int32(total),
	}, nil
}

// SetWorkflowEnabled enables or disables a workflow
func (s *Server) SetWorkflowEnabled(ctx context.Context, req *immichv1.SetWorkflowEnabledRequest) (*immichv1.WorkflowInfo, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	w, err := s.workflowService.SetWorkflowEnabled(ctx, req.WorkflowId, req.Enabled)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to update workflow", err)
	}

	return workflowToProto(w), nil
}

// Helper functions for conversion

func workflowToProto(w *workflow.WorkflowInfo) *immichv1.WorkflowInfo {
	proto := &immichv1.WorkflowInfo{
		Id:             w.ID,
		Name:           w.Name,
		Description:    w.Description,
		Status:         workflowStatusToProto(w.Status),
		Enabled:        w.Enabled,
		Trigger:        triggerToProto(w.Trigger),
		CreatedAt:      timestamppb.New(w.CreatedAt),
		UpdatedAt:      timestamppb.New(w.UpdatedAt),
		ExecutionCount: int32(w.ExecutionCount),
	}

	for _, a := range w.Actions {
		proto.Actions = append(proto.Actions, actionToProto(a))
	}

	if w.CreatedBy != "" {
		proto.CreatedBy = &w.CreatedBy
	}
	if w.ErrorMessage != "" {
		proto.ErrorMessage = &w.ErrorMessage
	}
	if w.LastExecutionAt != nil {
		proto.LastExecutionAt = timestamppb.New(*w.LastExecutionAt)
	}

	return proto
}

func workflowShareToProto(w *workflow.WorkflowInfo) (*immichv1.WorkflowShareResponseDto, error) {
	steps := make([]*immichv1.WorkflowShareStepDto, 0, len(w.Actions))
	for _, action := range w.Actions {
		step, err := actionToShareStepProto(action)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}

	return &immichv1.WorkflowShareResponseDto{
		Description: w.Description,
		Name:        w.Name,
		Steps:       steps,
		Trigger:     workflowTriggerToV3(w.Trigger.Type),
	}, nil
}

func triggerToProto(t workflow.Trigger) *immichv1.WorkflowTrigger {
	proto := &immichv1.WorkflowTrigger{
		Type: triggerTypeToProto(t.Type),
	}

	if t.CronExpression != "" {
		proto.CronExpression = &t.CronExpression
	}
	if t.Conditions != nil {
		proto.Conditions, _ = structpb.NewStruct(t.Conditions)
	}

	return proto
}

func actionToShareStepProto(a workflow.Action) (*immichv1.WorkflowShareStepDto, error) {
	method, config := actionShareMethodAndConfig(a)
	configStruct, err := structpb.NewStruct(config)
	if err != nil {
		return nil, err
	}
	return &immichv1.WorkflowShareStepDto{
		Method: method,
		Config: configStruct,
	}, nil
}

func actionShareMethodAndConfig(a workflow.Action) (string, map[string]interface{}) {
	config := maps.Clone(a.Params)
	if config == nil {
		config = map[string]interface{}{}
	}
	if a.Type == workflow.ActionTypeRunPlugin {
		if method, ok := config["method"].(string); ok && method != "" {
			delete(config, "method")
			return method, config
		}
	}
	return string(a.Type), config
}

func actionToProto(a workflow.Action) *immichv1.WorkflowAction {
	proto := &immichv1.WorkflowAction{
		Type: actionTypeToProto(a.Type),
	}

	if a.Params != nil {
		proto.Params, _ = structpb.NewStruct(a.Params)
	}
	if a.Order > 0 {
		order := int32(a.Order)
		proto.Order = &order
	}

	return proto
}

func executionToProto(e *workflow.ExecutionInfo) *immichv1.WorkflowExecutionInfo {
	proto := &immichv1.WorkflowExecutionInfo{
		Id:         e.ID,
		WorkflowId: e.WorkflowID,
		Status:     executionStatusToProto(e.Status),
		StartedAt:  timestamppb.New(e.StartedAt),
	}

	if e.CompletedAt != nil {
		proto.CompletedAt = timestamppb.New(*e.CompletedAt)
	}
	if e.ErrorMessage != "" {
		proto.ErrorMessage = &e.ErrorMessage
	}
	if e.TriggerData != nil {
		proto.TriggerData, _ = structpb.NewStruct(e.TriggerData)
	}

	for _, r := range e.ActionResults {
		proto.ActionResults = append(proto.ActionResults, &immichv1.WorkflowActionResult{
			Type:       actionTypeToProto(r.Type),
			Success:    r.Success,
			DurationMs: r.DurationMs,
		})
	}

	return proto
}

func protoTriggerToInternal(t *immichv1.WorkflowTrigger) workflow.Trigger {
	trigger := workflow.Trigger{
		Type: protoTriggerTypeToInternal(t.Type),
	}

	if t.CronExpression != nil {
		trigger.CronExpression = *t.CronExpression
	}
	if t.Conditions != nil {
		trigger.Conditions = t.Conditions.AsMap()
	}

	return trigger
}

func protoActionToInternal(a *immichv1.WorkflowAction) workflow.Action {
	action := workflow.Action{
		Type: protoActionTypeToInternal(a.Type),
	}

	if a.Params != nil {
		action.Params = a.Params.AsMap()
	}
	if a.Order != nil {
		action.Order = int(*a.Order)
	}

	return action
}

func workflowStatusToProto(s workflow.WorkflowStatus) immichv1.WorkflowStatus {
	return lookupWorkflowMapping(workflowStatusProtoValues, s, immichv1.WorkflowStatus_WORKFLOW_STATUS_UNSPECIFIED)
}

var workflowStatusProtoValues = map[workflow.WorkflowStatus]immichv1.WorkflowStatus{
	workflow.WorkflowStatusActive:   immichv1.WorkflowStatus_WORKFLOW_STATUS_ACTIVE,
	workflow.WorkflowStatusDisabled: immichv1.WorkflowStatus_WORKFLOW_STATUS_DISABLED,
	workflow.WorkflowStatusError:    immichv1.WorkflowStatus_WORKFLOW_STATUS_ERROR,
}

func triggerTypeToProto(t workflow.TriggerType) immichv1.WorkflowTriggerType {
	return lookupWorkflowMapping(triggerTypeProtoValues, t, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_UNSPECIFIED)
}

func workflowTriggerToV3(workflow.TriggerType) string {
	// The legacy workflow model has no metadata-extraction trigger; AssetCreate
	// is the closest v3 export trigger for existing workflows.
	return workflowTriggerAssetCreate
}

var triggerTypeProtoValues = map[workflow.TriggerType]immichv1.WorkflowTriggerType{
	workflow.TriggerTypeAssetUploaded:  immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ASSET_UPLOADED,
	workflow.TriggerTypeAssetDeleted:   immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ASSET_DELETED,
	workflow.TriggerTypeAlbumCreated:   immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ALBUM_CREATED,
	workflow.TriggerTypeAlbumUpdated:   immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ALBUM_UPDATED,
	workflow.TriggerTypeUserCreated:    immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_USER_CREATED,
	workflow.TriggerTypeScheduled:      immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_SCHEDULED,
	workflow.TriggerTypeManual:         immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_MANUAL,
	workflow.TriggerTypeFaceDetected:   immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_FACE_DETECTED,
	workflow.TriggerTypeDuplicateFound: immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_DUPLICATE_FOUND,
}

func protoTriggerTypeToInternal(t immichv1.WorkflowTriggerType) workflow.TriggerType {
	return lookupWorkflowMapping(protoTriggerTypeValues, t, workflow.TriggerTypeManual)
}

var protoTriggerTypeValues = map[immichv1.WorkflowTriggerType]workflow.TriggerType{
	immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ASSET_UPLOADED:  workflow.TriggerTypeAssetUploaded,
	immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ASSET_DELETED:   workflow.TriggerTypeAssetDeleted,
	immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ALBUM_CREATED:   workflow.TriggerTypeAlbumCreated,
	immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ALBUM_UPDATED:   workflow.TriggerTypeAlbumUpdated,
	immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_USER_CREATED:    workflow.TriggerTypeUserCreated,
	immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_SCHEDULED:       workflow.TriggerTypeScheduled,
	immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_MANUAL:          workflow.TriggerTypeManual,
	immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_FACE_DETECTED:   workflow.TriggerTypeFaceDetected,
	immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_DUPLICATE_FOUND: workflow.TriggerTypeDuplicateFound,
}

func actionTypeToProto(t workflow.ActionType) immichv1.WorkflowActionType {
	return lookupWorkflowMapping(actionTypeProtoValues, t, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_UNSPECIFIED)
}

var actionTypeProtoValues = map[workflow.ActionType]immichv1.WorkflowActionType{
	workflow.ActionTypeAddTag:            immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_ADD_TAG,
	workflow.ActionTypeRemoveTag:         immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_REMOVE_TAG,
	workflow.ActionTypeMoveToAlbum:       immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_MOVE_TO_ALBUM,
	workflow.ActionTypeSetVisibility:     immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_VISIBILITY,
	workflow.ActionTypeSendNotification:  immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SEND_NOTIFICATION,
	workflow.ActionTypeWebhook:           immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_WEBHOOK,
	workflow.ActionTypeRunPlugin:         immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_RUN_PLUGIN,
	workflow.ActionTypeSetMetadata:       immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_METADATA,
	workflow.ActionTypeGenerateThumbnail: immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_GENERATE_THUMBNAIL,
}

func protoActionTypeToInternal(t immichv1.WorkflowActionType) workflow.ActionType {
	return lookupWorkflowMapping(protoActionTypeValues, t, workflow.ActionTypeWebhook)
}

var protoActionTypeValues = map[immichv1.WorkflowActionType]workflow.ActionType{
	immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_ADD_TAG:            workflow.ActionTypeAddTag,
	immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_REMOVE_TAG:         workflow.ActionTypeRemoveTag,
	immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_MOVE_TO_ALBUM:      workflow.ActionTypeMoveToAlbum,
	immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_VISIBILITY:     workflow.ActionTypeSetVisibility,
	immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SEND_NOTIFICATION:  workflow.ActionTypeSendNotification,
	immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_WEBHOOK:            workflow.ActionTypeWebhook,
	immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_RUN_PLUGIN:         workflow.ActionTypeRunPlugin,
	immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_METADATA:       workflow.ActionTypeSetMetadata,
	immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_GENERATE_THUMBNAIL: workflow.ActionTypeGenerateThumbnail,
}

func executionStatusToProto(s workflow.ExecutionStatus) immichv1.WorkflowExecutionStatus {
	return lookupWorkflowMapping(executionStatusProtoValues, s, immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_UNSPECIFIED)
}

var executionStatusProtoValues = map[workflow.ExecutionStatus]immichv1.WorkflowExecutionStatus{
	workflow.ExecutionStatusPending:   immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_PENDING,
	workflow.ExecutionStatusRunning:   immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_RUNNING,
	workflow.ExecutionStatusCompleted: immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_COMPLETED,
	workflow.ExecutionStatusFailed:    immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_FAILED,
	workflow.ExecutionStatusCancelled: immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_CANCELLED,
}

func protoExecutionStatusToInternal(s immichv1.WorkflowExecutionStatus) workflow.ExecutionStatus {
	return lookupWorkflowMapping(protoExecutionStatusValues, s, workflow.ExecutionStatusPending)
}

var protoExecutionStatusValues = map[immichv1.WorkflowExecutionStatus]workflow.ExecutionStatus{
	immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_PENDING:   workflow.ExecutionStatusPending,
	immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_RUNNING:   workflow.ExecutionStatusRunning,
	immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_COMPLETED: workflow.ExecutionStatusCompleted,
	immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_FAILED:    workflow.ExecutionStatusFailed,
	immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_CANCELLED: workflow.ExecutionStatusCancelled,
}

func lookupWorkflowMapping[K comparable, V any](values map[K]V, key K, fallback V) V {
	if value, ok := values[key]; ok {
		return value
	}
	return fallback
}
