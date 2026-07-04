package server

import "net/http"

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
