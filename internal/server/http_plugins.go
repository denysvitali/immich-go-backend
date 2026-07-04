package server

import (
	"net/http"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

type pluginTriggerDTO struct {
	ContextType string `json:"contextType"`
	Type        string `json:"type"`
}

var pluginTriggers = []pluginTriggerDTO{
	{ContextType: "asset", Type: "AssetCreate"},
	{ContextType: "person", Type: "PersonRecognized"},
}

func (s *Server) handlePluginTriggers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}

	writeJSON(w, http.StatusOK, pluginTriggers)
}

func (s *Server) handlePluginMethods(w http.ResponseWriter, r *http.Request) {
	ctx, ok := s.frontendGatewayContext(w, r)
	if !ok {
		return
	}

	resp, err := s.SearchPluginMethods(ctx, &immichv1.SearchPluginMethodsRequest{
		Description:   optionalStringQuery(r, "description"),
		Enabled:       optionalBoolQuery(r, "enabled"),
		Id:            optionalStringQuery(r, "id"),
		Name:          optionalStringQuery(r, "name"),
		PluginName:    optionalStringQuery(r, "pluginName"),
		PluginVersion: optionalStringQuery(r, "pluginVersion"),
		Title:         optionalStringQuery(r, "title"),
		Trigger:       optionalStringQuery(r, "trigger"),
		Type:          optionalStringQuery(r, "type"),
	})
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}

	writeProtoJSONArray(w, frontendProtoMarshaler(), resp.Methods)
}

func (s *Server) handlePluginTemplates(w http.ResponseWriter, r *http.Request) {
	ctx, ok := s.frontendGatewayContext(w, r)
	if !ok {
		return
	}

	resp, err := s.SearchPluginTemplates(ctx, &emptypb.Empty{})
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}

	writeProtoJSONArray(w, frontendProtoMarshaler(), resp.Templates)
}
