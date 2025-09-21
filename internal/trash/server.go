package trash

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server implements the TrashService
type Server struct {
	immichv1.UnimplementedTrashServiceServer
	queries *sqlc.Queries
}

// NewServer creates a new trash server
func NewServer(queries *sqlc.Queries) *Server {
	return &Server{
		queries: queries,
	}
}

// EmptyTrash permanently deletes all trashed assets for the user
func (s *Server) EmptyTrash(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get all trashed assets for the user
	trashedAssets, err := s.queries.GetTrashedAssetsByUser(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get trashed assets: %v", err)
	}

	// Permanently delete each trashed asset
	for _, asset := range trashedAssets {
		err = s.queries.PermanentlyDeleteAsset(ctx, asset.ID)
		if err != nil {
			// Log error but continue with other assets
			continue
		}
	}

	return &emptypb.Empty{}, nil
}

// RestoreTrash restores all trashed assets for the user
func (s *Server) RestoreTrash(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get all trashed assets for the user
	trashedAssets, err := s.queries.GetTrashedAssetsByUser(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get trashed assets: %v", err)
	}

	// Restore each trashed asset
	for _, asset := range trashedAssets {
		err = s.queries.RestoreAssetFromTrash(ctx, asset.ID)
		if err != nil {
			// Log error but continue with other assets
			continue
		}
	}

	return &emptypb.Empty{}, nil
}

// RestoreAssets restores specific assets from trash
func (s *Server) RestoreAssets(ctx context.Context, request *immichv1.RestoreAssetsRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Collect valid asset UUIDs
	var assetUUIDs []pgtype.UUID
	for _, assetIDStr := range request.GetAssetIds() {
		// Parse asset ID
		assetID, err := uuid.Parse(assetIDStr)
		if err != nil {
			continue // Skip invalid IDs
		}
		assetUUIDs = append(assetUUIDs, pgtype.UUID{Bytes: assetID, Valid: true})
	}

	// Restore all assets in batch (ownership check is done in the query)
	if len(assetUUIDs) > 0 {
		err = s.queries.RestoreAssets(ctx, sqlc.RestoreAssetsParams{
			Column1: assetUUIDs,
			OwnerId: userUUID,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to restore assets: %v", err)
		}
	}

	return &emptypb.Empty{}, nil
}
