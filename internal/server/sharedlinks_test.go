package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func TestRedactSharedLinkAsset(t *testing.T) {
	asset := &immichv1.Asset{
		Id:               "asset-id",
		OriginalFileName: "IMG_20260331_120000.jpg",
		OriginalPath:     "/uploads/users/u/IMG_20260331_120000.jpg",
		ExifInfo:         &immichv1.ExifInfo{Make: proto.String("Canon")},
	}

	redacted := redactSharedLinkAsset(asset)

	assert.Equal(t, "asset-id", redacted.Id)
	assert.Empty(t, redacted.OriginalFileName)
	assert.Empty(t, redacted.OriginalPath)
	assert.Nil(t, redacted.ExifInfo)
}

func TestRedactSharedLinkAsset_DoesNotAlterOtherFields(t *testing.T) {
	asset := &immichv1.Asset{
		Id:               "asset-id",
		OriginalFileName: "photo.jpg",
		OriginalPath:     "/path/photo.jpg",
		DeviceAssetId:    "device-123",
		DeviceId:         "device-id",
		IsFavorite:       true,
	}

	redacted := redactSharedLinkAsset(asset)

	assert.Equal(t, "asset-id", redacted.Id)
	assert.Equal(t, "device-123", redacted.DeviceAssetId)
	assert.Equal(t, "device-id", redacted.DeviceId)
	assert.True(t, redacted.IsFavorite)
	assert.Empty(t, redacted.OriginalFileName)
	assert.Empty(t, redacted.OriginalPath)
}
