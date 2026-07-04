package server

import (
	"io"
	"net/http"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/systemconfig"
)

// requireAdminHTTP authenticates the request and rejects non-admin callers.
// Used by the frontend-shape handlers for admin-only upstream routes.
func (s *Server) requireAdminHTTP(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := s.requireAuth(w, r)
	if !ok {
		return nil, false
	}
	if !claims.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"message":    "Forbidden",
			"statusCode": http.StatusForbidden,
		})
		return nil, false
	}
	return claims, true
}

// handleSystemConfigGet serves GET /api/system-config with the exact upstream
// SystemConfigDto shape (the proto DTO is a subset, so the gateway route
// can't be used by the web UI).
func (s *Server) handleSystemConfigGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdminHTTP(w, r); !ok {
		return
	}

	cfg, err := s.systemConfigService.GetConfigDto(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleSystemConfigPut serves PUT /api/system-config.
func (s *Server) handleSystemConfigPut(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdminHTTP(w, r); !ok {
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "failed to read body"})
		return
	}

	cfg, err := s.systemConfigService.UpdateConfigDto(r.Context(), body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleSystemConfigDefaults serves GET /api/system-config/defaults.
func (s *Server) handleSystemConfigDefaults(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdminHTTP(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, systemconfig.DefaultDto())
}

// handleStorageTemplateOptions serves GET /api/system-config/storage-template-options.
func (s *Server) handleStorageTemplateOptions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdminHTTP(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, systemconfig.GetStorageTemplateStorageOptions())
}
