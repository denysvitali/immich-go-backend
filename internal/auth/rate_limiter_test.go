package auth

import (
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginRateLimitKey(t *testing.T) {
	assert.Equal(t, "user@example.com", loginRateLimitKey(" User@Example.COM "))
}

func TestLoginRateLimiterBlocksAfterConfiguredFailures(t *testing.T) {
	limiter := newLoginRateLimiter(2, time.Minute)
	require.NotNil(t, limiter)

	key := "user@example.com"
	assert.True(t, limiter.allow(key))
	limiter.recordFailure(key)

	assert.True(t, limiter.allow(key))
	limiter.recordFailure(key)

	assert.False(t, limiter.allow(key))
}

func TestLoginRateLimiterResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	limiter := newLoginRateLimiter(1, time.Minute)
	require.NotNil(t, limiter)
	limiter.now = func() time.Time {
		return now
	}

	key := "user@example.com"
	limiter.recordFailure(key)
	assert.False(t, limiter.allow(key))

	now = now.Add(time.Minute)
	assert.True(t, limiter.allow(key))
}

func TestLoginRateLimiterResetClearsFailures(t *testing.T) {
	limiter := newLoginRateLimiter(1, time.Minute)
	require.NotNil(t, limiter)

	key := "user@example.com"
	limiter.recordFailure(key)
	assert.False(t, limiter.allow(key))

	limiter.reset(key)
	assert.True(t, limiter.allow(key))
}

func TestLoginRateLimiterCanBeDisabled(t *testing.T) {
	assert.Nil(t, newLoginRateLimiter(0, time.Minute))
	assert.Nil(t, newLoginRateLimiter(2, 0))

	service := &Service{
		config: config.AuthConfig{},
	}
	assert.True(t, service.allowLoginAttempt("user@example.com"))
	service.recordFailedLogin("user@example.com")
	assert.True(t, service.allowLoginAttempt("user@example.com"))
}
