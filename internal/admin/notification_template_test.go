package admin

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/config"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTemplateTestService(t *testing.T) *Service {
	t.Helper()

	service, err := NewService(nil, &config.Config{}, nil)
	require.NoError(t, err)
	return service
}

func TestRenderNotificationTemplateUsesCustomTemplateData(t *testing.T) {
	service := newTemplateTestService(t)

	resp, err := service.RenderNotificationTemplate(
		context.Background(),
		"welcome",
		"Hello {displayName}, log in at {baseUrl}. Unknown {missing}",
	)
	require.NoError(t, err)

	assert.Equal(t, "welcome", resp.Name)
	assert.Contains(t, resp.HTML, "Hello John Doe, log in at https://demo.immich.app.")
	assert.Contains(t, resp.HTML, "Unknown {missing}")
}

func TestRenderNotificationTemplateEscapesCustomTemplate(t *testing.T) {
	service := newTemplateTestService(t)

	resp, err := service.RenderNotificationTemplate(
		context.Background(),
		"album-invite",
		"Album {albumName}<script>alert(1)</script>",
	)
	require.NoError(t, err)

	assert.Equal(t, "album-invite", resp.Name)
	assert.Contains(t, resp.HTML, "Album John Doe&#39;s Favorites")
	assert.Contains(t, resp.HTML, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.NotContains(t, resp.HTML, "<script>")
}

func TestRenderNotificationTemplateUnknownNameMatchesUpstreamEmptyPreview(t *testing.T) {
	service := newTemplateTestService(t)

	resp, err := service.RenderNotificationTemplate(context.Background(), "unknown", "ignored")
	require.NoError(t, err)

	assert.Equal(t, "unknown", resp.Name)
	assert.Empty(t, resp.HTML)
}

func TestRenderNotificationTemplateServerReturnsUpstreamShape(t *testing.T) {
	service := newTemplateTestService(t)
	server := NewServer(service, nil)

	resp, err := server.RenderNotificationTemplate(adminContext(), &immichv1.RenderNotificationTemplateRequest{
		Name:     "album-update",
		Template: "Album {albumName} for {recipientName}",
	})
	require.NoError(t, err)

	assert.Equal(t, "album-update", resp.GetName())
	assert.Contains(t, resp.GetHtml(), "Album Favorite Photos for Jane Doe")
}
