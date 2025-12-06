package server

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/plugin"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ListPlugins returns all installed plugins
func (s *Server) ListPlugins(ctx context.Context, _ *emptypb.Empty) (*immichv1.ListPluginsResponse, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	plugins, err := s.pluginService.ListPlugins(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list plugins: %v", err)
	}

	protoPlugins := make([]*immichv1.PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		protoPlugins = append(protoPlugins, pluginToProto(p))
	}

	return &immichv1.ListPluginsResponse{
		Plugins: protoPlugins,
	}, nil
}

// GetPlugin returns a specific plugin by ID
func (s *Server) GetPlugin(ctx context.Context, req *immichv1.GetPluginRequest) (*immichv1.PluginInfo, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	p, err := s.pluginService.GetPlugin(ctx, req.PluginId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "plugin not found: %v", err)
	}

	return pluginToProto(p), nil
}

// InstallPlugin installs a new plugin
func (s *Server) InstallPlugin(ctx context.Context, req *immichv1.InstallPluginRequest) (*immichv1.PluginInfo, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	p, err := s.pluginService.InstallPlugin(ctx, req.Source, req.Version)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to install plugin: %v", err)
	}

	return pluginToProto(p), nil
}

// UninstallPlugin removes a plugin
func (s *Server) UninstallPlugin(ctx context.Context, req *immichv1.UninstallPluginRequest) (*emptypb.Empty, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	removeData := false
	if req.RemoveData != nil {
		removeData = *req.RemoveData
	}

	err = s.pluginService.UninstallPlugin(ctx, req.PluginId, removeData)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to uninstall plugin: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// GetPluginConfig returns the configuration for a plugin
func (s *Server) GetPluginConfig(ctx context.Context, req *immichv1.GetPluginConfigRequest) (*immichv1.PluginConfigResponse, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	config, schema, err := s.pluginService.GetPluginConfig(ctx, req.PluginId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "plugin not found: %v", err)
	}

	configStruct, _ := structpb.NewStruct(config)
	schemaStruct, _ := structpb.NewStruct(schema)

	return &immichv1.PluginConfigResponse{
		PluginId: req.PluginId,
		Config:   configStruct,
		Schema:   schemaStruct,
	}, nil
}

// UpdatePluginConfig updates the configuration for a plugin
func (s *Server) UpdatePluginConfig(ctx context.Context, req *immichv1.UpdatePluginConfigRequest) (*immichv1.PluginConfigResponse, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	config := req.Config.AsMap()
	updatedConfig, err := s.pluginService.UpdatePluginConfig(ctx, req.PluginId, config)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update plugin config: %v", err)
	}

	configStruct, _ := structpb.NewStruct(updatedConfig)

	// Get schema
	_, schema, _ := s.pluginService.GetPluginConfig(ctx, req.PluginId)
	schemaStruct, _ := structpb.NewStruct(schema)

	return &immichv1.PluginConfigResponse{
		PluginId: req.PluginId,
		Config:   configStruct,
		Schema:   schemaStruct,
	}, nil
}

// SetPluginEnabled enables or disables a plugin
func (s *Server) SetPluginEnabled(ctx context.Context, req *immichv1.SetPluginEnabledRequest) (*immichv1.PluginInfo, error) {
	// Verify user is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	p, err := s.pluginService.SetPluginEnabled(ctx, req.PluginId, req.Enabled)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update plugin: %v", err)
	}

	return pluginToProto(p), nil
}

// Helper function to convert plugin to proto
func pluginToProto(p *plugin.PluginInfo) *immichv1.PluginInfo {
	proto := &immichv1.PluginInfo{
		Id:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Version:     p.Version,
		Author:      p.Author,
		Type:        pluginTypeToProto(p.Type),
		Status:      pluginStatusToProto(p.Status),
		Enabled:     p.Enabled,
		InstalledAt: timestamppb.New(p.InstalledAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}

	if p.ErrorMessage != "" {
		proto.ErrorMessage = &p.ErrorMessage
	}
	if p.HomepageURL != "" {
		proto.HomepageUrl = &p.HomepageURL
	}
	if p.RepositoryURL != "" {
		proto.RepositoryUrl = &p.RepositoryURL
	}

	return proto
}

func pluginTypeToProto(t plugin.PluginType) immichv1.PluginType {
	switch t {
	case plugin.PluginTypeStorage:
		return immichv1.PluginType_PLUGIN_TYPE_STORAGE
	case plugin.PluginTypeProcessor:
		return immichv1.PluginType_PLUGIN_TYPE_PROCESSOR
	case plugin.PluginTypeML:
		return immichv1.PluginType_PLUGIN_TYPE_ML
	case plugin.PluginTypeNotification:
		return immichv1.PluginType_PLUGIN_TYPE_NOTIFICATION
	case plugin.PluginTypeAuth:
		return immichv1.PluginType_PLUGIN_TYPE_AUTH
	case plugin.PluginTypeIntegration:
		return immichv1.PluginType_PLUGIN_TYPE_INTEGRATION
	default:
		return immichv1.PluginType_PLUGIN_TYPE_UNSPECIFIED
	}
}

func pluginStatusToProto(s plugin.PluginStatus) immichv1.PluginStatus {
	switch s {
	case plugin.PluginStatusActive:
		return immichv1.PluginStatus_PLUGIN_STATUS_ACTIVE
	case plugin.PluginStatusDisabled:
		return immichv1.PluginStatus_PLUGIN_STATUS_DISABLED
	case plugin.PluginStatusError:
		return immichv1.PluginStatus_PLUGIN_STATUS_ERROR
	case plugin.PluginStatusUpdating:
		return immichv1.PluginStatus_PLUGIN_STATUS_UPDATING
	default:
		return immichv1.PluginStatus_PLUGIN_STATUS_UNSPECIFIED
	}
}
