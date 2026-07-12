package server

import (
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/assets"
	"github.com/stretchr/testify/assert"
)

func TestAssetDownloadContentType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"jpeg", "photo.jpg", "image/jpeg"},
		{"jpeg long extension", "photo.jpeg", "image/jpeg"},
		{"png", "photo.png", "image/png"},
		{"gif", "animation.gif", "image/gif"},
		{"webp", "photo.webp", "image/webp"},
		{"mp4", "video.mp4", "video/mp4"},
		{"webm", "video.webm", "video/webm"},
		{"mov", "video.mov", "video/quicktime"},
		{"avi", "video.avi", "video/x-msvideo"},
		{"pdf", "document.pdf", "application/pdf"},
		{"uppercase", "PHOTO.JPG", "image/jpeg"},
		{"unknown", "archive.zip", "application/octet-stream"},
		{"no extension", "README", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, assetDownloadContentType(tt.filename))
		})
	}
}

func TestGetThumbnailContentType(t *testing.T) {
	tests := []struct {
		name          string
		thumbnailType assets.ThumbnailType
		want          string
	}{
		{"preview", assets.ThumbnailTypePreview, "image/jpeg"},
		{"webp fallback", assets.ThumbnailTypeWebp, "image/jpeg"},
		{"thumb", assets.ThumbnailTypeThumb, "image/jpeg"},
		{"unknown", assets.ThumbnailType("unknown"), "image/jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, (&Server{}).getThumbnailContentType(tt.thumbnailType))
		})
	}
}

func TestGetVideoContentType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"mp4", "clip.mp4", "video/mp4"},
		{"webm", "clip.webm", "video/webm"},
		{"avi", "clip.avi", "video/x-msvideo"},
		{"mov", "clip.mov", "video/quicktime"},
		{"mkv", "clip.mkv", "video/x-matroska"},
		{"flv", "clip.flv", "video/x-flv"},
		{"wmv", "clip.wmv", "video/x-ms-wmv"},
		{"m4v", "clip.m4v", "video/x-m4v"},
		{"uppercase", "CLIP.MP4", "video/mp4"},
		{"unknown", "clip.unknown", "video/mp4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, (&Server{}).getVideoContentType(tt.filename))
		})
	}
}
