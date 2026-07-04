package server

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type partnerCreateRequest struct {
	SharedWithID string `json:"sharedWithId"`
}

type partnerResponseDTO struct {
	AvatarColor      string `json:"avatarColor"`
	Email            string `json:"email"`
	ID               string `json:"id"`
	InTimeline       bool   `json:"inTimeline"`
	Name             string `json:"name"`
	ProfileChangedAt string `json:"profileChangedAt"`
	ProfileImagePath string `json:"profileImagePath"`
}

func (s *Server) handlePartnerCreate(w http.ResponseWriter, r *http.Request, pathPartnerID string) {
	claims, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	sharedWithID := pathPartnerID
	if sharedWithID == "" {
		var body partnerCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid partner request"})
			return
		}
		sharedWithID = body.SharedWithID
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "invalid user ID"})
		return
	}
	partnerID, err := uuid.Parse(sharedWithID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid partner ID"})
		return
	}

	partnerUser, err := s.queries.GetUserByID(r.Context(), pgtype.UUID{Bytes: partnerID, Valid: true})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "partner user not found"})
		return
	}

	partnership, err := s.queries.CreatePartnership(r.Context(), sqlc.CreatePartnershipParams{
		SharedById:   pgtype.UUID{Bytes: userID, Valid: true},
		SharedWithId: pgtype.UUID{Bytes: partnerID, Valid: true},
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create partnership"})
		return
	}

	writeJSON(w, http.StatusCreated, partnerDTO(partnerUser, partnership.InTimeline))
}

func partnerDTO(user sqlc.User, inTimeline bool) partnerResponseDTO {
	return partnerResponseDTO{
		AvatarColor:      user.AvatarColor.String,
		Email:            user.Email,
		ID:               uuid.UUID(user.ID.Bytes).String(),
		InTimeline:       inTimeline,
		Name:             user.Name,
		ProfileChangedAt: user.ProfileChangedAt.Time.Format(time.RFC3339Nano),
		ProfileImagePath: user.ProfileImagePath,
	}
}
