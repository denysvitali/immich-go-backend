package sync

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// Server implements the SyncServiceServer interface
type Server struct {
	service *Service
	immichv1.UnimplementedSyncServiceServer
}

// NewServer creates a new Sync server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// GetSyncAck returns acknowledged asset IDs for the current user
func (s *Server) GetSyncAck(ctx context.Context, _ *immichv1.GetSyncAckRequest) (*immichv1.GetSyncAckResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	assetIDs, err := s.service.GetAcknowledgedAssets(ctx, userID.String())
	if err != nil {
		return nil, err
	}

	return &immichv1.GetSyncAckResponse{
		AssetIds: assetIDs,
	}, nil
}

// SendSyncAck acknowledges sync for specified assets
func (s *Server) SendSyncAck(ctx context.Context, req *immichv1.SendSyncAckRequest) (*emptypb.Empty, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.service.AcknowledgeSync(ctx, userID.String(), req.AssetIds); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// DeleteSyncAck removes acknowledgment for specified assets
func (s *Server) DeleteSyncAck(ctx context.Context, req *immichv1.DeleteSyncAckRequest) (*emptypb.Empty, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.service.DeleteAcknowledgment(ctx, userID.String(), req.AssetIds); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// GetDeltaSync returns changes since the specified timestamp
func (s *Server) GetDeltaSync(ctx context.Context, req *immichv1.GetDeltaSyncRequest) (*immichv1.GetDeltaSyncResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Override with requesting user if not admin
	userIDStr := userID.String()
	// Check if user is admin (would need admin check in real implementation)
	// For now, only allow users to sync their own data
	req.UserId = &userIDStr

	updatedAfter := time.Now().Add(-24 * time.Hour) // Default to last 24 hours
	if req.UpdatedAfter != nil {
		updatedAfter = req.UpdatedAfter.AsTime()
	}

	result, err := s.service.GetDeltaSync(ctx, *req.UserId, updatedAfter)
	if err != nil {
		return nil, err
	}

	return &immichv1.GetDeltaSyncResponse{
		NeedsFullSync: result.NeedsFullSync,
		Upserted:      result.UpsertedAssets,
		Deleted:       result.DeletedAssets,
	}, nil
}

// GetFullSyncForUser returns all assets for a user with pagination
func (s *Server) GetFullSyncForUser(ctx context.Context, req *immichv1.GetFullSyncForUserRequest) (*immichv1.GetFullSyncForUserResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Override with requesting user if not admin
	userIDStr := userID.String()
	// Check if user is admin (would need admin check in real implementation)
	// For now, only allow users to sync their own data
	req.UserId = &userIDStr

	limit := 1000
	if req.Limit != nil {
		limit = int(*req.Limit)
	}

	var updatedUntil *time.Time
	if req.UpdatedUntil != nil {
		t := req.UpdatedUntil.AsTime()
		updatedUntil = &t
	}

	assetIDs, hasMore, lastUpdated, err := s.service.GetFullSync(ctx, *req.UserId, limit, updatedUntil)
	if err != nil {
		return nil, err
	}

	response := &immichv1.GetFullSyncForUserResponse{
		AssetIds: assetIDs,
		HasMore:  hasMore,
	}

	if lastUpdated != nil {
		response.LastUpdated = timestamppb.New(*lastUpdated)
	}

	return response, nil
}

// GetSyncStream returns a stream of sync events (for real-time updates)
func (s *Server) GetSyncStream(req *immichv1.GetSyncStreamRequest, stream immichv1.SyncService_GetSyncStreamServer) error {
	ctx := stream.Context()

	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return err
	}

	// Override with requesting user if not admin
	userIDStr := userID.String()
	// Check if user is admin (would need admin check in real implementation)
	// For now, only allow users to sync their own data
	req.UserId = &userIDStr

	// In a real implementation, this would:
	// 1. Subscribe to real-time events (from Redis pub/sub or similar)
	// 2. Stream events as they occur
	// 3. Handle disconnections gracefully

	// For now, send a test event and complete
	testEvent := &immichv1.SyncStreamResponse{
		Event: &immichv1.SyncStreamResponse_AssetEvent{
			AssetEvent: &immichv1.AssetSyncEvent{
				Type:      "upsert",
				AssetId:   "test-asset-id",
				Timestamp: timestamppb.Now(),
			},
		},
	}

	if err := stream.Send(testEvent); err != nil {
		return err
	}

	// Keep the stream open until context is cancelled
	<-ctx.Done()

	return ctx.Err()
}
