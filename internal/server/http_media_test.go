package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssetMediaRouteFromPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		assetID string
		kind    assetMediaKind
	}{
		{"thumbnail", "/api/assets/abc-123/thumbnail", "abc-123", assetMediaThumbnail},
		{"original", "/api/assets/abc-123/original", "abc-123", assetMediaOriginal},
		{"video playback", "/api/assets/abc-123/video/playback", "abc-123", assetMediaVideoPlayback},
		{"hls main playlist", "/api/assets/abc-123/video/stream/main.m3u8", "", assetMediaNone},
		{"asset by id", "/api/assets/abc-123", "", assetMediaNone},
		{"assets root", "/api/assets", "", assetMediaNone},
		{"statistics", "/api/assets/statistics", "", assetMediaNone},
		{"device assets", "/api/assets/device/some-device", "", assetMediaNone},
		{"missing id", "/api/assets//thumbnail", "", assetMediaNone},
		{"unrelated", "/api/albums/abc-123", "", assetMediaNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assetID, kind := assetMediaRouteFromPath(tt.path)
			assert.Equal(t, tt.kind, kind)
			assert.Equal(t, tt.assetID, assetID)
		})
	}
}
