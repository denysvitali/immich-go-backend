package view

import (
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestConvertAssetToAssetInfoTypeMapping(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want AssetType
	}{
		{name: "image", in: "IMAGE", want: AssetType(immichv1.AssetType_ASSET_TYPE_IMAGE)},
		{name: "video", in: "VIDEO", want: AssetType(immichv1.AssetType_ASSET_TYPE_VIDEO)},
		{name: "audio", in: "AUDIO", want: AssetType(immichv1.AssetType_ASSET_TYPE_AUDIO)},
		{name: "unknown", in: "RAW", want: AssetType(immichv1.AssetType_ASSET_TYPE_OTHER)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := sqlc.Asset{
				ID:            pgutil.UUIDToPgtype(uuid.New()),
				DeviceAssetId: "device-asset-id",
				DeviceId:      "device-id",
				Type:          tt.in,
			}

			got := convertAssetToAssetInfo(&asset)

			assert.Equal(t, tt.want, got.Type)
		})
	}
}
