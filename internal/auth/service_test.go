package auth

import (
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateToken(t *testing.T) {
	cfg := config.AuthConfig{
		JWTSecret: "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry: time.Hour,
	}

	service := &Service{
		config: cfg,
	}

	userID := "test-user-id"
	email := "test@example.com"
	duration := time.Hour

	token, err := service.GenerateToken(userID, email, duration)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Parse and validate the token
	parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})
	require.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	claims, ok := parsedToken.Claims.(*Claims)
	require.True(t, ok)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
}

func TestValidateToken(t *testing.T) {
	cfg := config.AuthConfig{
		JWTSecret: "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry: time.Hour,
	}

	service := &Service{
		config: cfg,
	}

	t.Run("valid token", func(t *testing.T) {
		// Generate a valid token
		token, err := service.GenerateToken("user-123", "user@example.com", time.Hour)
		require.NoError(t, err)

		// Validate it
		claims, err := service.ValidateToken(token)
		require.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, "user-123", claims.UserID)
		assert.Equal(t, "user@example.com", claims.Email)
	})

	t.Run("invalid token", func(t *testing.T) {
		claims, err := service.ValidateToken("invalid.token.here")
		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("expired token", func(t *testing.T) {
		// Create an expired token
		expiredClaims := &Claims{
			UserID: "user-123",
			Email:  "user@example.com",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}

		expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
		tokenString, err := expiredToken.SignedString([]byte(cfg.JWTSecret))
		require.NoError(t, err)

		// Try to validate expired token
		claims, err := service.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "expired")
	})
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		config   config.AuthConfig
		password string
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid password - no requirements",
			config: config.AuthConfig{
				PasswordMinLength: 8,
			},
			password: "password",
			wantErr:  false,
		},
		{
			name: "too short",
			config: config.AuthConfig{
				PasswordMinLength: 10,
			},
			password: "short",
			wantErr:  true,
			errMsg:   "at least 10 characters",
		},
		{
			name: "missing uppercase",
			config: config.AuthConfig{
				PasswordMinLength:        8,
				PasswordRequireUppercase: true,
			},
			password: "password123",
			wantErr:  true,
			errMsg:   "uppercase letter",
		},
		{
			name: "missing lowercase",
			config: config.AuthConfig{
				PasswordMinLength:        8,
				PasswordRequireLowercase: true,
			},
			password: "PASSWORD123",
			wantErr:  true,
			errMsg:   "lowercase letter",
		},
		{
			name: "missing numbers",
			config: config.AuthConfig{
				PasswordMinLength:      8,
				PasswordRequireNumbers: true,
			},
			password: "Password",
			wantErr:  true,
			errMsg:   "number",
		},
		{
			name: "missing symbols",
			config: config.AuthConfig{
				PasswordMinLength:      8,
				PasswordRequireSymbols: true,
			},
			password: "Password123",
			wantErr:  true,
			errMsg:   "symbol",
		},
		{
			name: "all requirements met",
			config: config.AuthConfig{
				PasswordMinLength:        8,
				PasswordRequireUppercase: true,
				PasswordRequireLowercase: true,
				PasswordRequireNumbers:   true,
				PasswordRequireSymbols:   true,
			},
			password: "Pass@123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{
				config: tt.config,
			}

			err := service.validatePassword(tt.password)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateUserID(t *testing.T) {
	service := &Service{}

	id1, err := service.generateUserID()
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	id2, err := service.generateUserID()
	require.NoError(t, err)
	assert.NotEmpty(t, id2)

	// IDs should be unique
	assert.NotEqual(t, id1, id2)
}
