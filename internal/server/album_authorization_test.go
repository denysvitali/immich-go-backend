package server

import (
	"context"
	"testing"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSharedAlbumAccessRoles(t *testing.T) {
	assert.True(t, sharedAlbumAccessAllowed("viewer", albumAccessView))
	assert.True(t, sharedAlbumAccessAllowed("editor", albumAccessView))
	assert.False(t, sharedAlbumAccessAllowed("viewer", albumAccessEdit))
	assert.True(t, sharedAlbumAccessAllowed("editor", albumAccessEdit))
	assert.False(t, sharedAlbumAccessAllowed("editor", albumAccessManage))
}

func TestAlbumOperationsRequireAuthentication(t *testing.T) {
	albumID := uuid.NewString()
	tests := []struct {
		name string
		call func() error
	}{
		{name: "get", call: func() error {
			_, err := (&Server{}).GetAlbumInfo(context.Background(), &immichv1.GetAlbumInfoRequest{Id: albumID})
			return err
		}},
		{name: "update", call: func() error {
			_, err := (&Server{}).UpdateAlbumInfo(context.Background(), &immichv1.UpdateAlbumInfoRequest{Id: albumID})
			return err
		}},
		{name: "delete", call: func() error {
			_, err := (&Server{}).DeleteAlbum(context.Background(), &immichv1.DeleteAlbumRequest{Id: albumID})
			return err
		}},
		{name: "add assets", call: func() error {
			_, err := (&Server{}).AddAssetsToAlbum(context.Background(), &immichv1.AddAssetsToAlbumRequest{Id: albumID})
			return err
		}},
		{name: "remove assets", call: func() error {
			_, err := (&Server{}).RemoveAssetFromAlbum(context.Background(), &immichv1.RemoveAssetFromAlbumRequest{Id: albumID})
			return err
		}},
		{name: "add users", call: func() error {
			_, err := (&Server{}).AddUsersToAlbum(context.Background(), &immichv1.AddUsersToAlbumRequest{Id: albumID})
			return err
		}},
		{name: "remove user", call: func() error {
			_, err := (&Server{}).RemoveUserFromAlbum(context.Background(), &immichv1.RemoveUserFromAlbumRequest{Id: albumID})
			return err
		}},
		{name: "update user", call: func() error {
			_, err := (&Server{}).UpdateAlbumUser(context.Background(), &immichv1.UpdateAlbumUserRequest{Id: albumID})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, codes.Unauthenticated, status.Code(tt.call()))
		})
	}
}
