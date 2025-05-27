package server

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) GetAllAlbums(ctx context.Context, request *immichv1.GetAllAlbumsRequest) (*immichv1.GetAllAlbumsResponse, error) {
	albums, err := s.db.GetAlbums(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get albums: %w", err)
	}
	immichAlbums := make([]*immichv1.Album, len(albums))
	for i, album := range albums {
		immichAlbums[i] = &immichv1.Album{
			Id:          album.ID.String(),
			AlbumName:   album.AlbumName,
			Description: album.Description,
			OwnerId:     album.OwnerId.String(),
			// TODO: populate other fields
		}
	}
	return &immichv1.GetAllAlbumsResponse{Albums: immichAlbums}, nil
}

func (s *Server) CreateAlbum(ctx context.Context, request *immichv1.CreateAlbumRequest) (*immichv1.Album, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetAlbumInfo(ctx context.Context, request *immichv1.GetAlbumInfoRequest) (*immichv1.Album, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) UpdateAlbumInfo(ctx context.Context, request *immichv1.UpdateAlbumInfoRequest) (*immichv1.Album, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) DeleteAlbum(ctx context.Context, request *immichv1.DeleteAlbumRequest) (*emptypb.Empty, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) AddAssetsToAlbum(ctx context.Context, request *immichv1.AddAssetsToAlbumRequest) (*immichv1.AddAssetsToAlbumResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) RemoveAssetFromAlbum(ctx context.Context, request *immichv1.RemoveAssetFromAlbumRequest) (*immichv1.RemoveAssetFromAlbumResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) AddUsersToAlbum(ctx context.Context, request *immichv1.AddUsersToAlbumRequest) (*immichv1.Album, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) RemoveUserFromAlbum(ctx context.Context, request *immichv1.RemoveUserFromAlbumRequest) (*emptypb.Empty, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) UpdateAlbumUser(ctx context.Context, request *immichv1.UpdateAlbumUserRequest) (*emptypb.Empty, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetAlbumStatistics(ctx context.Context, request *immichv1.GetAlbumStatisticsRequest) (*immichv1.AlbumStatisticsResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}
