package server

import (
	"encoding/json"
	"net/http"
	"strings"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// handleActivityRoutes preserves the upstream REST contract where the
// grpc-gateway representation differs: list responses are arrays, reaction
// enums are lowercase strings, and create/delete use 201/204 respectively.
func (s *Server) handleActivityRoutes(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/activities":
		s.handleGetActivities(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/activities/statistics":
		s.handleGetActivityStatistics(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/activities":
		s.handleCreateActivity(w, r)
	case r.Method == http.MethodDelete:
		activityID, ok := activityIDFromPath(r.URL.Path)
		if !ok {
			return false
		}
		s.handleDeleteActivity(w, r, activityID)
	default:
		return false
	}
	return true
}

func (s *Server) handleGetActivities(w http.ResponseWriter, r *http.Request) {
	ctx, ok := s.frontendGatewayContext(w, r)
	if !ok {
		return
	}
	req, err := activitySearchRequest(r)
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}

	response, err := s.activityService.GetActivities(ctx, req)
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}
	activities := make([]activityHTTPResponse, len(response.Activities))
	for i, activity := range response.Activities {
		activities[i] = activityHTTPResponseFromProto(activity)
	}
	writeJSON(w, http.StatusOK, activities)
}

func (s *Server) handleGetActivityStatistics(w http.ResponseWriter, r *http.Request) {
	ctx, ok := s.frontendGatewayContext(w, r)
	if !ok {
		return
	}
	req := &immichv1.GetActivityStatisticsRequest{AlbumId: r.URL.Query().Get("albumId")}
	if assetID := r.URL.Query().Get("assetId"); assetID != "" {
		req.AssetId = &assetID
	}

	response, err := s.activityService.GetActivityStatistics(ctx, req)
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, activityStatisticsHTTPResponse{Comments: response.Comments, Likes: response.Likes})
}

func (s *Server) handleCreateActivity(w http.ResponseWriter, r *http.Request) {
	ctx, ok := s.frontendGatewayContext(w, r)
	if !ok {
		return
	}

	var body activityCreateHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeGRPCErrorJSON(w, r, status.Error(codes.InvalidArgument, "invalid activity request"))
		return
	}
	typeValue, err := parseReactionType(body.Type)
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}
	req := &immichv1.CreateActivityRequest{AlbumId: body.AlbumID, AssetId: body.AssetID, Comment: activityStringValue(body.Comment), Type: typeValue}
	response, err := s.activityService.CreateActivity(ctx, req)
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, activityHTTPResponseFromProto(response))
}

func (s *Server) handleDeleteActivity(w http.ResponseWriter, r *http.Request, activityID string) {
	ctx, ok := s.frontendGatewayContext(w, r)
	if !ok {
		return
	}
	if _, err := s.activityService.DeleteActivity(ctx, &immichv1.DeleteActivityRequest{Id: activityID}); err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func activitySearchRequest(r *http.Request) (*immichv1.GetActivitiesRequest, error) {
	query := r.URL.Query()
	req := &immichv1.GetActivitiesRequest{AlbumId: query.Get("albumId")}
	if assetID := query.Get("assetId"); assetID != "" {
		req.AssetId = &assetID
	}
	if userID := query.Get("userId"); userID != "" {
		req.UserId = &userID
	}
	if level := query.Get("level"); level != "" {
		parsed, err := parseReactionLevel(level)
		if err != nil {
			return nil, err
		}
		req.Level = &parsed
	}
	if reactionType := query.Get("type"); reactionType != "" {
		parsed, err := parseReactionType(reactionType)
		if err != nil {
			return nil, err
		}
		req.Type = &parsed
	}
	return req, nil
}

func parseReactionType(value string) (immichv1.ReactionType, error) {
	switch value {
	case "comment":
		return immichv1.ReactionType_REACTION_TYPE_COMMENT, nil
	case "like":
		return immichv1.ReactionType_REACTION_TYPE_LIKE, nil
	default:
		return immichv1.ReactionType_REACTION_TYPE_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid activity type")
	}
}

func parseReactionLevel(value string) (immichv1.ReactionLevel, error) {
	switch value {
	case "album":
		return immichv1.ReactionLevel_REACTION_LEVEL_ALBUM, nil
	case "asset":
		return immichv1.ReactionLevel_REACTION_LEVEL_ASSET, nil
	default:
		return immichv1.ReactionLevel_REACTION_LEVEL_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid activity level")
	}
}

func activityIDFromPath(path string) (string, bool) {
	id, ok := strings.CutPrefix(path, "/api/activities/")
	if !ok || id == "" || id == "statistics" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

type activityCreateHTTPRequest struct {
	AlbumID string  `json:"albumId"`
	AssetID *string `json:"assetId"`
	Comment *string `json:"comment"`
	Type    string  `json:"type"`
}

type activityHTTPResponse struct {
	AssetID   *string          `json:"assetId"`
	Comment   *string          `json:"comment"`
	CreatedAt string           `json:"createdAt"`
	ID        string           `json:"id"`
	Type      string           `json:"type"`
	User      activityHTTPUser `json:"user"`
}

type activityHTTPUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type activityStatisticsHTTPResponse struct {
	Comments int32 `json:"comments"`
	Likes    int32 `json:"likes"`
}

func activityHTTPResponseFromProto(activity *immichv1.ActivityResponseDto) activityHTTPResponse {
	response := activityHTTPResponse{
		CreatedAt: activity.GetCreatedAt().AsTime().Format("2006-01-02T15:04:05.999Z07:00"),
		ID:        activity.Id,
		Type:      "comment",
		User: activityHTTPUser{
			ID:    activity.GetUser().GetId(),
			Email: activity.GetUser().GetEmail(),
			Name:  activity.GetUser().GetName(),
		},
	}
	if activity.Type == immichv1.ReactionType_REACTION_TYPE_LIKE {
		response.Type = "like"
	}
	if activity.AssetId != "" {
		response.AssetID = &activity.AssetId
	}
	if activity.Comment != "" {
		response.Comment = &activity.Comment
	}
	return response
}

func activityStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
