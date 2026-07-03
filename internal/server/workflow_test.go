package server

import (
	"testing"
	"time"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowStatusToProto(t *testing.T) {
	tests := []struct {
		name string
		in   workflow.WorkflowStatus
		want immichv1.WorkflowStatus
	}{
		{"active", workflow.WorkflowStatusActive, immichv1.WorkflowStatus_WORKFLOW_STATUS_ACTIVE},
		{"disabled", workflow.WorkflowStatusDisabled, immichv1.WorkflowStatus_WORKFLOW_STATUS_DISABLED},
		{"error", workflow.WorkflowStatusError, immichv1.WorkflowStatus_WORKFLOW_STATUS_ERROR},
		{"unknown", workflow.WorkflowStatus("unknown"), immichv1.WorkflowStatus_WORKFLOW_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, workflowStatusToProto(tt.in))
		})
	}
}

func TestWorkflowTriggerTypeMappings(t *testing.T) {
	tests := []struct {
		name     string
		internal workflow.TriggerType
		proto    immichv1.WorkflowTriggerType
	}{
		{"asset uploaded", workflow.TriggerTypeAssetUploaded, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ASSET_UPLOADED},
		{"asset deleted", workflow.TriggerTypeAssetDeleted, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ASSET_DELETED},
		{"album created", workflow.TriggerTypeAlbumCreated, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ALBUM_CREATED},
		{"album updated", workflow.TriggerTypeAlbumUpdated, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_ALBUM_UPDATED},
		{"user created", workflow.TriggerTypeUserCreated, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_USER_CREATED},
		{"scheduled", workflow.TriggerTypeScheduled, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_SCHEDULED},
		{"manual", workflow.TriggerTypeManual, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_MANUAL},
		{"face detected", workflow.TriggerTypeFaceDetected, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_FACE_DETECTED},
		{"duplicate found", workflow.TriggerTypeDuplicateFound, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_DUPLICATE_FOUND},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.proto, triggerTypeToProto(tt.internal))
			assert.Equal(t, tt.internal, protoTriggerTypeToInternal(tt.proto))
		})
	}

	assert.Equal(t, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_UNSPECIFIED, triggerTypeToProto(workflow.TriggerType("unknown")))
	assert.Equal(t, workflow.TriggerTypeManual, protoTriggerTypeToInternal(immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_UNSPECIFIED))
}

func TestWorkflowActionTypeMappings(t *testing.T) {
	tests := []struct {
		name     string
		internal workflow.ActionType
		proto    immichv1.WorkflowActionType
	}{
		{"add tag", workflow.ActionTypeAddTag, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_ADD_TAG},
		{"remove tag", workflow.ActionTypeRemoveTag, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_REMOVE_TAG},
		{"move to album", workflow.ActionTypeMoveToAlbum, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_MOVE_TO_ALBUM},
		{"set visibility", workflow.ActionTypeSetVisibility, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_VISIBILITY},
		{"send notification", workflow.ActionTypeSendNotification, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SEND_NOTIFICATION},
		{"webhook", workflow.ActionTypeWebhook, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_WEBHOOK},
		{"run plugin", workflow.ActionTypeRunPlugin, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_RUN_PLUGIN},
		{"set metadata", workflow.ActionTypeSetMetadata, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_METADATA},
		{"generate thumbnail", workflow.ActionTypeGenerateThumbnail, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_GENERATE_THUMBNAIL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.proto, actionTypeToProto(tt.internal))
			assert.Equal(t, tt.internal, protoActionTypeToInternal(tt.proto))
		})
	}

	assert.Equal(t, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_UNSPECIFIED, actionTypeToProto(workflow.ActionType("unknown")))
	assert.Equal(t, workflow.ActionTypeWebhook, protoActionTypeToInternal(immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_UNSPECIFIED))
}

func TestWorkflowExecutionStatusMappings(t *testing.T) {
	tests := []struct {
		name     string
		internal workflow.ExecutionStatus
		proto    immichv1.WorkflowExecutionStatus
	}{
		{"pending", workflow.ExecutionStatusPending, immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_PENDING},
		{"running", workflow.ExecutionStatusRunning, immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_RUNNING},
		{"completed", workflow.ExecutionStatusCompleted, immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_COMPLETED},
		{"failed", workflow.ExecutionStatusFailed, immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_FAILED},
		{"cancelled", workflow.ExecutionStatusCancelled, immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_CANCELLED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.proto, executionStatusToProto(tt.internal))
			assert.Equal(t, tt.internal, protoExecutionStatusToInternal(tt.proto))
		})
	}

	assert.Equal(t, immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_UNSPECIFIED, executionStatusToProto(workflow.ExecutionStatus("unknown")))
	assert.Equal(t, workflow.ExecutionStatusPending, protoExecutionStatusToInternal(immichv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_UNSPECIFIED))
}

func TestWorkflowToProto(t *testing.T) {
	createdAt := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	lastExecutionAt := updatedAt.Add(time.Hour)
	w := &workflow.WorkflowInfo{
		ID:              "workflow-1",
		Name:            "Archive",
		Description:     "Archive old assets",
		Status:          workflow.WorkflowStatusActive,
		Enabled:         true,
		Trigger:         workflow.Trigger{Type: workflow.TriggerTypeScheduled, CronExpression: "0 0 * * *"},
		Actions:         []workflow.Action{{Type: workflow.ActionTypeSetVisibility, Order: 2}},
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		CreatedBy:       "user-1",
		ErrorMessage:    "warning",
		ExecutionCount:  7,
		LastExecutionAt: &lastExecutionAt,
	}

	got := workflowToProto(w)

	require.NotNil(t, got)
	assert.Equal(t, "workflow-1", got.Id)
	assert.Equal(t, "Archive", got.Name)
	assert.Equal(t, immichv1.WorkflowStatus_WORKFLOW_STATUS_ACTIVE, got.Status)
	assert.True(t, got.Enabled)
	assert.Equal(t, immichv1.WorkflowTriggerType_WORKFLOW_TRIGGER_TYPE_SCHEDULED, got.Trigger.Type)
	require.NotNil(t, got.Trigger.CronExpression)
	assert.Equal(t, "0 0 * * *", *got.Trigger.CronExpression)
	require.Len(t, got.Actions, 1)
	assert.Equal(t, immichv1.WorkflowActionType_WORKFLOW_ACTION_TYPE_SET_VISIBILITY, got.Actions[0].Type)
	require.NotNil(t, got.Actions[0].Order)
	assert.Equal(t, int32(2), *got.Actions[0].Order)
	assert.Equal(t, createdAt, got.CreatedAt.AsTime())
	assert.Equal(t, updatedAt, got.UpdatedAt.AsTime())
	assert.Equal(t, int32(7), got.ExecutionCount)
	require.NotNil(t, got.CreatedBy)
	assert.Equal(t, "user-1", *got.CreatedBy)
	require.NotNil(t, got.ErrorMessage)
	assert.Equal(t, "warning", *got.ErrorMessage)
	assert.Equal(t, lastExecutionAt, got.LastExecutionAt.AsTime())
}
