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
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	plugins, err := s.pluginService.ListPlugins(ctx)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to list plugins", err)
	}

	protoPlugins := make([]*immichv1.PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		protoPlugins = append(protoPlugins, pluginToProto(p))
	}

	return &immichv1.ListPluginsResponse{
		Plugins: protoPlugins,
	}, nil
}

// SearchPluginMethods returns available workflow methods exposed by plugins.
func (s *Server) SearchPluginMethods(ctx context.Context, req *immichv1.SearchPluginMethodsRequest) (*immichv1.SearchPluginMethodsResponse, error) {
	if _, err := s.getUserFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	methods, err := s.pluginService.SearchPluginMethods(ctx, plugin.PluginMethodSearchFilter{
		Description:   req.Description,
		Enabled:       req.Enabled,
		ID:            req.Id,
		Name:          req.Name,
		PluginName:    req.PluginName,
		PluginVersion: req.PluginVersion,
		Title:         req.Title,
		Trigger:       req.Trigger,
		Type:          req.Type,
	})
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to search plugin methods", err)
	}

	protoMethods := make([]*immichv1.PluginMethodResponseDto, 0, len(methods))
	for _, method := range methods {
		protoMethod, err := pluginMethodToProto(method)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to convert plugin method", err)
		}
		protoMethods = append(protoMethods, protoMethod)
	}

	return &immichv1.SearchPluginMethodsResponse{Methods: protoMethods}, nil
}

// SearchPluginTemplates returns workflow templates exposed by plugins.
func (s *Server) SearchPluginTemplates(ctx context.Context, _ *emptypb.Empty) (*immichv1.SearchPluginTemplatesResponse, error) {
	if _, err := s.getUserFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	templates, err := s.pluginService.SearchPluginTemplates(ctx)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to search plugin templates", err)
	}

	protoTemplates := make([]*immichv1.PluginTemplateResponseDto, 0, len(templates))
	for _, template := range templates {
		protoTemplate, err := pluginTemplateToProto(template)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to convert plugin template", err)
		}
		protoTemplates = append(protoTemplates, protoTemplate)
	}

	return &immichv1.SearchPluginTemplatesResponse{Templates: protoTemplates}, nil
}

// GetPlugin returns a specific plugin by ID
func (s *Server) GetPlugin(ctx context.Context, req *immichv1.GetPluginRequest) (*immichv1.PluginInfo, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	p, err := s.pluginService.GetPlugin(ctx, req.PluginId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "plugin not found: %v", err)
	}

	return pluginToProto(p), nil
}

// InstallPlugin installs a new plugin
func (s *Server) InstallPlugin(ctx context.Context, req *immichv1.InstallPluginRequest) (*immichv1.PluginInfo, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	p, err := s.pluginService.InstallPlugin(ctx, req.Source, req.Version)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to install plugin", err)
	}

	return pluginToProto(p), nil
}

// UninstallPlugin removes a plugin
func (s *Server) UninstallPlugin(ctx context.Context, req *immichv1.UninstallPluginRequest) (*emptypb.Empty, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	removeData := false
	if req.RemoveData != nil {
		removeData = *req.RemoveData
	}

	err := s.pluginService.UninstallPlugin(ctx, req.PluginId, removeData)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to uninstall plugin", err)
	}

	return &emptypb.Empty{}, nil
}

// GetPluginConfig returns the configuration for a plugin
func (s *Server) GetPluginConfig(ctx context.Context, req *immichv1.GetPluginConfigRequest) (*immichv1.PluginConfigResponse, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
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
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	config := req.Config.AsMap()
	updatedConfig, err := s.pluginService.UpdatePluginConfig(ctx, req.PluginId, config)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to update plugin config", err)
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
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	p, err := s.pluginService.SetPluginEnabled(ctx, req.PluginId, req.Enabled)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to update plugin", err)
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

func pluginMethodToProto(method plugin.PluginMethodInfo) (*immichv1.PluginMethodResponseDto, error) {
	schema, err := structpb.NewStruct(method.Schema)
	if err != nil {
		return nil, err
	}
	return &immichv1.PluginMethodResponseDto{
		Description:   method.Description,
		HostFunctions: method.HostFunctions,
		Key:           method.PluginName + "#" + method.Name,
		Name:          method.Name,
		Schema:        schema,
		Title:         method.Title,
		Types:         append([]string(nil), method.Types...),
		UiHints:       append([]string(nil), method.UIHints...),
	}, nil
}

func pluginTemplateToProto(template plugin.PluginTemplateInfo) (*immichv1.PluginTemplateResponseDto, error) {
	steps := make([]*immichv1.PluginTemplateStepResponseDto, 0, len(template.Steps))
	for _, step := range template.Steps {
		config, err := structpb.NewStruct(step.Config)
		if err != nil {
			return nil, err
		}
		steps = append(steps, &immichv1.PluginTemplateStepResponseDto{
			Method:  step.Method,
			Config:  config,
			Enabled: step.Enabled,
		})
	}

	return &immichv1.PluginTemplateResponseDto{
		Description: template.Description,
		Key:         template.PluginName + "#" + template.Name,
		Steps:       steps,
		Title:       template.Title,
		Trigger:     template.Trigger,
		UiHints:     append([]string(nil), template.UIHints...),
	}, nil
}

func pluginTypeToProto(t plugin.PluginType) immichv1.PluginType {
	if protoType, ok := pluginTypeProtoValues[t]; ok {
		return protoType
	}
	return immichv1.PluginType_PLUGIN_TYPE_UNSPECIFIED
}

var pluginTypeProtoValues = map[plugin.PluginType]immichv1.PluginType{
	plugin.PluginTypeStorage:      immichv1.PluginType_PLUGIN_TYPE_STORAGE,
	plugin.PluginTypeProcessor:    immichv1.PluginType_PLUGIN_TYPE_PROCESSOR,
	plugin.PluginTypeML:           immichv1.PluginType_PLUGIN_TYPE_ML,
	plugin.PluginTypeNotification: immichv1.PluginType_PLUGIN_TYPE_NOTIFICATION,
	plugin.PluginTypeAuth:         immichv1.PluginType_PLUGIN_TYPE_AUTH,
	plugin.PluginTypeIntegration:  immichv1.PluginType_PLUGIN_TYPE_INTEGRATION,
}

func pluginStatusToProto(s plugin.PluginStatus) immichv1.PluginStatus {
	if protoStatus, ok := pluginStatusProtoValues[s]; ok {
		return protoStatus
	}
	return immichv1.PluginStatus_PLUGIN_STATUS_UNSPECIFIED
}

var pluginStatusProtoValues = map[plugin.PluginStatus]immichv1.PluginStatus{
	plugin.PluginStatusActive:   immichv1.PluginStatus_PLUGIN_STATUS_ACTIVE,
	plugin.PluginStatusDisabled: immichv1.PluginStatus_PLUGIN_STATUS_DISABLED,
	plugin.PluginStatusError:    immichv1.PluginStatus_PLUGIN_STATUS_ERROR,
	plugin.PluginStatusUpdating: immichv1.PluginStatus_PLUGIN_STATUS_UPDATING,
}
