package assets

import (
	"testing"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
)

func TestAssetTypeFromString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want immichv1.AssetType
	}{
		{name: "image", in: "IMAGE", want: immichv1.AssetType_ASSET_TYPE_IMAGE},
		{name: "video", in: "VIDEO", want: immichv1.AssetType_ASSET_TYPE_VIDEO},
		{name: "audio", in: "AUDIO", want: immichv1.AssetType_ASSET_TYPE_AUDIO},
		{name: "other", in: "OTHER", want: immichv1.AssetType_ASSET_TYPE_OTHER},
		{name: "unknown", in: "RAW", want: immichv1.AssetType_ASSET_TYPE_OTHER},
		{name: "empty", in: "", want: immichv1.AssetType_ASSET_TYPE_OTHER},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AssetTypeFromString(tt.in))
		})
	}
}
