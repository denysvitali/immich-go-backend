package server

import (
	"context"
	"errors"

	"github.com/denysvitali/immich-go-backend/internal/grpcutil"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/ptr"
)

type albumAccessLevel uint8

const (
	albumAccessView albumAccessLevel = iota
	albumAccessEdit
	albumAccessManage
)

// authorizedAlbum loads an album and verifies that the authenticated user has
// the requested access. Owners can perform every operation, shared editors can
// change album assets, and shared viewers have read-only access.
func (s *Server) authorizedAlbum(ctx context.Context, albumID pgtype.UUID, required albumAccessLevel) (sqlc.Album, pgtype.UUID, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return sqlc.Album{}, pgtype.UUID{}, err
	}

	userID, err := pgutil.StringToUUID(claims.UserID)
	if err != nil {
		return sqlc.Album{}, pgtype.UUID{}, grpcutil.SanitizedInternal(ctx, "invalid user ID", err)
	}

	album, err := s.db.GetAlbum(ctx, albumID)
	if err != nil {
		return sqlc.Album{}, pgtype.UUID{}, status.Error(codes.NotFound, "album not found")
	}
	if album.OwnerId == userID {
		return album, userID, nil
	}
	if required == albumAccessManage {
		return sqlc.Album{}, pgtype.UUID{}, status.Error(codes.PermissionDenied, "access denied")
	}

	sharedUsers, err := s.db.GetAlbumSharedUsers(ctx, albumID)
	if err != nil {
		return sqlc.Album{}, pgtype.UUID{}, grpcutil.SanitizedInternal(ctx, "failed to check album access", err)
	}
	for _, sharedUser := range sharedUsers {
		if sharedUser.ID != userID {
			continue
		}
		if !sharedAlbumAccessAllowed(sharedUser.Role, required) {
			break
		}
		return album, userID, nil
	}

	return sqlc.Album{}, pgtype.UUID{}, status.Error(codes.PermissionDenied, "access denied")
}

func sharedAlbumAccessAllowed(role string, required albumAccessLevel) bool {
	return required == albumAccessView || (required == albumAccessEdit && role == "editor")
}

func (s *Server) GetAllAlbums(ctx context.Context, request *immichv1.GetAllAlbumsRequest) (*immichv1.GetAllAlbumsResponse, error) {
	// Get user ID from context/auth
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "invalid user ID", err)
	}

	// Get albums owned by the user
	albums, err := s.db.GetAlbumsByOwner(ctx, userID)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get albums", err)
	}

	immichAlbums := make([]*immichv1.Album, len(albums))
	for i, album := range albums {
		immichAlbums[i] = s.convertAlbumToProto(album)
	}
	return &immichv1.GetAllAlbumsResponse{Albums: immichAlbums}, nil
}

func (s *Server) CreateAlbum(ctx context.Context, request *immichv1.CreateAlbumRequest) (*immichv1.Album, error) {
	// Get user ID from context/auth
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "invalid user ID", err)
	}

	assetIDs := make([]pgtype.UUID, 0, len(request.AssetIds))
	for _, assetID := range request.AssetIds {
		assetUUID, err := pgutil.StringToUUID(assetID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
		}
		if _, err := s.db.GetAssetByIDAndUser(ctx, sqlc.GetAssetByIDAndUserParams{
			ID:      assetUUID,
			OwnerId: userID,
		}); err != nil {
			return nil, status.Error(codes.NotFound, "asset not found")
		}
		assetIDs = append(assetIDs, assetUUID)
	}

	album, err := s.db.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:     userID,
		AlbumName:   request.AlbumName,
		Description: request.Description,
	})
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to create album", err)
	}

	// Add assets to album if provided
	for _, assetUUID := range assetIDs {
		_ = s.db.AddAssetToAlbum(ctx, sqlc.AddAssetToAlbumParams{
			AlbumsId: album.ID,
			AssetsId: assetUUID,
		})
	}

	return s.convertAlbumToProto(album), nil
}

func (s *Server) GetAlbumInfo(ctx context.Context, request *immichv1.GetAlbumInfoRequest) (*immichv1.Album, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.Id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}

	album, _, err := s.authorizedAlbum(ctx, albumID, albumAccessView)
	if err != nil {
		return nil, err
	}

	return s.convertAlbumToProto(album), nil
}

func (s *Server) GetAlbumMapMarkers(ctx context.Context, request *immichv1.GetAlbumMapMarkersRequest) (*immichv1.GetAlbumMapMarkersResponse, error) {
	albumID, err := pgutil.StringToUUID(request.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}

	if _, _, err := s.authorizedAlbum(ctx, albumID, albumAccessView); err != nil {
		return nil, err
	}

	rows, err := s.db.GetAlbumMapMarkers(ctx, albumID)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get album map markers", err)
	}

	markers := make([]*immichv1.MapMarker, 0, len(rows))
	for _, row := range rows {
		markers = append(markers, albumMapMarkerToProto(row))
	}

	return &immichv1.GetAlbumMapMarkersResponse{Markers: markers}, nil
}

func (s *Server) UpdateAlbumInfo(ctx context.Context, request *immichv1.UpdateAlbumInfoRequest) (*immichv1.Album, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.Id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}
	_, userID, err := s.authorizedAlbum(ctx, albumID, albumAccessManage)
	if err != nil {
		return nil, err
	}

	albumName := pgutil.OptionalText(request.AlbumName)
	description := pgutil.OptionalText(request.Description)
	isActivityEnabled := pgutil.OptionalBool(request.IsActivityEnabled)

	var thumbnailAssetID pgtype.UUID
	if request.AlbumThumbnailAssetId != nil {
		if err := thumbnailAssetID.Scan(*request.AlbumThumbnailAssetId); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid thumbnail asset ID: %v", err)
		}
	}

	album, err := s.db.UpdateOwnedAlbum(ctx, sqlc.UpdateOwnedAlbumParams{
		OwnerID: userID,
		Update: sqlc.UpdateAlbumParams{
			ID:                    albumID,
			AlbumName:             albumName,
			Description:           description,
			AlbumThumbnailAssetID: thumbnailAssetID,
			IsActivityEnabled:     isActivityEnabled,
		},
	})
	if err != nil {
		if errors.Is(err, sqlc.ErrAlbumThumbnailInvalid) {
			return nil, status.Error(codes.InvalidArgument, "thumbnail asset must be owned by the caller and belong to the album")
		}
		if errors.Is(err, sqlc.ErrAlbumUpdateAccessDenied) {
			return nil, status.Error(codes.PermissionDenied, "access denied")
		}
		return nil, grpcutil.SanitizedInternal(ctx, "failed to update album", err)
	}

	return s.convertAlbumToProto(album), nil
}

func (s *Server) DeleteAlbum(ctx context.Context, request *immichv1.DeleteAlbumRequest) (*emptypb.Empty, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.Id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}
	if _, _, err := s.authorizedAlbum(ctx, albumID, albumAccessManage); err != nil {
		return nil, err
	}

	if err := s.db.DeleteAlbum(ctx, albumID); err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to delete album", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) AddAssetsToAlbum(ctx context.Context, request *immichv1.AddAssetsToAlbumRequest) (*immichv1.AddAssetsToAlbumResponse, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.Id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}
	_, userID, err := s.authorizedAlbum(ctx, albumID, albumAccessEdit)
	if err != nil {
		return nil, err
	}

	results := make([]*immichv1.BulkIdResponse, len(request.AssetIds.Ids))
	for i, assetID := range request.AssetIds.Ids {
		assetUUID := pgtype.UUID{}
		if err := assetUUID.Scan(assetID); err != nil {
			results[i] = &immichv1.BulkIdResponse{
				Id:      assetID,
				Success: false,
				Error:   ptr.To("invalid asset ID"),
			}
			continue
		}
		if _, err := s.db.GetAssetByIDAndUser(ctx, sqlc.GetAssetByIDAndUserParams{
			ID:      assetUUID,
			OwnerId: userID,
		}); err != nil {
			results[i] = &immichv1.BulkIdResponse{
				Id:      assetID,
				Success: false,
				Error:   ptr.To("asset not found"),
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
	if err := albumID.Scan(request.Id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}
	if _, _, err := s.authorizedAlbum(ctx, albumID, albumAccessEdit); err != nil {
		return nil, err
	}

	results := make([]*immichv1.BulkIdResponse, len(request.AssetIds.Ids))
	for i, assetID := range request.AssetIds.Ids {
		assetUUID := pgtype.UUID{}
		if err := assetUUID.Scan(assetID); err != nil {
			results[i] = &immichv1.BulkIdResponse{
				Id:      assetID,
				Success: false,
				Error:   ptr.To("invalid asset ID"),
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
	if err := albumID.Scan(request.Id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}
	if _, _, err := s.authorizedAlbum(ctx, albumID, albumAccessManage); err != nil {
		return nil, err
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
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get updated album", err)
	}

	return s.convertAlbumToProto(album), nil
}

func (s *Server) RemoveUserFromAlbum(ctx context.Context, request *immichv1.RemoveUserFromAlbumRequest) (*emptypb.Empty, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.Id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}
	if _, _, err := s.authorizedAlbum(ctx, albumID, albumAccessManage); err != nil {
		return nil, err
	}

	userID := pgtype.UUID{}
	if err := userID.Scan(request.UserId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	if err := s.db.RemoveUserFromAlbum(ctx, sqlc.RemoveUserFromAlbumParams{
		AlbumsId: albumID,
		UsersId:  userID,
	}); err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to remove user from album", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) UpdateAlbumUser(ctx context.Context, request *immichv1.UpdateAlbumUserRequest) (*emptypb.Empty, error) {
	albumID := pgtype.UUID{}
	if err := albumID.Scan(request.Id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
	}
	if _, _, err := s.authorizedAlbum(ctx, albumID, albumAccessManage); err != nil {
		return nil, err
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
		return nil, grpcutil.SanitizedInternal(ctx, "failed to update album user", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) GetAlbumStatistics(ctx context.Context, request *immichv1.GetAlbumStatisticsRequest) (*immichv1.AlbumStatisticsResponse, error) {
	// Get user ID from context/auth
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "invalid user ID", err)
	}

	stats, err := s.db.GetAlbumStatistics(ctx, userID)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get album statistics", err)
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

func albumMapMarkerToProto(row sqlc.GetAlbumMapMarkersRow) *immichv1.MapMarker {
	marker := &immichv1.MapMarker{
		Id:        row.ID.String(),
		Lat:       row.ExifLatitude.Float64,
		Lon:       row.ExifLongitude.Float64,
		Latitude:  row.ExifLatitude.Float64,
		Longitude: row.ExifLongitude.Float64,
	}

	if row.City.Valid {
		marker.City = row.City.String
	}
	if row.State.Valid {
		marker.State = row.State.String
	}
	if row.Country.Valid {
		marker.Country = row.Country.String
	}

	return marker
}

// AddAssetsToAlbums adds a set of assets to multiple albums owned by the
// current user in one call (upstream PUT /albums/assets).
func (s *Server) AddAssetsToAlbums(ctx context.Context, request *immichv1.AddAssetsToAlbumsRequest) (*immichv1.AlbumsAddAssetsResponseDto, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := pgutil.StringToUUID(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	if len(request.GetAlbumIds()) == 0 || len(request.GetAssetIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "albumIds and assetIds must not be empty")
	}

	failure := func(reason string) (*immichv1.AlbumsAddAssetsResponseDto, error) {
		return &immichv1.AlbumsAddAssetsResponseDto{
			Success: false,
			Error:   ptr.To(reason),
		}, nil
	}

	albumUUIDs := make([]pgtype.UUID, 0, len(request.GetAlbumIds()))
	for _, albumID := range request.GetAlbumIds() {
		albumUUID, err := pgutil.StringToUUID(albumID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
		}

		// Only album owners may modify multiple albums in bulk.
		_, _, err = s.authorizedAlbum(ctx, albumUUID, albumAccessManage)
		if err != nil {
			return failure("no_permission")
		}
		albumUUIDs = append(albumUUIDs, albumUUID)
	}

	assetUUIDs := make([]pgtype.UUID, 0, len(request.GetAssetIds()))
	for _, assetID := range request.GetAssetIds() {
		assetUUID, err := pgutil.StringToUUID(assetID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
		}
		if _, err := s.db.GetAssetByIDAndUser(ctx, sqlc.GetAssetByIDAndUserParams{
			ID:      assetUUID,
			OwnerId: userID,
		}); err != nil {
			return failure("not_found")
		}
		assetUUIDs = append(assetUUIDs, assetUUID)
	}

	for _, albumUUID := range albumUUIDs {
		for _, assetUUID := range assetUUIDs {
			if err := s.db.AddAssetToAlbum(ctx, sqlc.AddAssetToAlbumParams{
				AlbumsId: albumUUID,
				AssetsId: assetUUID,
			}); err != nil {
				return failure("unknown")
			}
		}
	}

	return &immichv1.AlbumsAddAssetsResponseDto{Success: true}, nil
}
