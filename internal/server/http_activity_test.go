package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestActivitySearchRequestUsesUpstreamQueryValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/activities?albumId=album&assetId=asset&userId=user&level=asset&type=like", nil)
	got, err := activitySearchRequest(req)

	assert.NoError(t, err)
	assert.Equal(t, "album", got.AlbumId)
	assert.Equal(t, "asset", got.GetAssetId())
	assert.Equal(t, "user", got.GetUserId())
	assert.Equal(t, immichv1.ReactionLevel_REACTION_LEVEL_ASSET, got.GetLevel())
	assert.Equal(t, immichv1.ReactionType_REACTION_TYPE_LIKE, got.GetType())
}

func TestActivitySearchRequestRejectsInvalidEnums(t *testing.T) {
	for _, rawQuery := range []string{"albumId=album&type=invalid", "albumId=album&level=invalid"} {
		t.Run(rawQuery, func(t *testing.T) {
			_, err := activitySearchRequest(httptest.NewRequest(http.MethodGet, "/api/activities?"+rawQuery, nil))
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestActivityHTTPResponseUsesUpstreamShape(t *testing.T) {
	assetID := "asset-id"
	response := activityHTTPResponseFromProto(&immichv1.ActivityResponseDto{
		Id:        "activity-id",
		AssetId:   assetID,
		Comment:   "A comment",
		CreatedAt: timestamppb.New(time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)),
		Type:      immichv1.ReactionType_REACTION_TYPE_LIKE,
		User:      &immichv1.User{Id: "user-id", Email: "user@example.com", Name: "User"},
	})

	body, err := json.Marshal(response)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"assetId":"asset-id","comment":"A comment","createdAt":"2026-07-09T12:00:00Z","id":"activity-id","type":"like","user":{"id":"user-id","email":"user@example.com","name":"User"}}`, string(body))
}

func TestActivityHTTPResponseUsesNullForLikeOptionalFields(t *testing.T) {
	response := activityHTTPResponseFromProto(&immichv1.ActivityResponseDto{
		Id:        "activity-id",
		CreatedAt: timestamppb.New(time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)),
		Type:      immichv1.ReactionType_REACTION_TYPE_LIKE,
		User:      &immichv1.User{Id: "user-id"},
	})

	body, err := json.Marshal(response)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"assetId":null,"comment":null,"createdAt":"2026-07-09T12:00:00Z","id":"activity-id","type":"like","user":{"id":"user-id","email":"","name":""}}`, string(body))
}

func TestActivityIDFromPath(t *testing.T) {
	assert.Equal(t, "activity-id", mustActivityID(t, "/api/activities/activity-id"))
	for _, path := range []string{"/api/activities", "/api/activities/statistics", "/api/activities/a/b"} {
		_, ok := activityIDFromPath(path)
		assert.False(t, ok, path)
	}
}

func mustActivityID(t *testing.T, path string) string {
	t.Helper()
	id, ok := activityIDFromPath(path)
	if !ok {
		t.Fatalf("activityIDFromPath(%q) did not match", path)
	}
	return id
}
