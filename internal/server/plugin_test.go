package server

import (
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/plugin"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginTypeToProto(t *testing.T) {
	tests := []struct {
		name string
		in   plugin.PluginType
		want immichv1.PluginType
	}{
		{"storage", plugin.PluginTypeStorage, immichv1.PluginType_PLUGIN_TYPE_STORAGE},
		{"processor", plugin.PluginTypeProcessor, immichv1.PluginType_PLUGIN_TYPE_PROCESSOR},
		{"ml", plugin.PluginTypeML, immichv1.PluginType_PLUGIN_TYPE_ML},
		{"notification", plugin.PluginTypeNotification, immichv1.PluginType_PLUGIN_TYPE_NOTIFICATION},
		{"auth", plugin.PluginTypeAuth, immichv1.PluginType_PLUGIN_TYPE_AUTH},
		{"integration", plugin.PluginTypeIntegration, immichv1.PluginType_PLUGIN_TYPE_INTEGRATION},
		{"unknown", plugin.PluginType("unknown"), immichv1.PluginType_PLUGIN_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pluginTypeToProto(tt.in))
		})
	}
}

func TestPluginStatusToProto(t *testing.T) {
	tests := []struct {
		name string
		in   plugin.PluginStatus
		want immichv1.PluginStatus
	}{
		{"active", plugin.PluginStatusActive, immichv1.PluginStatus_PLUGIN_STATUS_ACTIVE},
		{"disabled", plugin.PluginStatusDisabled, immichv1.PluginStatus_PLUGIN_STATUS_DISABLED},
		{"error", plugin.PluginStatusError, immichv1.PluginStatus_PLUGIN_STATUS_ERROR},
		{"updating", plugin.PluginStatusUpdating, immichv1.PluginStatus_PLUGIN_STATUS_UPDATING},
		{"unknown", plugin.PluginStatus("unknown"), immichv1.PluginStatus_PLUGIN_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pluginStatusToProto(tt.in))
		})
	}
}

func TestPluginToProto(t *testing.T) {
	installedAt := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	updatedAt := installedAt.Add(time.Hour)
	p := &plugin.PluginInfo{
		ID:            "plugin-1",
		Name:          "Example",
		Description:   "Example plugin",
		Version:       "1.0.0",
		Author:        "Immich",
		Type:          plugin.PluginTypeIntegration,
		Status:        plugin.PluginStatusError,
		Enabled:       true,
		InstalledAt:   installedAt,
		UpdatedAt:     updatedAt,
		ErrorMessage:  "failed",
		HomepageURL:   "https://example.com",
		RepositoryURL: "https://example.com/repo",
	}

	got := pluginToProto(p)

	require.NotNil(t, got)
	assert.Equal(t, "plugin-1", got.Id)
	assert.Equal(t, "Example", got.Name)
	assert.Equal(t, "Example plugin", got.Description)
	assert.Equal(t, "1.0.0", got.Version)
	assert.Equal(t, "Immich", got.Author)
	assert.Equal(t, immichv1.PluginType_PLUGIN_TYPE_INTEGRATION, got.Type)
	assert.Equal(t, immichv1.PluginStatus_PLUGIN_STATUS_ERROR, got.Status)
	assert.True(t, got.Enabled)
	assert.Equal(t, installedAt, got.InstalledAt.AsTime())
	assert.Equal(t, updatedAt, got.UpdatedAt.AsTime())
	require.NotNil(t, got.ErrorMessage)
	assert.Equal(t, "failed", *got.ErrorMessage)
	require.NotNil(t, got.HomepageUrl)
	assert.Equal(t, "https://example.com", *got.HomepageUrl)
	require.NotNil(t, got.RepositoryUrl)
	assert.Equal(t, "https://example.com/repo", *got.RepositoryUrl)
}

func TestPluginToProtoOmitsEmptyOptionals(t *testing.T) {
	got := pluginToProto(&plugin.PluginInfo{})

	assert.Nil(t, got.ErrorMessage)
	assert.Nil(t, got.HomepageUrl)
	assert.Nil(t, got.RepositoryUrl)
}
