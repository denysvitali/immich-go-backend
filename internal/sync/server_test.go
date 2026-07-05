package sync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

const (
	syncTestUserID    = "11111111-2222-3333-4444-555555555555"
	syncSpoofedUserID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

var _ syncService = (*fakeSyncService)(nil)

type fakeSyncService struct {
	acknowledgedAssets []string
	getAckUserID       string

	ackUserID   string
	ackAssetIDs []string

	deleteAckUserID   string
	deleteAckAssetIDs []string

	deltaUserID       string
	deltaUpdatedAfter time.Time
	deltaResult       *DeltaSyncResult

	fullUserID       string
	fullLimit        int
	fullUpdatedUntil *time.Time
	fullAssetIDs     []string
	fullHasMore      bool
	fullLastUpdated  *time.Time
}

func (f *fakeSyncService) GetAcknowledgedAssets(ctx context.Context, userID string) ([]string, error) {
	f.getAckUserID = userID
	return f.acknowledgedAssets, nil
}

func (f *fakeSyncService) AcknowledgeSync(ctx context.Context, userID string, assetIDs []string) error {
	f.ackUserID = userID
	f.ackAssetIDs = assetIDs
	return nil
}

func (f *fakeSyncService) DeleteAcknowledgment(ctx context.Context, userID string, assetIDs []string) error {
	f.deleteAckUserID = userID
	f.deleteAckAssetIDs = assetIDs
	return nil
}

func (f *fakeSyncService) GetDeltaSync(ctx context.Context, userID string, updatedAfter time.Time) (*DeltaSyncResult, error) {
	f.deltaUserID = userID
	f.deltaUpdatedAfter = updatedAfter
	if f.deltaResult != nil {
		return f.deltaResult, nil
	}
	return &DeltaSyncResult{}, nil
}

func (f *fakeSyncService) GetFullSync(ctx context.Context, userID string, limit int, updatedUntil *time.Time) ([]string, bool, *time.Time, error) {
	f.fullUserID = userID
	f.fullLimit = limit
	f.fullUpdatedUntil = updatedUntil
	return f.fullAssetIDs, f.fullHasMore, f.fullLastUpdated, nil
}

func (f *fakeSyncService) SubscribeToEvents(userID string) chan *SyncEvent {
	return make(chan *SyncEvent)
}

func (f *fakeSyncService) UnsubscribeFromEvents(userID string, eventChan chan *SyncEvent) {}

func TestCurrentUserIDFromContext(t *testing.T) {
	got, err := currentUserIDFromContext(syncTestContext(syncTestUserID))
	require.NoError(t, err)
	assert.Equal(t, syncTestUserID, got)

	_, err = currentUserIDFromContext(context.Background())
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestSyncAckMethodsUseAuthenticatedUser(t *testing.T) {
	service := &fakeSyncService{
		acknowledgedAssets: []string{"asset-1", "asset-2"},
	}
	server := newServer(service)
	ctx := syncTestContext(syncTestUserID)

	getResp, err := server.GetSyncAck(ctx, &immichv1.GetSyncAckRequest{})
	require.NoError(t, err)
	assert.Equal(t, syncTestUserID, service.getAckUserID)
	assert.Equal(t, []string{"asset-1", "asset-2"}, getResp.AssetIds)

	_, err = server.SendSyncAck(ctx, &immichv1.SendSyncAckRequest{AssetIds: []string{"asset-3"}})
	require.NoError(t, err)
	assert.Equal(t, syncTestUserID, service.ackUserID)
	assert.Equal(t, []string{"asset-3"}, service.ackAssetIDs)

	_, err = server.DeleteSyncAck(ctx, &immichv1.DeleteSyncAckRequest{AssetIds: []string{"asset-4"}})
	require.NoError(t, err)
	assert.Equal(t, syncTestUserID, service.deleteAckUserID)
	assert.Equal(t, []string{"asset-4"}, service.deleteAckAssetIDs)
}

func TestGetDeltaSyncUsesAuthenticatedUserWithoutMutatingRequest(t *testing.T) {
	updatedAfter := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	service := &fakeSyncService{
		deltaResult: &DeltaSyncResult{
			NeedsFullSync:  false,
			UpsertedAssets: []string{"asset-1"},
			DeletedAssets:  []string{"asset-2"},
		},
	}
	server := newServer(service)
	req := &immichv1.GetDeltaSyncRequest{
		UserId:       stringPtr(syncSpoofedUserID),
		UpdatedAfter: timestamppb.New(updatedAfter),
	}

	resp, err := server.GetDeltaSync(syncTestContext(syncTestUserID), req)
	require.NoError(t, err)

	assert.Equal(t, syncTestUserID, service.deltaUserID)
	assert.True(t, service.deltaUpdatedAfter.Equal(updatedAfter))
	assert.Equal(t, syncSpoofedUserID, req.GetUserId())
	assert.False(t, resp.NeedsFullSync)
	assert.Equal(t, []string{"asset-1"}, resp.Upserted)
	assert.Equal(t, []string{"asset-2"}, resp.Deleted)
}

func TestGetFullSyncForUserUsesAuthenticatedUserWithoutMutatingRequest(t *testing.T) {
	lastUpdated := time.Date(2026, 7, 5, 13, 0, 0, 0, time.UTC)
	service := &fakeSyncService{
		fullAssetIDs:    []string{"asset-1", "asset-2"},
		fullHasMore:     true,
		fullLastUpdated: &lastUpdated,
	}
	server := newServer(service)
	req := &immichv1.GetFullSyncForUserRequest{
		UserId: stringPtr(syncSpoofedUserID),
	}

	resp, err := server.GetFullSyncForUser(syncTestContext(syncTestUserID), req)
	require.NoError(t, err)

	assert.Equal(t, syncTestUserID, service.fullUserID)
	assert.Equal(t, 1000, service.fullLimit)
	assert.Nil(t, service.fullUpdatedUntil)
	assert.Equal(t, syncSpoofedUserID, req.GetUserId())
	assert.Equal(t, []string{"asset-1", "asset-2"}, resp.AssetIds)
	assert.True(t, resp.HasMore)
	require.NotNil(t, resp.LastUpdated)
	assert.True(t, resp.LastUpdated.AsTime().Equal(lastUpdated))
}

func syncTestContext(userID string) context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{UserID: userID})
}

func stringPtr(value string) *string {
	return &value
}
