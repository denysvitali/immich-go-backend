package server

import "net/http"

type versionCheckStateDTO struct {
	CheckedAt      *string `json:"checkedAt"`
	ReleaseVersion *string `json:"releaseVersion"`
}

func (s *Server) handleVersionCheckState(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdminHTTP(w, r); !ok {
		return
	}

	state, err := s.systemMetadataService.GetVersionCheckState(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to get version check state"})
		return
	}

	writeJSON(w, http.StatusOK, versionCheckStateDTO{
		CheckedAt:      state.CheckedAt,
		ReleaseVersion: state.ReleaseVersion,
	})
}
