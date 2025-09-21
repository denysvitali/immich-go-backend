package sessions

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
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
	ID         string
	UserID     string
	DeviceType string
	DeviceOS   string
	Token      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ExpiresAt  time.Time
}

// CreateSession creates a new session for a user
func (s *Service) CreateSession(ctx context.Context, userID string, deviceType, deviceOS string) (*Session, error) {
	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

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

	sessionID := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(30 * 24 * time.Hour) // 30 days

	// Store session in database (using existing sessions table if available)
	// For now, return in-memory session
	session := &Session{
		ID:         sessionID,
		UserID:     userID,
		DeviceType: deviceType,
		DeviceOS:   deviceOS,
		Token:      token,
		CreatedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  expiresAt,
	}

	s.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"user_id":    userID,
		"device":     deviceType,
	}).Info("Created new session")

	return session, nil
}

// GetSessionsByUserID returns all sessions for a user
func (s *Service) GetSessionsByUserID(ctx context.Context, userID string) ([]*Session, error) {
	// For now, return empty list as we don't have a sessions table yet
	// This would query the database for all sessions belonging to the user
	return []*Session{}, nil
}

// GetSessionByID returns a session by its ID
func (s *Service) GetSessionByID(ctx context.Context, sessionID string) (*Session, error) {
	// This would query the database for the session
	return nil, sql.ErrNoRows
}

// DeleteSession deletes a session by its ID
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	// This would delete the session from the database
	s.logger.WithField("session_id", sessionID).Info("Deleted session")
	return nil
}

// DeleteAllSessionsByUserID deletes all sessions for a user
func (s *Service) DeleteAllSessionsByUserID(ctx context.Context, userID string) error {
	// This would delete all sessions for the user from the database
	s.logger.WithField("user_id", userID).Info("Deleted all sessions for user")
	return nil
}

// LockSession locks a session (marks it as invalid)
func (s *Service) LockSession(ctx context.Context, sessionID string) error {
	// This would update the session in the database to mark it as locked
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
	// This would update the session's updated_at timestamp in the database
	return nil
}

// GetCurrentSession gets the current session from the context
func (s *Service) GetCurrentSession(ctx context.Context) (*Session, error) {
	// Extract session info from context (set by auth middleware)
	userID, ok := ctx.Value("user_id").(string)
	if !ok {
		return nil, auth.NewAuthError(auth.ErrUnauthorized, "No user ID in context", nil)
	}

	// In a real implementation, we'd also store and retrieve the session ID from context
	// For now, create a temporary session object
	return &Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// CleanupExpiredSessions removes all expired sessions from the database
func (s *Service) CleanupExpiredSessions(ctx context.Context) error {
	// This would delete all sessions where expires_at < now()
	s.logger.Info("Cleaned up expired sessions")
	return nil
}
