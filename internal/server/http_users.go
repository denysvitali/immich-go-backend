package server

import (
	"net/http"

	"github.com/google/uuid"
)

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
