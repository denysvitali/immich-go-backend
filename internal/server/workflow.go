package server

import (
	"context"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/workflow"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ListWorkflows returns all workflows
func (s *Server) ListWorkflows(ctx context.Context, _ *emptypb.Empty) (*immichv1.ListWorkflowsResponse, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	workflows, err := s.workflowService.ListWorkflows(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list workflows: %v", err)
	}

	protoWorkflows := make([]*immichv1.WorkflowInfo, 0, len(workflows))
	for _, w := range workflows {
		protoWorkflows = append(protoWorkflows, workflowToProto(w))
	}

	return &immichv1.ListWorkflowsResponse{
		Workflows: protoWorkflows,
	}, nil
}

// GetWorkflow returns a specific workflow by ID
func (s *Server) GetWorkflow(ctx context.Context, req *immichv1.GetWorkflowRequest) (*immichv1.WorkflowInfo, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	w, err := s.workflowService.GetWorkflow(ctx, req.WorkflowId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "workflow not found: %v", err)
	}

	return workflowToProto(w), nil
}

// CreateWorkflow creates a new workflow
func (s *Server) CreateWorkflow(ctx context.Context, req *immichv1.CreateWorkflowRequest) (*immichv1.WorkflowInfo, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
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
		return nil, status.Errorf(codes.Internal, "failed to create workflow: %v", err)
	}

	return workflowToProto(w), nil
}

// UpdateWorkflow updates an existing workflow
func (s *Server) UpdateWorkflow(ctx context.Context, req *immichv1.UpdateWorkflowRequest) (*immichv1.WorkflowInfo, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
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
		return nil, status.Errorf(codes.Internal, "failed to update workflow: %v", err)
	}

	return workflowToProto(w), nil
}

// DeleteWorkflow deletes a workflow
func (s *Server) DeleteWorkflow(ctx context.Context, req *immichv1.DeleteWorkflowRequest) (*emptypb.Empty, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	err = s.workflowService.DeleteWorkflow(ctx, req.WorkflowId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete workflow: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// TriggerWorkflow manually triggers a workflow execution
func (s *Server) TriggerWorkflow(ctx context.Context, req *immichv1.TriggerWorkflowRequest) (*immichv1.WorkflowExecutionInfo, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	var triggerData map[string]interface{}
	if req.TriggerData != nil {
		triggerData = req.TriggerData.AsMap()
	}

	exec, err := s.workflowService.TriggerWorkflow(ctx, req.WorkflowId, triggerData)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to trigger workflow: %v", err)
	}

	return executionToProto(exec), nil
}

// GetWorkflowExecutions returns execution history for a workflow
func (s *Server) GetWorkflowExecutions(ctx context.Context, req *immichv1.GetWorkflowExecutionsRequest) (*immichv1.ListWorkflowExecutionsResponse, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
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
		return nil, status.Errorf(codes.Internal, "failed to get workflow executions: %v", err)
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
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	w, err := s.workflowService.SetWorkflowEnabled(ctx, req.WorkflowId, req.Enabled)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update workflow: %v", err)
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
	switch s {
	case workflow.WorkflowStatusActive:
		return immichv1.WorkflowStatus_WORKFLOW_STATUS_ACTIVE
	case workflow.WorkflowStatusDisabled:
		return immichv1.WorkflowStatus_WORKFLOW_STATUS_DISABLED
	case workflow.WorkflowStatusError:
		return immichv1.WorkflowStatus_WORKFLOW_STATUS_ERROR
	default:
		return immichv1.WorkflowStatus_WORKFLOW_STATUS_UNSPECIFIED
	}
}

func triggerTypeToProto(t workflow.TriggerType) immichv1.WorkflowTriggerType {
	switch t {
	case workflow.TriggerTypeAssetUploaded:
		return immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ASSET_UPLOADED
	case workflow.TriggerTypeAssetDeleted:
		return immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ASSET_DELETED
	case workflow.TriggerTypeAlbumCreated:
		return immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ALBUM_CREATED
	case workflow.TriggerTypeAlbumUpdated:
		return immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ALBUM_UPDATED
	case workflow.TriggerTypeUserCreated:
		return immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_USER_CREATED
	case workflow.TriggerTypeScheduled:
		return immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_SCHEDULED
	case workflow.TriggerTypeManual:
		return immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_MANUAL
	case workflow.TriggerTypeFaceDetected:
		return immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_FACE_DETECTED
	case workflow.TriggerTypeDuplicateFound:
		return immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_DUPLICATE_FOUND
	default:
		return immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_UNSPECIFIED
	}
}

func protoTriggerTypeToInternal(t immichv1.WorkflowTriggerType) workflow.TriggerType {
	switch t {
	case immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ASSET_UPLOADED:
		return workflow.TriggerTypeAssetUploaded
	case immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ASSET_DELETED:
		return workflow.TriggerTypeAssetDeleted
	case immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ALBUM_CREATED:
		return workflow.TriggerTypeAlbumCreated
	case immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ALBUM_UPDATED:
		return workflow.TriggerTypeAlbumUpdated
	case immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_USER_CREATED:
		return workflow.TriggerTypeUserCreated
	case immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_SCHEDULED:
		return workflow.TriggerTypeScheduled
	case immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_MANUAL:
		return workflow.TriggerTypeManual
	case immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_FACE_DETECTED:
		return workflow.TriggerTypeFaceDetected
	case immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_DUPLICATE_FOUND:
		return workflow.TriggerTypeDuplicateFound
	default:
		return workflow.TriggerTypeManual
	}
}

func actionTypeToProto(t workflow.ActionType) immichv1.WorkflowActionType {
	switch t {
	case workflow.ActionTypeAddTag:
		return immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_ADD_TAG
	case workflow.ActionTypeRemoveTag:
		return immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_REMOVE_TAG
	case workflow.ActionTypeMoveToAlbum:
		return immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_MOVE_TO_ALBUM
	case workflow.ActionTypeSetVisibility:
		return immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_VISIBILITY
	case workflow.ActionTypeSendNotification:
		return immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SEND_NOTIFICATION
	case workflow.ActionTypeWebhook:
		return immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_WEBHOOK
	case workflow.ActionTypeRunPlugin:
		return immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_RUN_PLUGIN
	case workflow.ActionTypeSetMetadata:
		return immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_METADATA
	case workflow.ActionTypeGenerateThumbnail:
		return immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_GENERATE_THUMBNAIL
	default:
		return immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_UNSPECIFIED
	}
}

func protoActionTypeToInternal(t immichv1.WorkflowActionType) workflow.ActionType {
	switch t {
	case immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_ADD_TAG:
		return workflow.ActionTypeAddTag
	case immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_REMOVE_TAG:
		return workflow.ActionTypeRemoveTag
	case immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_MOVE_TO_ALBUM:
		return workflow.ActionTypeMoveToAlbum
	case immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_VISIBILITY:
		return workflow.ActionTypeSetVisibility
	case immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SEND_NOTIFICATION:
		return workflow.ActionTypeSendNotification
	case immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_WEBHOOK:
		return workflow.ActionTypeWebhook
	case immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_RUN_PLUGIN:
		return workflow.ActionTypeRunPlugin
	case immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_METADATA:
		return workflow.ActionTypeSetMetadata
	case immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_GENERATE_THUMBNAIL:
		return workflow.ActionTypeGenerateThumbnail
	default:
		return workflow.ActionTypeWebhook
	}
}

func executionStatusToProto(s workflow.ExecutionStatus) immichv1.WorkflowExecutionStatus {
	switch s {
	case workflow.ExecutionStatusPending:
		return immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_PENDING
	case workflow.ExecutionStatusRunning:
		return immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_RUNNING
	case workflow.ExecutionStatusCompleted:
		return immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_COMPLETED
	case workflow.ExecutionStatusFailed:
		return immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_FAILED
	case workflow.ExecutionStatusCancelled:
		return immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_CANCELLED
	default:
		return immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_UNSPECIFIED
	}
}

func protoExecutionStatusToInternal(s immichv1.WorkflowExecutionStatus) workflow.ExecutionStatus {
	switch s {
	case immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_PENDING:
		return workflow.ExecutionStatusPending
	case immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_RUNNING:
		return workflow.ExecutionStatusRunning
	case immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return workflow.ExecutionStatusCompleted
	case immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_FAILED:
		return workflow.ExecutionStatusFailed
	case immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_CANCELLED:
		return workflow.ExecutionStatusCancelled
	default:
		return workflow.ExecutionStatusPending
	}
}
