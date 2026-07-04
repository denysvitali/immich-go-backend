package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/download"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/storage"
)

// buildDownloadRequest converts the proto download selectors (asset IDs,
// album ID, owner user ID) into a download.DownloadRequest understood by the
// download service. Matching upstream Immich semantics, exactly one selector
// drives the request: an album ID takes precedence, otherwise the explicit
// asset IDs are used, optionally expanded with all assets owned by the given
// user ID. Per-asset access control (ownership or shared-album access) is
// enforced by the download service.
func (s *Server) buildDownloadRequest(ctx context.Context, assetIDs []string, albumID, ownerID *string) (*download.DownloadRequest, error) {
	if len(assetIDs) == 0 && albumID == nil && ownerID == nil {
		return nil, status.Error(codes.InvalidArgument, "at least one of asset_ids, album_id or user_id must be provided")
	}

	if albumID != nil {
		if _, err := uuid.Parse(*albumID); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %s", *albumID)
		}
		return &download.DownloadRequest{AlbumID: albumID}, nil
	}

	ids := make([]string, 0, len(assetIDs))
	for _, id := range assetIDs {
		if _, err := uuid.Parse(id); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid asset ID: %s", id)
		}
		ids = append(ids, id)
	}

	if ownerID != nil {
		ownerUUID, err := pgutil.StringToUUID(*ownerID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %s", *ownerID)
		}

		ownedAssets, err := s.db.GetUserAssets(ctx, sqlc.GetUserAssetsParams{
			OwnerId: ownerUUID,
		})
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to list user assets", err)
		}

		for _, asset := range ownedAssets {
			ids = append(ids, uuid.UUID(asset.ID.Bytes).String())
		}
	}

	if len(ids) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no assets to download")
	}

	return &download.DownloadRequest{AssetIDs: ids}, nil
}

// GetDownloadInfo returns size information and the resolved set of asset IDs
// for a potential download (POST /api/download/info).
func (s *Server) GetDownloadInfo(ctx context.Context, request *immichv1.DownloadInfoRequest) (*immichv1.DownloadInfoResponse, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	req, err := s.buildDownloadRequest(ctx, request.GetAssetIds(), request.AlbumId, request.UserId)
	if err != nil {
		return nil, err
	}

	info, err := s.downloadService.GetDownloadInfo(ctx, uuid.UUID(userID.Bytes), req)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get download info", err)
	}

	return &immichv1.DownloadInfoResponse{
		ArchiveSize: info.ArchiveSize,
		AssetIds:    info.AssetIDs,
		AlbumId:     request.AlbumId,
		UserId:      request.UserId,
	}, nil
}

// DownloadArchive resolves the archive contents for a download request
// (POST /api/download/archive). The RPC is unary and its response carries
// archive metadata (per-asset path and size plus the total size); the actual
// bytes of each asset are fetched separately via the asset download
// endpoints.
func (s *Server) DownloadArchive(ctx context.Context, request *immichv1.DownloadArchiveRequest) (*immichv1.DownloadResponse, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	req, err := s.buildDownloadRequest(ctx, request.GetAssetIds(), request.AlbumId, request.UserId)
	if err != nil {
		return nil, err
	}

	// Resolve and authorize the asset set through the download service. Only
	// assets the caller owns or has shared access to are returned.
	info, err := s.downloadService.GetDownloadInfo(ctx, uuid.UUID(userID.Bytes), req)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to resolve download archive", err)
	}

	archives := make([]*immichv1.DownloadArchiveInfo, 0, len(info.AssetIDs))
	var totalSize int64

	for _, assetIDStr := range info.AssetIDs {
		assetUUID, err := uuid.Parse(assetIDStr)
		if err != nil {
			continue
		}

		asset, err := s.db.GetAsset(ctx, pgtype.UUID{Bytes: assetUUID, Valid: true})
		if err != nil {
			continue
		}

		assetPath := asset.OriginalPath
		if assetPath == "" {
			assetPath = storage.AssetFallbackPath(assetUUID, asset.OriginalFileName)
		}

		var size int64
		exif, err := s.db.GetExifByAssetId(ctx, pgtype.UUID{Bytes: assetUUID, Valid: true})
		if err == nil && exif.FileSizeInByte.Valid {
			size = exif.FileSizeInByte.Int64
		}
		totalSize += size

		archives = append(archives, &immichv1.DownloadArchiveInfo{
			AssetId:   assetIDStr,
			AssetPath: assetPath,
			Size:      size,
		})
	}

	return &immichv1.DownloadResponse{
		TotalSize: totalSize,
		Archives:  archives,
	}, nil
}
