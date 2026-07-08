package sessions

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"
)

type Service struct {
	queries *sqlc.Queries
	auth    *auth.Service
	logger  *logrus.Logger
}

func NewService(queries *sqlc.Queries, authService *auth.Service, logger *logrus.Logger) *Service {
	return &Service{
		queries: queries,
		auth:    authService,
		logger:  logger,
	}
}

// Session represents a user session
type Session struct {
	ID                 string
	UserID             string
	DeviceType         string
	DeviceOS           string
	Token              string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ExpiresAt          time.Time
	IsPendingSyncReset bool
	AppVersion         string
}

// CreateOAuthSession creates a session linked to an OIDC session id (sid) so
// backchannel logout can target it.
func (s *Service) CreateOAuthSession(ctx context.Context, userID, deviceType, deviceOS, oauthSid string) (*Session, error) {
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return nil, err
	}
	user, err := s.queries.GetUser(ctx, userUUID)
	if err != nil {
		return nil, err
	}
	token, err := s.auth.GenerateToken(userID, user.Email, 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	sid := pgtype.Text{}
	if oauthSid != "" {
		sid = pgtype.Text{String: oauthSid, Valid: true}
	}
	dbSession, err := s.queries.CreateSession(ctx, sqlc.CreateSessionParams{
		Token:      token,
		UserId:     userUUID,
		DeviceType: deviceType,
		DeviceOS:   deviceOS,
		ExpiresAt:  pgutil.TimeToTimestamptz(expiresAt),
		OauthSid:   sid,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create oauth session: %w", err)
	}
	return sessionFromDB(dbSession), nil
}

// CreateSession creates a new session for a user
func (s *Service) CreateSession(ctx context.Context, userID string, deviceType, deviceOS string) (*Session, error) {
	// Parse user ID
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return nil, err
	}

	// Get user from database to fetch email
	user, err := s.queries.GetUser(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	// Generate a new session token with real user email
	token, err := s.auth.GenerateToken(userID, user.Email, 30*24*time.Hour)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(30 * 24 * time.Hour) // 30 days

	// Store session in database
	dbSession, err := s.queries.CreateSession(ctx, sqlc.CreateSessionParams{
		Token:      token,
		UserId:     userUUID,
		DeviceType: deviceType,
		DeviceOS:   deviceOS,
		ExpiresAt:  pgutil.TimeToTimestamptz(expiresAt),
		OauthSid:   pgtype.Text{}, // set via CreateOAuthSession for OIDC logins
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Convert to service model
	session := sessionFromDB(dbSession)

	s.logger.WithFields(logrus.Fields{
		"session_id": session.ID,
		"user_id":    userID,
		"device":     deviceType,
	}).Info("Created new session")

	return session, nil
}

// GetSessionsByUserID returns all sessions for a user
func (s *Service) GetSessionsByUserID(ctx context.Context, userID string) ([]*Session, error) {
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	dbSessions, err := s.queries.GetUserSessions(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	sessions := make([]*Session, len(dbSessions))
	for i, dbSession := range dbSessions {
		sessions[i] = sessionFromDB(dbSession)
	}

	return sessions, nil
}

// GetSessionByID returns a session by its ID
func (s *Service) GetSessionByID(ctx context.Context, sessionID string) (*Session, error) {
	sessUUID, err := pgutil.StringToUUID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session ID: %w", err)
	}

	dbSession, err := s.queries.GetSession(ctx, sessUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, auth.NewAuthError(auth.ErrInvalidToken, "Session not found", nil)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check if session is expired
	if dbSession.ExpiresAt.Valid && dbSession.ExpiresAt.Time.Before(time.Now()) {
		return nil, auth.NewAuthError(auth.ErrTokenExpired, "Session has expired", nil)
	}

	return sessionFromDB(dbSession), nil
}

// GetSessionByToken returns a session by its token
func (s *Service) GetSessionByToken(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, auth.NewAuthError(auth.ErrInvalidToken, "Empty token", nil)
	}

	dbSession, err := s.queries.GetSessionByToken(ctx, token)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, auth.NewAuthError(auth.ErrInvalidToken, "Session not found", nil)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check if session is expired
	if dbSession.ExpiresAt.Valid && dbSession.ExpiresAt.Time.Before(time.Now()) {
		return nil, auth.NewAuthError(auth.ErrTokenExpired, "Session has expired", nil)
	}

	return sessionFromDB(dbSession), nil
}

// DeleteSession deletes a session by its ID
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	sessUUID, err := pgutil.StringToUUID(sessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	err = s.queries.DeleteSession(ctx, sessUUID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	s.logger.WithField("session_id", sessionID).Info("Deleted session")
	return nil
}

// DeleteAllSessionsByUserID deletes all sessions for a user
func (s *Service) DeleteAllSessionsByUserID(ctx context.Context, userID string) error {
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	err = s.queries.DeleteUserSessions(ctx, userUUID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	s.logger.WithField("user_id", userID).Info("Deleted all sessions for user")
	return nil
}

// LockSession locks a session (marks it as invalid by setting expiry to now)
func (s *Service) LockSession(ctx context.Context, sessionID string) error {
	// Delete the session to lock it
	err := s.DeleteSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to lock session: %w", err)
	}

	s.logger.WithField("session_id", sessionID).Info("Locked session")
	return nil
}

// ValidateSession checks if a session is valid
func (s *Service) ValidateSession(ctx context.Context, sessionID string) (*Session, error) {
	session, err := s.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, auth.NewAuthError(auth.ErrTokenExpired, "Session has expired", nil)
	}

	return session, nil
}

// RefreshSession updates the session's last activity time
func (s *Service) RefreshSession(ctx context.Context, sessionID string) error {
	sessUUID, err := pgutil.StringToUUID(sessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	err = s.queries.UpdateSessionActivity(ctx, sessUUID)
	if err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}

	return nil
}

// GetCurrentSession gets the current session from the context
func (s *Service) GetCurrentSession(ctx context.Context) (*Session, error) {
	// Extract session info from context (set by auth middleware)
	userID, ok := ctx.Value("user_id").(string)
	if !ok {
		return nil, auth.NewAuthError(auth.ErrUnauthorized, "No user ID in context", nil)
	}

	// Get session token from context
	token, ok := ctx.Value("session_token").(string)
	if !ok {
		// If no session token, create a basic session object with just user ID
		// This is for backwards compatibility with JWT-only auth
		return &Session{
			ID:        uuid.New().String(),
			UserID:    userID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}, nil
	}

	// Get full session from database
	return s.GetSessionByToken(ctx, token)
}

// CleanupExpiredSessions removes all expired sessions from the database
func (s *Service) CleanupExpiredSessions(ctx context.Context) error {
	err := s.queries.DeleteExpiredSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	s.logger.Info("Cleaned up expired sessions")
	return nil
}

func sessionFromDB(dbSession sqlc.Session) *Session {
	return &Session{
		ID:                 pgutil.UUIDToString(dbSession.ID),
		UserID:             pgutil.UUIDToString(dbSession.UserId),
		DeviceType:         dbSession.DeviceType,
		DeviceOS:           dbSession.DeviceOS,
		Token:              dbSession.Token,
		CreatedAt:          pgutil.TimestamptzToTime(dbSession.CreatedAt),
		UpdatedAt:          pgutil.TimestamptzToTime(dbSession.UpdatedAt),
		ExpiresAt:          pgutil.TimestamptzToTime(dbSession.ExpiresAt),
		IsPendingSyncReset: dbSession.IsPendingSyncReset,
		AppVersion:         dbSession.AppVersion.String,
	}
}

// UpdateSession updates mutable session fields (currently the pending sync
// reset flag) for a session owned by the given user.
func (s *Service) UpdateSession(ctx context.Context, userID, sessionID string, isPendingSyncReset *bool) (*Session, error) {
	sessionUUID, err := pgutil.StringToUUID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session ID: %w", err)
	}
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	params := sqlc.UpdateSessionParams{
		ID:     sessionUUID,
		UserId: userUUID,
	}
	if isPendingSyncReset != nil {
		params.IsPendingSyncReset = pgtype.Bool{Bool: *isPendingSyncReset, Valid: true}
	}

	dbSession, err := s.queries.UpdateSession(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return sessionFromDB(dbSession), nil
}
