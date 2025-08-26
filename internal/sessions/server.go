package sessions

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// Server implements the SessionsServiceServer interface
type Server struct {
	service *Service
	immichv1.UnimplementedSessionsServiceServer
}

// NewServer creates a new Sessions server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// GetSessions returns all sessions for the current user
func (s *Server) GetSessions(ctx context.Context, _ *emptypb.Empty) (*immichv1.GetSessionsResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	sessions, err := s.service.GetSessionsByUserID(ctx, userID.String())
	if err != nil {
		return nil, err
	}

	// Get current session to mark it
	currentSession, _ := s.service.GetCurrentSession(ctx)

	response := &immichv1.GetSessionsResponse{
		Sessions: make([]*immichv1.SessionResponse, len(sessions)),
	}

	for i, session := range sessions {
		response.Sessions[i] = &immichv1.SessionResponse{
			Id:         session.ID,
			UserId:     session.UserID,
			DeviceType: session.DeviceType,
			DeviceOs:   session.DeviceOS,
			CreatedAt:  timestamppb.New(session.CreatedAt),
			UpdatedAt:  timestamppb.New(session.UpdatedAt),
			Current:    currentSession != nil && currentSession.ID == session.ID,
		}
	}

	return response, nil
}

// CreateSession creates a new session
func (s *Server) CreateSession(ctx context.Context, req *immichv1.CreateSessionRequest) (*immichv1.CreateSessionResponse, error) {
	// Get user ID from context (from auth middleware)
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	session, err := s.service.CreateSession(ctx, userID.String(), req.DeviceType, req.DeviceOs)
	if err != nil {
		return nil, err
	}

	return &immichv1.CreateSessionResponse{
		AccessToken: session.Token,
		UserId:      session.UserID,
	}, nil
}

// DeleteSession deletes a specific session
func (s *Server) DeleteSession(ctx context.Context, req *immichv1.DeleteSessionRequest) (*emptypb.Empty, error) {
	// Verify user owns this session
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	session, err := s.service.GetSessionByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	if session.UserID != userID.String() {
		return nil, auth.NewAuthError(auth.ErrUnauthorized, "User does not own this session", nil)
	}

	if err := s.service.DeleteSession(ctx, req.Id); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllSessions deletes all sessions for the current user
func (s *Server) DeleteAllSessions(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.service.DeleteAllSessionsByUserID(ctx, userID.String()); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// LockSession locks a specific session
func (s *Server) LockSession(ctx context.Context, req *immichv1.LockSessionRequest) (*emptypb.Empty, error) {
	// Verify user owns this session
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	session, err := s.service.GetSessionByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	if session.UserID != userID.String() {
		return nil, auth.NewAuthError(auth.ErrUnauthorized, "User does not own this session", nil)
	}

	if err := s.service.LockSession(ctx, req.Id); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
