package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var tracer = telemetry.GetTracer("plugin")

// PluginStatus represents the status of a plugin
type PluginStatus string

const (
	PluginStatusActive   PluginStatus = "active"
	PluginStatusDisabled PluginStatus = "disabled"
	PluginStatusError    PluginStatus = "error"
	PluginStatusUpdating PluginStatus = "updating"
)

// PluginType represents the type of plugin
type PluginType string

const (
	PluginTypeStorage      PluginType = "storage"
	PluginTypeProcessor    PluginType = "processor"
	PluginTypeML           PluginType = "ml"
	PluginTypeNotification PluginType = "notification"
	PluginTypeAuth         PluginType = "auth"
	PluginTypeIntegration  PluginType = "integration"
)

// PluginInfo contains information about a plugin
type PluginInfo struct {
	ID            string
	Name          string
	Description   string
	Version       string
	Author        string
	Type          PluginType
	Status        PluginStatus
	Enabled       bool
	InstalledAt   time.Time
	UpdatedAt     time.Time
	ErrorMessage  string
	HomepageURL   string
	RepositoryURL string
	Config        map[string]interface{}
	ConfigSchema  map[string]interface{}
}

// Service handles plugin management operations
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// In-memory plugin registry (in production, would use database)
	mu      sync.RWMutex
	plugins map[string]*PluginInfo

	// Metrics
	pluginCounter     metric.Int64UpDownCounter
	operationCounter  metric.Int64Counter
	operationDuration metric.Float64Histogram
}

// NewService creates a new plugin management service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	meter := telemetry.GetMeter()

	pluginCounter, err := meter.Int64UpDownCounter(
		"plugins_total",
		metric.WithDescription("Total number of installed plugins"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin counter: %w", err)
	}

	operationCounter, err := meter.Int64Counter(
		"plugin_operations_total",
		metric.WithDescription("Total number of plugin operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation counter: %w", err)
	}

	operationDuration, err := meter.Float64Histogram(
		"plugin_operation_duration_seconds",
		metric.WithDescription("Time spent on plugin operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation duration histogram: %w", err)
	}

	s := &Service{
		db:                queries,
		config:            cfg,
		plugins:           make(map[string]*PluginInfo),
		pluginCounter:     pluginCounter,
		operationCounter:  operationCounter,
		operationDuration: operationDuration,
	}

	// Initialize with built-in plugins
	s.initializeBuiltinPlugins()

	return s, nil
}

// initializeBuiltinPlugins registers built-in plugins
func (s *Service) initializeBuiltinPlugins() {
	now := time.Now()

	// Built-in storage plugins
	s.plugins["local-storage"] = &PluginInfo{
		ID:          "local-storage",
		Name:        "Local Storage",
		Description: "Store assets on local filesystem",
		Version:     "1.0.0",
		Author:      "Immich",
		Type:        PluginTypeStorage,
		Status:      PluginStatusActive,
		Enabled:     true,
		InstalledAt: now,
		UpdatedAt:   now,
		Config:      map[string]interface{}{"path": "/data/immich"},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Base path for storage",
				},
			},
		},
	}

	s.plugins["s3-storage"] = &PluginInfo{
		ID:          "s3-storage",
		Name:        "S3 Storage",
		Description: "Store assets in S3-compatible object storage",
		Version:     "1.0.0",
		Author:      "Immich",
		Type:        PluginTypeStorage,
		Status:      PluginStatusActive,
		Enabled:     true,
		InstalledAt: now,
		UpdatedAt:   now,
		Config: map[string]interface{}{
			"bucket":   "immich",
			"region":   "us-east-1",
			"endpoint": "",
		},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"bucket":     map[string]interface{}{"type": "string"},
				"region":     map[string]interface{}{"type": "string"},
				"endpoint":   map[string]interface{}{"type": "string"},
				"accessKey":  map[string]interface{}{"type": "string"},
				"secretKey":  map[string]interface{}{"type": "string", "format": "password"},
			},
			"required": []string{"bucket", "region"},
		},
	}

	// Built-in ML plugins
	s.plugins["face-detection"] = &PluginInfo{
		ID:          "face-detection",
		Name:        "Face Detection",
		Description: "Detect and recognize faces in photos",
		Version:     "1.0.0",
		Author:      "Immich",
		Type:        PluginTypeML,
		Status:      PluginStatusActive,
		Enabled:     true,
		InstalledAt: now,
		UpdatedAt:   now,
		Config: map[string]interface{}{
			"minConfidence": 0.7,
			"modelSize":     "small",
		},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"minConfidence": map[string]interface{}{
					"type":    "number",
					"minimum": 0,
					"maximum": 1,
				},
				"modelSize": map[string]interface{}{
					"type": "string",
					"enum": []string{"small", "medium", "large"},
				},
			},
		},
	}

	s.plugins["smart-search"] = &PluginInfo{
		ID:          "smart-search",
		Name:        "Smart Search",
		Description: "AI-powered semantic search for photos",
		Version:     "1.0.0",
		Author:      "Immich",
		Type:        PluginTypeML,
		Status:      PluginStatusActive,
		Enabled:     true,
		InstalledAt: now,
		UpdatedAt:   now,
		Config: map[string]interface{}{
			"model": "clip",
		},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"model": map[string]interface{}{
					"type": "string",
					"enum": []string{"clip", "blip"},
				},
			},
		},
	}

	// Thumbnail processor plugin
	s.plugins["thumbnail-processor"] = &PluginInfo{
		ID:          "thumbnail-processor",
		Name:        "Thumbnail Processor",
		Description: "Generate thumbnails for assets",
		Version:     "1.0.0",
		Author:      "Immich",
		Type:        PluginTypeProcessor,
		Status:      PluginStatusActive,
		Enabled:     true,
		InstalledAt: now,
		UpdatedAt:   now,
		Config: map[string]interface{}{
			"quality":         80,
			"thumbnailSize":   250,
			"previewSize":     1440,
			"colorspace":      "srgb",
			"preferredFormat": "webp",
		},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"quality": map[string]interface{}{
					"type":    "integer",
					"minimum": 1,
					"maximum": 100,
				},
				"thumbnailSize": map[string]interface{}{"type": "integer"},
				"previewSize":   map[string]interface{}{"type": "integer"},
				"colorspace":    map[string]interface{}{"type": "string"},
				"preferredFormat": map[string]interface{}{
					"type": "string",
					"enum": []string{"jpeg", "webp"},
				},
			},
		},
	}
}

// ListPlugins returns all installed plugins
func (s *Service) ListPlugins(ctx context.Context) ([]*PluginInfo, error) {
	ctx, span := tracer.Start(ctx, "plugin.list_plugins")
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "list_plugins")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "list_plugins")))
	}()

	s.mu.RLock()
	defer s.mu.RUnlock()

	plugins := make([]*PluginInfo, 0, len(s.plugins))
	for _, plugin := range s.plugins {
		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

// GetPlugin returns a specific plugin by ID
func (s *Service) GetPlugin(ctx context.Context, pluginID string) (*PluginInfo, error) {
	ctx, span := tracer.Start(ctx, "plugin.get_plugin",
		trace.WithAttributes(attribute.String("plugin_id", pluginID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_plugin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_plugin")))
	}()

	s.mu.RLock()
	defer s.mu.RUnlock()

	plugin, exists := s.plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	return plugin, nil
}

// InstallPlugin installs a new plugin
func (s *Service) InstallPlugin(ctx context.Context, source string, version *string) (*PluginInfo, error) {
	ctx, span := tracer.Start(ctx, "plugin.install_plugin",
		trace.WithAttributes(attribute.String("source", source)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "install_plugin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "install_plugin")))
	}()

	// In a real implementation, this would:
	// 1. Fetch plugin from source (git repo, npm, etc.)
	// 2. Validate plugin manifest
	// 3. Install dependencies
	// 4. Register plugin hooks
	// 5. Store plugin metadata in database

	s.mu.Lock()
	defer s.mu.Unlock()

	pluginID := uuid.New().String()
	now := time.Now()

	ver := "1.0.0"
	if version != nil {
		ver = *version
	}

	plugin := &PluginInfo{
		ID:          pluginID,
		Name:        fmt.Sprintf("Plugin from %s", source),
		Description: "User-installed plugin",
		Version:     ver,
		Author:      "External",
		Type:        PluginTypeIntegration,
		Status:      PluginStatusActive,
		Enabled:     true,
		InstalledAt: now,
		UpdatedAt:   now,
		Config:      make(map[string]interface{}),
		ConfigSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}

	s.plugins[pluginID] = plugin
	s.pluginCounter.Add(ctx, 1)

	return plugin, nil
}

// UninstallPlugin removes a plugin
func (s *Service) UninstallPlugin(ctx context.Context, pluginID string, removeData bool) error {
	ctx, span := tracer.Start(ctx, "plugin.uninstall_plugin",
		trace.WithAttributes(
			attribute.String("plugin_id", pluginID),
			attribute.Bool("remove_data", removeData),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "uninstall_plugin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "uninstall_plugin")))
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	plugin, exists := s.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	// Don't allow uninstalling built-in plugins
	builtinPlugins := map[string]bool{
		"local-storage":       true,
		"s3-storage":          true,
		"face-detection":      true,
		"smart-search":        true,
		"thumbnail-processor": true,
	}
	if builtinPlugins[pluginID] {
		return fmt.Errorf("cannot uninstall built-in plugin: %s", plugin.Name)
	}

	delete(s.plugins, pluginID)
	s.pluginCounter.Add(ctx, -1)

	return nil
}

// GetPluginConfig returns the configuration for a plugin
func (s *Service) GetPluginConfig(ctx context.Context, pluginID string) (map[string]interface{}, map[string]interface{}, error) {
	ctx, span := tracer.Start(ctx, "plugin.get_plugin_config",
		trace.WithAttributes(attribute.String("plugin_id", pluginID)))
	defer span.End()

	s.mu.RLock()
	defer s.mu.RUnlock()

	plugin, exists := s.plugins[pluginID]
	if !exists {
		return nil, nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	return plugin.Config, plugin.ConfigSchema, nil
}

// UpdatePluginConfig updates the configuration for a plugin
func (s *Service) UpdatePluginConfig(ctx context.Context, pluginID string, config map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := tracer.Start(ctx, "plugin.update_plugin_config",
		trace.WithAttributes(attribute.String("plugin_id", pluginID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "update_plugin_config")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "update_plugin_config")))
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	plugin, exists := s.plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	// In a real implementation, validate config against schema
	plugin.Config = config
	plugin.UpdatedAt = time.Now()

	return plugin.Config, nil
}

// SetPluginEnabled enables or disables a plugin
func (s *Service) SetPluginEnabled(ctx context.Context, pluginID string, enabled bool) (*PluginInfo, error) {
	ctx, span := tracer.Start(ctx, "plugin.set_plugin_enabled",
		trace.WithAttributes(
			attribute.String("plugin_id", pluginID),
			attribute.Bool("enabled", enabled),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "set_plugin_enabled")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "set_plugin_enabled")))
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	plugin, exists := s.plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	plugin.Enabled = enabled
	plugin.UpdatedAt = time.Now()

	if enabled {
		plugin.Status = PluginStatusActive
	} else {
		plugin.Status = PluginStatusDisabled
	}

	return plugin, nil
}
