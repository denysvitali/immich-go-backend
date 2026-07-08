package oauth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signLogoutToken(t *testing.T, secret string, claims LogoutClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

func TestValidateLogoutToken_HS256Success(t *testing.T) {
	secret := "test-client-secret"
	now := time.Now()
	claims := LogoutClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-sub-1",
			Audience:  []string{"immich-client"},
			Issuer:    "https://idp.example.com",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
		Sid: "session-sid-1",
		Events: map[string]any{
			backchannelLogoutEvent: map[string]any{},
		},
	}
	token := signLogoutToken(t, secret, claims)

	sub, sid, err := ValidateLogoutToken(OIDCConfig{
		Enabled:          true,
		IssuerURL:        "https://idp.example.com",
		ClientID:         "immich-client",
		ClientSecret:     secret,
		SigningAlgorithm: "HS256",
	}, token)
	require.NoError(t, err)
	assert.Equal(t, "user-sub-1", sub)
	assert.Equal(t, "session-sid-1", sid)
}

func TestValidateLogoutToken_RejectsMissingEvent(t *testing.T) {
	secret := "test-client-secret"
	now := time.Now()
	claims := LogoutClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-sub-1",
			Audience:  []string{"immich-client"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
	}
	token := signLogoutToken(t, secret, claims)

	_, _, err := ValidateLogoutToken(OIDCConfig{
		Enabled:          true,
		ClientID:         "immich-client",
		ClientSecret:     secret,
		SigningAlgorithm: "HS256",
	}, token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token validation failed")
}

func TestValidateLogoutToken_RejectsNonce(t *testing.T) {
	secret := "test-client-secret"
	now := time.Now()
	claims := LogoutClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-sub-1",
			Audience:  []string{"immich-client"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
		Nonce: "must-not-be-present",
		Events: map[string]any{
			backchannelLogoutEvent: map[string]any{},
		},
	}
	token := signLogoutToken(t, secret, claims)

	_, _, err := ValidateLogoutToken(OIDCConfig{
		Enabled:          true,
		ClientID:         "immich-client",
		ClientSecret:     secret,
		SigningAlgorithm: "HS256",
	}, token)
	require.Error(t, err)
}

func TestValidateLogoutToken_RejectsWhenDisabled(t *testing.T) {
	_, _, err := ValidateLogoutToken(OIDCConfig{Enabled: false}, "anything")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oauth not enabled")
}

func TestValidateLogoutToken_SubOnly(t *testing.T) {
	secret := "test-client-secret"
	now := time.Now()
	claims := LogoutClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-sub-only",
			Audience:  []string{"immich-client"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
		Events: map[string]any{
			backchannelLogoutEvent: map[string]any{},
		},
	}
	token := signLogoutToken(t, secret, claims)

	sub, sid, err := ValidateLogoutToken(OIDCConfig{
		Enabled:          true,
		ClientID:         "immich-client",
		ClientSecret:     secret,
		SigningAlgorithm: "HS256",
	}, token)
	require.NoError(t, err)
	assert.Equal(t, "user-sub-only", sub)
	assert.Empty(t, sid)
}
