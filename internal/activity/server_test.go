package activity

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func authedContext(userID string) context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{UserID: userID})
}

func TestGetActivitiesValidation(t *testing.T) {
	server := NewServer(nil)
	invalidType := immichv1.ReactionType(99)
	invalidLevel := immichv1.ReactionLevel(99)
	badAssetID := "not-a-uuid"
	badUserID := "not-a-uuid"

	tests := []struct {
		name     string
		ctx      context.Context
		request  *immichv1.GetActivitiesRequest
		wantCode codes.Code
	}{
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			request:  &immichv1.GetActivitiesRequest{AlbumId: uuid.NewString()},
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "missing album id",
			ctx:      authedContext(uuid.NewString()),
			request:  &immichv1.GetActivitiesRequest{},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "invalid album id",
			ctx:      authedContext(uuid.NewString()),
			request:  &immichv1.GetActivitiesRequest{AlbumId: "not-a-uuid"},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid asset id",
			ctx:  authedContext(uuid.NewString()),
			request: &immichv1.GetActivitiesRequest{
				AlbumId: uuid.NewString(), AssetId: &badAssetID,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid user id",
			ctx:  authedContext(uuid.NewString()),
			request: &immichv1.GetActivitiesRequest{
				AlbumId: uuid.NewString(), UserId: &badUserID,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid activity type",
			ctx:  authedContext(uuid.NewString()),
			request: &immichv1.GetActivitiesRequest{
				AlbumId: uuid.NewString(), Type: &invalidType,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid activity level",
			ctx:  authedContext(uuid.NewString()),
			request: &immichv1.GetActivitiesRequest{
				AlbumId: uuid.NewString(), Level: &invalidLevel,
			},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.GetActivities(tt.ctx, tt.request)
			assert.Nil(t, resp)
			assert.Equal(t, tt.wantCode, status.Code(err))
		})
	}
}

func TestCreateActivityValidation(t *testing.T) {
	server := NewServer(nil)
	badAssetID := "not-a-uuid"
	commentType := immichv1.ReactionType_REACTION_TYPE_COMMENT

	tests := []struct {
		name     string
		ctx      context.Context
		request  *immichv1.CreateActivityRequest
		wantCode codes.Code
	}{
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			request:  &immichv1.CreateActivityRequest{AlbumId: uuid.NewString()},
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "invalid user id in claims",
			ctx:      authedContext("not-a-uuid"),
			request:  &immichv1.CreateActivityRequest{AlbumId: uuid.NewString()},
			wantCode: codes.Internal,
		},
		{
			name:     "missing album id",
			ctx:      authedContext(uuid.NewString()),
			request:  &immichv1.CreateActivityRequest{},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "invalid album id",
			ctx:      authedContext(uuid.NewString()),
			request:  &immichv1.CreateActivityRequest{AlbumId: "not-a-uuid"},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid asset id",
			ctx:  authedContext(uuid.NewString()),
			request: &immichv1.CreateActivityRequest{
				AlbumId: uuid.NewString(),
				AssetId: &badAssetID,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing comment for comment activity",
			ctx:  authedContext(uuid.NewString()),
			request: &immichv1.CreateActivityRequest{
				AlbumId: uuid.NewString(),
				Type:    commentType,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid activity type",
			ctx:  authedContext(uuid.NewString()),
			request: &immichv1.CreateActivityRequest{
				AlbumId: uuid.NewString(),
				Comment: "comment",
				Type:    immichv1.ReactionType_REACTION_TYPE_UNSPECIFIED,
			},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.CreateActivity(tt.ctx, tt.request)
			assert.Nil(t, resp)
			assert.Equal(t, tt.wantCode, status.Code(err))
		})
	}
}

func TestGetActivityStatisticsValidation(t *testing.T) {
	server := NewServer(nil)
	badAssetID := "not-a-uuid"

	tests := []struct {
		name     string
		ctx      context.Context
		request  *immichv1.GetActivityStatisticsRequest
		wantCode codes.Code
	}{
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			request:  &immichv1.GetActivityStatisticsRequest{AlbumId: uuid.NewString()},
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "missing album id",
			ctx:      authedContext(uuid.NewString()),
			request:  &immichv1.GetActivityStatisticsRequest{},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "invalid album id",
			ctx:      authedContext(uuid.NewString()),
			request:  &immichv1.GetActivityStatisticsRequest{AlbumId: "definitely-not-a-uuid"},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid asset id",
			ctx:  authedContext(uuid.NewString()),
			request: &immichv1.GetActivityStatisticsRequest{
				AlbumId: uuid.NewString(), AssetId: &badAssetID,
			},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.GetActivityStatistics(tt.ctx, tt.request)
			assert.Nil(t, resp)
			assert.Equal(t, tt.wantCode, status.Code(err))
		})
	}
}

func TestDeleteActivityValidation(t *testing.T) {
	server := NewServer(nil)

	tests := []struct {
		name     string
		ctx      context.Context
		request  *immichv1.DeleteActivityRequest
		wantCode codes.Code
	}{
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			request:  &immichv1.DeleteActivityRequest{Id: uuid.NewString()},
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "empty activity id",
			ctx:      authedContext(uuid.NewString()),
			request:  &immichv1.DeleteActivityRequest{},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "invalid activity id",
			ctx:      authedContext(uuid.NewString()),
			request:  &immichv1.DeleteActivityRequest{Id: "not-a-uuid"},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.DeleteActivity(tt.ctx, tt.request)
			assert.Nil(t, resp)
			assert.Equal(t, tt.wantCode, status.Code(err))
		})
	}
}
