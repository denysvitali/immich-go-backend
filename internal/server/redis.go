package server

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// parseRedisURL parses a Redis URL and returns address, password, and database number.
// Supports formats:
// - redis://localhost:6379
// - redis://:password@localhost:6379
// - redis://:password@localhost:6379/0
// - rediss://... (TLS)
func parseRedisURL(redisURL string) (string, string, int) {
	addr := "localhost:6379"
	password := ""
	db := 0

	if redisURL == "" {
		return addr, password, db
	}

	parsed, err := url.Parse(redisURL)
	if err != nil {
		logrus.WithError(err).Warn("Failed to parse Redis URL, using defaults")
		return addr, password, db
	}

	if parsed.Host != "" {
		addr = parsed.Host

		if !strings.Contains(addr, ":") {
			addr += ":6379"
		}
	}

	if parsed.User != nil {
		password, _ = parsed.User.Password()
	}

	if parsed.Path != "" && len(parsed.Path) > 1 {
		dbStr := strings.TrimPrefix(parsed.Path, "/")
		if dbNum, err := strconv.Atoi(dbStr); err == nil {
			db = dbNum
		}
	}

	return addr, password, db
}
