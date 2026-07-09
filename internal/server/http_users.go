package server

import (
	"encoding/json"
	"net/http"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

// writeUserAdminJSON preserves nullable fields that protojson omits for
// unset optional scalars. The upstream web client requires quotaSizeInBytes
// to be explicitly null when the user has unlimited storage.
func writeUserAdminJSON(w http.ResponseWriter, marshaler runtime.Marshaler, response *immichv1.UserAdminResponse) {
	data, err := marshaler.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, field := range []string{"quotaSizeInBytes", "quotaUsageInBytes", "storageLabel", "deletedAt", "license"} {
		if _, ok := body[field]; !ok {
			body[field] = nil
		}
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleUserOnboardingDelete(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "invalid user ID"})
		return
	}

	if _, err := s.userService.UpdateUserOnboarding(r.Context(), userID, false); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to delete onboarding status"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
