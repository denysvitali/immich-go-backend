package maintenance

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("immich-go-backend/maintenance")

// MaintenanceState represents the current maintenance mode state
type MaintenanceState struct {
	IsMaintenanceMode bool      `json:"isMaintenanceMode"`
	Secret            string    `json:"secret,omitempty"`
	StartedBy         string    `json:"startedBy,omitempty"`
	StartedAt         time.Time `json:"startedAt,omitempty"`
}

// MaintenanceClaims represents JWT claims for maintenance mode
type MaintenanceClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Service handles maintenance mode operations
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// In-memory state (in production, this should be stored in system_metadata table)
	mu    sync.RWMutex
	state MaintenanceState
}

// NewService creates a new maintenance service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	return &Service{
		db:     queries,
		config: cfg,
		state: MaintenanceState{
			IsMaintenanceMode: false,
		},
	}, nil
}

// GetMaintenanceMode returns the current maintenance mode state
func (s *Service) GetMaintenanceMode(ctx context.Context) (*MaintenanceState, error) {
	ctx, span := tracer.Start(ctx, "maintenance.get_mode")
	defer span.End()

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	stateCopy := MaintenanceState{
		IsMaintenanceMode: s.state.IsMaintenanceMode,
		StartedBy:         s.state.StartedBy,
		StartedAt:         s.state.StartedAt,
	}

	span.SetAttributes(attribute.Bool("is_maintenance_mode", stateCopy.IsMaintenanceMode))

	return &stateCopy, nil
}

// StartMaintenance enables maintenance mode
func (s *Service) StartMaintenance(ctx context.Context, username string) (string, error) {
	ctx, span := tracer.Start(ctx, "maintenance.start",
		trace.WithAttributes(attribute.String("username", username)))
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a random secret for JWT signing
	secret, err := generateMaintenanceSecret()
	if err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("failed to generate maintenance secret: %w", err)
	}

	s.state = MaintenanceState{
		IsMaintenanceMode: true,
		Secret:            secret,
		StartedBy:         username,
		StartedAt:         time.Now(),
	}

	// Generate JWT token
	token, err := s.signMaintenanceJWT(secret, username)
	if err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("failed to sign maintenance JWT: %w", err)
	}

	span.SetAttributes(attribute.Bool("maintenance_started", true))

	return token, nil
}

// StopMaintenance disables maintenance mode
func (s *Service) StopMaintenance(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "maintenance.stop")
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = MaintenanceState{
		IsMaintenanceMode: false,
	}

	span.SetAttributes(attribute.Bool("maintenance_stopped", true))

	return nil
}

// ValidateMaintenanceToken validates a maintenance JWT token
func (s *Service) ValidateMaintenanceToken(ctx context.Context, tokenString string) (*MaintenanceClaims, error) {
	ctx, span := tracer.Start(ctx, "maintenance.validate_token")
	defer span.End()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.state.IsMaintenanceMode {
		return nil, fmt.Errorf("not in maintenance mode")
	}

	token, err := jwt.ParseWithClaims(tokenString, &MaintenanceClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.state.Secret), nil
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*MaintenanceClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	span.SetAttributes(attribute.String("username", claims.Username))

	return claims, nil
}

// IsMaintenanceMode returns whether maintenance mode is enabled
func (s *Service) IsMaintenanceMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.IsMaintenanceMode
}

// signMaintenanceJWT creates a JWT token for maintenance mode
func (s *Service) signMaintenanceJWT(secret, username string) (string, error) {
	claims := MaintenanceClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "immich-maintenance",
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// generateMaintenanceSecret generates a random secret for maintenance mode
func generateMaintenanceSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
