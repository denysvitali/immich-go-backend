package config

import (
	"strings"
	"testing"
	"time"
)

// This file pins the *current* environment-variable behaviour of LoadConfig.
//
// It exists because the config structs carry 72 `env:"…"` tags while only the
// hand-written os.Getenv block in loadFromEnv is actually consulted. The two
// lists disagree, and the disagreement is invisible at compile time. These
// tests make the real contract executable: honouredEnvVars is what works,
// ignoredEnvTags is what silently does nothing.
//
// If someone wires the struct tags up (see FINDINGS.md #2), TestIgnoredEnvTags
// fails loudly — which is the intent. Update it deliberately, don't delete it.

// loadWithEnv loads a config from environment only, with the mandatory
// AUTH_JWT_SECRET always present so validateConfig passes. Extra key/value
// pairs are applied on top and are cleaned up by t.Setenv.
func loadWithEnv(t *testing.T, kv ...string) *Config {
	t.Helper()

	t.Setenv("AUTH_JWT_SECRET", "characterization-secret")
	for i := 0; i+1 < len(kv); i += 2 {
		t.Setenv(kv[i], kv[i+1])
	}

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

// TestHonouredEnvVars asserts that every environment variable loadFromEnv
// reads does in fact reach the Config it returns.
func TestHonouredEnvVars(t *testing.T) {
	tests := map[string]struct {
		env   string
		value string
		check func(*Config) any
		want  any
	}{
		"server address":     {"SERVER_ADDRESS", "1.2.3.4:1111", func(c *Config) any { return c.Server.Address }, "1.2.3.4:1111"},
		"grpc address":       {"SERVER_GRPC_ADDRESS", "1.2.3.4:2222", func(c *Config) any { return c.Server.GRPCAddress }, "1.2.3.4:2222"},
		"database url":       {"DATABASE_URL", "postgres://x/y", func(c *Config) any { return c.Database.URL }, "postgres://x/y"},
		"auto migrate":       {"IMMICH_DATABASE_AUTO_MIGRATE", "false", func(c *Config) any { return c.Database.AutoMigrate }, false},
		"jwt secret":         {"AUTH_JWT_SECRET", "s3cret", func(c *Config) any { return c.Auth.JWTSecret }, "s3cret"},
		"login rate limit":   {"AUTH_LOGIN_RATE_LIMIT", "7", func(c *Config) any { return c.Auth.LoginRateLimit }, 7},
		"login rate window":  {"AUTH_LOGIN_RATE_WINDOW", "3m", func(c *Config) any { return c.Auth.LoginRateWindow }, 3 * time.Minute},
		"storage backend":    {"STORAGE_BACKEND", "local", func(c *Config) any { return c.Storage.Backend }, "local"},
		"storage local root": {"STORAGE_LOCAL_ROOT", "/data/x", func(c *Config) any { return c.Storage.Local.RootPath }, "/data/x"},
		"upload temp dir":    {"UPLOAD_TEMP_DIR", "/tmp/x", func(c *Config) any { return c.Storage.Upload.TempDir }, "/tmp/x"},
		"webui dir":          {"IMMICH_WEBUI_DIR", "/web", func(c *Config) any { return c.WebUIDir }, "/web"},
		"job retries":        {"IMMICH_JOBS_RETRY_MAX_RETRIES", "9", func(c *Config) any { return c.Jobs.RetryMaxRetries }, 9},
		"ml enabled":         {"MACHINE_LEARNING_ENABLED", "true", func(c *Config) any { return c.MachineLearning.Enabled }, true},
		"ml url":             {"MACHINE_LEARNING_URL", "http://ml:3003", func(c *Config) any { return c.MachineLearning.URL }, "http://ml:3003"},
		"ml timeout":         {"MACHINE_LEARNING_TIMEOUT", "90s", func(c *Config) any { return c.MachineLearning.Timeout }, 90 * time.Second},
		"feature ml":         {"FEATURE_MACHINE_LEARNING_ENABLED", "true", func(c *Config) any { return c.Features.MachineLearningEnabled }, true},
		"feature faces":      {"FEATURE_FACE_RECOGNITION_ENABLED", "false", func(c *Config) any { return c.Features.FaceRecognitionEnabled }, false},
		"feature clip":       {"FEATURE_CLIP_SEARCH_ENABLED", "false", func(c *Config) any { return c.Features.CLIPSearchEnabled }, false},
		"feature duplicates": {"FEATURE_DUPLICATE_DETECTION_ENABLED", "false", func(c *Config) any { return c.Features.DuplicateDetectionEnabled }, false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := loadWithEnv(t, tc.env, tc.value)

			if got := tc.check(cfg); got != tc.want {
				t.Errorf("%s=%q → got %v, want %v", tc.env, tc.value, got, tc.want)
			}
		})
	}
}

// TestIgnoredEnvTags documents the 56 `env:"…"` struct tags that loadFromEnv
// never reads. Setting them changes nothing — the value stays at whatever
// setDefaults produced. See FINDINGS.md #2.
func TestIgnoredEnvTags(t *testing.T) {
	tests := map[string]struct {
		env   string
		value string
		check func(*Config) any
		want  any // the default, i.e. the env var had no effect
	}{
		"LOG_LEVEL":            {"LOG_LEVEL", "debug", func(c *Config) any { return c.Logging.Level }, "info"},
		"LOG_FORMAT":           {"LOG_FORMAT", "text", func(c *Config) any { return c.Logging.Format }, "json"},
		"JOBS_ENABLED":         {"JOBS_ENABLED", "false", func(c *Config) any { return c.Jobs.Enabled }, true},
		"JOBS_WORKERS":         {"JOBS_WORKERS", "32", func(c *Config) any { return c.Jobs.Workers }, 4},
		"JOBS_REDIS_URL":       {"JOBS_REDIS_URL", "redis://elsewhere:6379", func(c *Config) any { return c.Jobs.RedisURL }, "redis://localhost:6379/0"},
		"SERVER_READ_TIMEOUT":  {"SERVER_READ_TIMEOUT", "5m", func(c *Config) any { return c.Server.ReadTimeout }, 30 * time.Second},
		"SERVER_WRITE_TIMEOUT": {"SERVER_WRITE_TIMEOUT", "5m", func(c *Config) any { return c.Server.WriteTimeout }, 30 * time.Second},
		"TLS_ENABLED":          {"TLS_ENABLED", "true", func(c *Config) any { return c.Server.TLS.Enabled }, false},
		"AUTH_JWT_EXPIRY":      {"AUTH_JWT_EXPIRY", "99h", func(c *Config) any { return c.Auth.JWTExpiry }, 24 * time.Hour},
		"FEATURE_SHARING":      {"FEATURE_SHARING_ENABLED", "false", func(c *Config) any { return c.Features.SharingEnabled }, true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := loadWithEnv(t, tc.env, tc.value)

			if got := tc.check(cfg); got != tc.want {
				t.Errorf("%s is now honoured (got %v, default is %v). "+
					"If that was intentional, update this test and FINDINGS.md #2.",
					tc.env, got, tc.want)
			}
		})
	}
}

// TestSetDefaultsMatchesEmptyLoad pins that LoadConfig("") is exactly
// setDefaults + loadFromEnv, with no third source of values.
func TestSetDefaultsMatchesEmptyLoad(t *testing.T) {
	var want Config
	setDefaults(&want)

	got := loadWithEnv(t)

	if got.Server.Address != want.Server.Address {
		t.Errorf("Server.Address: got %q, want %q", got.Server.Address, want.Server.Address)
	}
	if got.Server.ReadTimeout != want.Server.ReadTimeout {
		t.Errorf("Server.ReadTimeout: got %v, want %v", got.Server.ReadTimeout, want.Server.ReadTimeout)
	}
	if got.Auth.JWTExpiry != want.Auth.JWTExpiry {
		t.Errorf("Auth.JWTExpiry: got %v, want %v", got.Auth.JWTExpiry, want.Auth.JWTExpiry)
	}
	if got.Jobs.Workers != want.Jobs.Workers {
		t.Errorf("Jobs.Workers: got %v, want %v", got.Jobs.Workers, want.Jobs.Workers)
	}
	if got.Features != want.Features {
		t.Errorf("Features: got %+v, want %+v", got.Features, want.Features)
	}
}

// TestStorageBackendS3RejectedFromEnvAlone pins a consequence of the two
// findings above: STORAGE_BACKEND *is* honoured, but none of the S3 settings
// are, so selecting s3 purely from the environment can only ever fail
// validation. Configuring S3 requires config.yaml today.
func TestStorageBackendS3RejectedFromEnvAlone(t *testing.T) {
	t.Setenv("AUTH_JWT_SECRET", "characterization-secret")
	t.Setenv("STORAGE_BACKEND", "s3")

	_, err := LoadConfig("")
	if err == nil {
		t.Fatal("expected validation to reject an s3 backend with no bucket")
	}
	if !strings.Contains(err.Error(), "bucket is required") {
		t.Errorf("got %v, want a missing-bucket validation error", err)
	}
}
