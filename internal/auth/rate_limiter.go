package auth

import (
	"strings"
	"sync"
	"time"
)

type loginAttemptState struct {
	count      int
	windowEnds time.Time
}

type loginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttemptState
	limit    int
	window   time.Duration
	now      func() time.Time
}

func newLoginRateLimiter(limit int, window time.Duration) *loginRateLimiter {
	if limit <= 0 || window <= 0 {
		return nil
	}

	return &loginRateLimiter{
		attempts: make(map[string]loginAttemptState),
		limit:    limit,
		window:   window,
		now:      time.Now,
	}
}

func loginRateLimitKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *Service) allowLoginAttempt(key string) bool {
	if s.loginLimiter == nil {
		return true
	}

	return s.loginLimiter.allow(key)
}

func (s *Service) recordFailedLogin(key string) {
	if s.loginLimiter == nil {
		return
	}

	s.loginLimiter.recordFailure(key)
}

func (s *Service) resetLoginAttempts(key string) {
	if s.loginLimiter == nil {
		return
	}

	s.loginLimiter.reset(key)
}

func (l *loginRateLimiter) allow(key string) bool {
	if key == "" {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	state, ok := l.attempts[key]
	if !ok {
		return true
	}
	if !now.Before(state.windowEnds) {
		delete(l.attempts, key)
		return true
	}

	return state.count < l.limit
}

func (l *loginRateLimiter) recordFailure(key string) {
	if key == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	state, ok := l.attempts[key]
	if !ok || !now.Before(state.windowEnds) {
		l.attempts[key] = loginAttemptState{
			count:      1,
			windowEnds: now.Add(l.window),
		}
		return
	}

	state.count++
	l.attempts[key] = state
}

func (l *loginRateLimiter) reset(key string) {
	if key == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.attempts, key)
}
