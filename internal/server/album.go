package server

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) GetAllAlbums(ctx context.Context, request *immichv1.GetAllAlbumsRequest) (*immichv1.GetAllAlbumsResponse, error) {
	// TODO: Get user ID from context/auth
	// userID := "00000000-0000-0000-0000-000000000000" // Placeholder

	albums, err := s.db.GetAlbums(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get albums: %v", err)
	}

	immichAlbums := make([]*immichv1.Album, len(albums))
	for i, album := range albums {
		immichAlbums[i] = s.convertAlbumToProto(album)
	}
	return &immichv1.GetAllAlbumsResponse{Albums: immichAlbums}, nil
}

func (s *Server) CreateAlbum(ctx context.Context, request *immichv1.CreateAlbumRequest) (*immichv1.Album, error) {
	// TODO: Get user ID from context/auth
	userID := pgtype.UUID{}
	if err := userID.Scan("00000000-0000-0000-0000-000000000000"); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

	album, err := s.db.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:     userID,
		AlbumName:   request.AlbumName,
		Description: request.Description,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create album: %v", err)
	}

	// Add assets to album if provided
	for _, assetID := range request.AssetIds {
		assetUUID := pgtype.UUID{}
		if err := assetUUID.Scan(assetID); err != nil {
			continue // Skip invalid UUIDs
		}
		_ = s.db.AddAssetToAlbum(ctx, sqlc.AddAssetToAlbumParams{
			AlbumsId: album.ID,
			AssetsId: assetUUID,
		})
	}

	return s.convertAlbumToProto(album), nil
}

func (s *Server) GetAlbumInfo(ctx context.Context, request *immichv1.GetAlbumInfoRequest) (*immichv1.Album, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.AlbumId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}

	album, err := s.db.GetAlbum(ctx, albumID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "album not found: %v", err)
	}

	return s.convertAlbumToProto(album), nil
}

func (s *Server) UpdateAlbumInfo(ctx context.Context, request *immichv1.UpdateAlbumInfoRequest) (*immichv1.Album, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.AlbumId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}

	var albumName, description pgtype.Text
	var thumbnailAssetID pgtype.UUID
	var isActivityEnabled pgtype.Bool

	if request.AlbumName != nil {
		albumName = pgtype.Text{String: *request.AlbumName, Valid: true}
	}
	if request.Description != nil {
		description = pgtype.Text{String: *request.Description, Valid: true}
	}
	if request.AlbumThumbnailAssetId != nil {
		if err := thumbnailAssetID.Scan(*request.AlbumThumbnailAssetId); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid thumbnail asset ID: %v", err)
		}
	}
	if request.IsActivityEnabled != nil {
		isActivityEnabled = pgtype.Bool{Bool: *request.IsActivityEnabled, Valid: true}
	}

	album, err := s.db.UpdateAlbum(ctx, sqlc.UpdateAlbumParams{
		ID:                     albumID,
		AlbumName:              albumName,
		Description:            description,
		AlbumThumbnailAssetID:  thumbnailAssetID,
		IsActivityEnabled:      isActivityEnabled,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update album: %v", err)
	}

	return s.convertAlbumToProto(album), nil
}

func (s *Server) DeleteAlbum(ctx context.Context, request *immichv1.DeleteAlbumRequest) (*emptypb.Empty, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.AlbumId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}

	if err := s.db.DeleteAlbum(ctx, albumID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete album: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) AddAssetsToAlbum(ctx context.Context, request *immichv1.AddAssetsToAlbumRequest) (*immichv1.AddAssetsToAlbumResponse, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.AlbumId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}

	results := make([]*immichv1.BulkIdResponse, len(request.AssetIds.Ids))
	for i, assetID := range request.AssetIds.Ids {
		assetUUID := pgtype.UUID{}
		if err := assetUUID.Scan(assetID); err != nil {
			results[i] = &immichv1.BulkIdResponse{
				Id:      assetID,
				Success: false,
				Error:   &[]string{"invalid asset ID"}[0],
			}
			continue
		}

		err := s.db.AddAssetToAlbum(ctx, sqlc.AddAssetToAlbumParams{
			AlbumsId: albumID,
			AssetsId: assetUUID,
		})
		results[i] = &immichv1.BulkIdResponse{
			Id:      assetID,
			Success: err == nil,
		}
		if err != nil {
			errMsg := err.Error()
			results[i].Error = &errMsg
		}
	}

	return &immichv1.AddAssetsToAlbumResponse{Results: results}, nil
}

func (s *Server) RemoveAssetFromAlbum(ctx context.Context, request *immichv1.RemoveAssetFromAlbumRequest) (*immichv1.RemoveAssetFromAlbumResponse, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.AlbumId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}

	results := make([]*immichv1.BulkIdResponse, len(request.AssetIds.Ids))
	for i, assetID := range request.AssetIds.Ids {
		assetUUID := pgtype.UUID{}
		if err := assetUUID.Scan(assetID); err != nil {
			results[i] = &immichv1.BulkIdResponse{
				Id:      assetID,
				Success: false,
				Error:   &[]string{"invalid asset ID"}[0],
			}
			continue
		}

		err := s.db.RemoveAssetFromAlbum(ctx, sqlc.RemoveAssetFromAlbumParams{
			AlbumsId: albumID,
			AssetsId: assetUUID,
		})
		results[i] = &immichv1.BulkIdResponse{
			Id:      assetID,
			Success: err == nil,
		}
		if err != nil {
			errMsg := err.Error()
			results[i].Error = &errMsg
		}
	}

	return &immichv1.RemoveAssetFromAlbumResponse{Results: results}, nil
}

func (s *Server) AddUsersToAlbum(ctx context.Context, request *immichv1.AddUsersToAlbumRequest) (*immichv1.Album, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.AlbumId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}

	for _, userID := range request.SharedUserIds {
		userUUID := pgtype.UUID{}
		if err := userUUID.Scan(userID); err != nil {
			continue // Skip invalid UUIDs
		}
		_ = s.db.AddUserToAlbum(ctx, sqlc.AddUserToAlbumParams{
			AlbumsId: albumID,
			UsersId:  userUUID,
			Role:     "editor", // Default role
		})
	}

	// Return updated album
	album, err := s.db.GetAlbum(ctx, albumID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get updated album: %v", err)
	}

	return s.convertAlbumToProto(album), nil
}

func (s *Server) RemoveUserFromAlbum(ctx context.Context, request *immichv1.RemoveUserFromAlbumRequest) (*emptypb.Empty, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.AlbumId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}

	userID := pgtype.UUID{}
	if err := userID.Scan(request.UserId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	if err := s.db.RemoveUserFromAlbum(ctx, sqlc.RemoveUserFromAlbumParams{
		AlbumsId: albumID,
		UsersId:  userID,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove user from album: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) UpdateAlbumUser(ctx context.Context, request *immichv1.UpdateAlbumUserRequest) (*emptypb.Empty, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.AlbumId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}

	userID := pgtype.UUID{}
	if err := userID.Scan(request.UserId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	role := "viewer"
	if request.Role == immichv1.AlbumUserRole_ALBUM_USER_ROLE_EDITOR {
		role = "editor"
	}

	if err := s.db.AddUserToAlbum(ctx, sqlc.AddUserToAlbumParams{
		AlbumsId: albumID,
		UsersId:  userID,
		Role:     role,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update album user: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) GetAlbumStatistics(ctx context.Context, request *immichv1.GetAlbumStatisticsRequest) (*immichv1.AlbumStatisticsResponse, error) {
	// TODO: Get user ID from context/auth
	userID := pgtype.UUID{}
	if err := userID.Scan("00000000-0000-0000-0000-000000000000"); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

	stats, err := s.db.GetAlbumStatistics(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get album statistics: %v", err)
	}

	return &immichv1.AlbumStatisticsResponse{
		Owned:     int32(stats.Owned),
		Shared:    int32(stats.Shared),
		NotShared: int32(stats.NotShared),
	}, nil
}

// Helper function to convert database album to proto
func (s *Server) convertAlbumToProto(album sqlc.Album) *immichv1.Album {
	protoAlbum := &immichv1.Album{
		Id:                album.ID.String(),
		AlbumName:         album.AlbumName,
		Description:       album.Description,
		OwnerId:           album.OwnerId.String(),
		IsActivityEnabled: album.IsActivityEnabled,
		CreatedAt:         timestamppb.New(album.CreatedAt.Time),
		UpdatedAt:         timestamppb.New(album.UpdatedAt.Time),
	}

	if album.AlbumThumbnailAssetId.Valid {
		thumbnailID := album.AlbumThumbnailAssetId.String()
		protoAlbum.AlbumThumbnailAssetId = &thumbnailID
	}

	return protoAlbum
}
