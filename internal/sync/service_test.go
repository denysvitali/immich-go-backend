package sync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceAcknowledgments(t *testing.T) {
	service := NewService(nil, nil)
	require.NotNil(t, service.logger)

	ctx := context.Background()
	userID := "11111111-2222-3333-4444-555555555555"

	require.NoError(t, service.AcknowledgeSync(ctx, userID, []string{"asset-1", "asset-2"}))

	got, err := service.GetAcknowledgedAssets(ctx, userID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"asset-1", "asset-2"}, got)

	require.NoError(t, service.DeleteAcknowledgment(ctx, userID, []string{"asset-1"}))

	got, err = service.GetAcknowledgedAssets(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, []string{"asset-2"}, got)

	require.NoError(t, service.ClearUserSyncState(ctx, userID))

	got, err = service.GetAcknowledgedAssets(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestServiceBroadcastsAssetEventsToUserSubscribers(t *testing.T) {
	service := NewService(nil, nil)
	userID := "11111111-2222-3333-4444-555555555555"

	ch := service.SubscribeToEvents(userID)
	service.BroadcastAssetEvent(userID, "asset-1", "upsert")

	select {
	case event := <-ch:
		require.NotNil(t, event)
		assert.Equal(t, "asset", event.Type)
		assert.Equal(t, "upsert", event.Action)
		assert.Equal(t, userID, event.UserID)
		assert.Equal(t, "asset-1", event.ResourceID)
		assert.False(t, event.Timestamp.IsZero())
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sync event")
	}

	service.UnsubscribeFromEvents(userID, ch)
	_, ok := <-ch
	assert.False(t, ok)
}

func TestServiceDoesNotBroadcastEventsAcrossUsers(t *testing.T) {
	service := NewService(nil, nil)
	ch := service.SubscribeToEvents("user-1")
	defer service.UnsubscribeFromEvents("user-1", ch)

	service.BroadcastAssetEvent("user-2", "asset-1", "upsert")

	select {
	case event := <-ch:
		t.Fatalf("received event for another user: %+v", event)
	default:
	}
}
