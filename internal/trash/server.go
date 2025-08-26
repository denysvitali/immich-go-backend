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

	// Convert to pgtype.UUID
	var pgUserID pgtype.UUID
	if err := pgUserID.Scan(claims.UserID); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

	// TODO: Implement when SQLC queries are generated
	// This requires GetTrashedAssetsByUser and PermanentlyDeleteAsset queries
	// For now, return success
	_ = pgUserID

	return &emptypb.Empty{}, nil
}

// RestoreTrash restores all trashed assets for the user
func (s *Server) RestoreTrash(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Convert to pgtype.UUID
	var pgUserID pgtype.UUID
	if err := pgUserID.Scan(claims.UserID); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

	// TODO: Implement when SQLC queries are generated
	// This requires GetTrashedAssetsByUser and RestoreAssetFromTrash queries
	// For now, return success
	_ = pgUserID

	return &emptypb.Empty{}, nil
}

// RestoreAssets restores specific assets from trash
func (s *Server) RestoreAssets(ctx context.Context, request *immichv1.RestoreAssetsRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Convert to pgtype.UUID
	var pgUserID pgtype.UUID
	if err := pgUserID.Scan(claims.UserID); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

	// Restore each specified asset
	for _, assetIDStr := range request.GetAssetIds() {
		// Parse asset ID
		assetID, err := uuid.Parse(assetIDStr)
		if err != nil {
			continue // Skip invalid IDs
		}

		var pgAssetID pgtype.UUID
		if err := pgAssetID.Scan(assetID); err != nil {
			continue
		}

		// Get asset to verify ownership
		asset, err := s.queries.GetAssetByID(ctx, pgAssetID)
		if err != nil {
			continue // Asset not found or error
		}

		// Verify ownership
		if asset.OwnerId != pgUserID {
			continue // Skip assets not owned by user
		}

		// TODO: Restore asset from trash when RestoreAssetFromTrash query is available
		// err = s.queries.RestoreAssetFromTrash(ctx, pgAssetID)
		if err != nil {
			// Log error but continue with other assets
			continue
		}
	}

	return &emptypb.Empty{}, nil
}