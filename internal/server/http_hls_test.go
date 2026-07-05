package server

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHLSPathParsing(t *testing.T) {
	assetID := "00000000-0000-4000-8000-000000000001"
	sessionID := "00000000-0000-4000-8000-000000000002"

	gotAssetID, ok := hlsMainPlaylistAssetIDFromPath("/api/assets/" + assetID + "/video/stream/main.m3u8")
	require.True(t, ok)
	assert.Equal(t, assetID, gotAssetID)

	media, ok := hlsMediaPlaylistFromPath("/api/assets/" + assetID + "/video/stream/" + sessionID + "/0/playlist.m3u8")
	require.True(t, ok)
	assert.Equal(t, assetID, media.assetID)
	assert.Equal(t, sessionID, media.sessionID)
	assert.Equal(t, uint64(0), media.variantIndex)

	segment, ok := hlsSegmentFromPath("/api/assets/" + assetID + "/video/stream/" + sessionID + "/0/seg_12.m4s")
	require.True(t, ok)
	assert.Equal(t, "seg_12.m4s", segment.filename)

	deleteAssetID, deleteSessionID, ok := hlsSessionFromPath("/api/assets/" + assetID + "/video/stream/" + sessionID)
	require.True(t, ok)
	assert.Equal(t, assetID, deleteAssetID)
	assert.Equal(t, sessionID, deleteSessionID)
}

func TestHLSPathParsingRejectsOtherAssetRoutes(t *testing.T) {
	for _, path := range []string{
		"/api/assets",
		"/api/assets/statistics",
		"/api/assets/asset-id/video/playback",
		"/api/assets/asset-id/video/stream/session/0/../secret",
		"/api/albums/album-id",
	} {
		_, mainOK := hlsMainPlaylistAssetIDFromPath(path)
		_, sessionID, sessionOK := hlsSessionFromPath(path)
		_, mediaOK := hlsMediaPlaylistFromPath(path)
		_, segmentOK := hlsSegmentFromPath(path)

		assert.False(t, mainOK, path)
		assert.False(t, sessionOK, path)
		assert.Empty(t, sessionID, path)
		assert.False(t, mediaOK, path)
		assert.False(t, segmentOK, path)
	}
}

func TestBuildHLSMainPlaylist(t *testing.T) {
	assetID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	sessionID := hlsSessionID(assetID)

	playlist := buildHLSMainPlaylist(sessionID, hlsMetadata{Width: 640, Height: 480, Bandwidth: 123456})

	assert.True(t, strings.HasPrefix(playlist, "#EXTM3U\n"))
	assert.Contains(t, playlist, "#EXT-X-VERSION:7")
	assert.Contains(t, playlist, "BANDWIDTH=123456")
	assert.Contains(t, playlist, "RESOLUTION=640x480")
	assert.Contains(t, playlist, sessionID+"/0/playlist.m3u8")
	assert.NoError(t, validateHLSSession(assetID, sessionID))
	assert.Error(t, validateHLSSession(uuid.New(), sessionID))
}

func TestHLSDisplaySize(t *testing.T) {
	for _, tt := range []struct {
		name       string
		width      int
		height     int
		wantWidth  int
		wantHeight int
	}{
		{name: "native", width: 640, height: 480, wantWidth: 640, wantHeight: 480},
		{name: "max landscape", width: 1920, height: 1080, wantWidth: 1920, wantHeight: 1080},
		{name: "wide", width: 3840, height: 2160, wantWidth: 1920, wantHeight: 1080},
		{name: "portrait", width: 2160, height: 3840, wantWidth: 607, wantHeight: 1080},
		{name: "invalid", width: 0, height: 1080, wantWidth: 1920, wantHeight: 1080},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotWidth, gotHeight := hlsDisplaySize(tt.width, tt.height)
			assert.Equal(t, tt.wantWidth, gotWidth)
			assert.Equal(t, tt.wantHeight, gotHeight)
		})
	}
}
