package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConfig(t *testing.T) {
	cfg := ServerConfig{
		Address:            "0.0.0.0:8080",
		GRPCAddress:        "0.0.0.0:9090",
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		IdleTimeout:        120 * time.Second,
		ShutdownTimeout:    30 * time.Second,
		CORSEnabled:        true,
		CORSAllowedOrigins: []string{"http://localhost:3000"},
		RequestLogging:     true,
		MetricsEnabled:     true,
		MetricsPath:        "/metrics",
		HealthCheckEnabled: true,
		HealthCheckPath:    "/health",
	}

	assert.Equal(t, "0.0.0.0:8080", cfg.Address)
	assert.Equal(t, "0.0.0.0:9090", cfg.GRPCAddress)
	assert.Equal(t, 30*time.Second, cfg.ReadTimeout)
	assert.True(t, cfg.CORSEnabled)
	assert.Contains(t, cfg.CORSAllowedOrigins, "http://localhost:3000")
	assert.True(t, cfg.MetricsEnabled)
	assert.Equal(t, "/metrics", cfg.MetricsPath)
}

func TestServerMetricsDefaultToDisabled(t *testing.T) {
	var cfg Config
	setDefaults(&cfg)
	assert.False(t, cfg.Server.MetricsEnabled)
}

func TestDatabaseConfig(t *testing.T) {
	cfg := DatabaseConfig{
		URL:             "postgres://user:pass@localhost:5432/db",
		MaxOpenConns:    25,
		MinIdleConns:    5,
		MaxConnLifetime: time.Hour,
		ConnectTimeout:  30 * time.Second,
		QueryTimeout:    30 * time.Second,
		LogQueries:      false,
		AutoMigrate:     true,
	}

	assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.URL)
	assert.Equal(t, 25, cfg.MaxOpenConns)
	assert.Equal(t, 5, cfg.MinIdleConns)
	assert.Equal(t, time.Hour, cfg.MaxConnLifetime)
	assert.False(t, cfg.LogQueries)
	assert.True(t, cfg.AutoMigrate)
}

func TestAuthConfig(t *testing.T) {
	cfg := AuthConfig{
		JWTSecret:                 "super-secret-key",
		JWTExpiry:                 24 * time.Hour,
		JWTRefreshExpiry:          7 * 24 * time.Hour,
		JWTIssuer:                 "immich-go-backend",
		RegistrationEnabled:       true,
		EmailVerificationRequired: false,
		PasswordMinLength:         8,
		PasswordRequireUppercase:  true,
		PasswordRequireLowercase:  true,
		PasswordRequireNumbers:    true,
		PasswordRequireSymbols:    false,
		SessionTimeout:            24 * time.Hour,
		LoginRateLimit:            5,
		LoginRateWindow:           15 * time.Minute,
	}

	assert.Equal(t, "super-secret-key", cfg.JWTSecret)
	assert.Equal(t, 24*time.Hour, cfg.JWTExpiry)
	assert.Equal(t, 7*24*time.Hour, cfg.JWTRefreshExpiry)
	assert.Equal(t, "immich-go-backend", cfg.JWTIssuer)
	assert.True(t, cfg.RegistrationEnabled)
	assert.Equal(t, 8, cfg.PasswordMinLength)
	assert.True(t, cfg.PasswordRequireUppercase)
	assert.False(t, cfg.PasswordRequireSymbols)
	assert.Equal(t, 5, cfg.LoginRateLimit)
}

func TestOAuthConfig(t *testing.T) {
	cfg := OAuthConfig{
		Enabled: true,
		Google: OAuthProviderConfig{
			Enabled:      true,
			ClientID:     "google-client-id",
			ClientSecret: "google-client-secret",
			RedirectURL:  "http://localhost:8080/auth/google/callback",
			Scopes:       []string{"email", "profile"},
		},
		GitHub: OAuthProviderConfig{
			Enabled:      false,
			ClientID:     "",
			ClientSecret: "",
			RedirectURL:  "",
			Scopes:       []string{},
		},
	}

	assert.True(t, cfg.Enabled)
	assert.True(t, cfg.Google.Enabled)
	assert.Equal(t, "google-client-id", cfg.Google.ClientID)
	assert.Contains(t, cfg.Google.Scopes, "email")
	assert.Contains(t, cfg.Google.Scopes, "profile")
	assert.False(t, cfg.GitHub.Enabled)
}

func TestJobsConfig(t *testing.T) {
	cfg := JobsConfig{
		Enabled:         true,
		RedisURL:        "redis://localhost:6379",
		Workers:         10,
		RetryMaxRetries: 10,
		JobTimeout:      30 * time.Minute,
	}

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "redis://localhost:6379", cfg.RedisURL)
	assert.Equal(t, 10, cfg.Workers)
	assert.Equal(t, 10, cfg.RetryMaxRetries)
	assert.Equal(t, 30*time.Minute, cfg.JobTimeout)
}

func TestFeatureConfig(t *testing.T) {
	cfg := FeatureConfig{
		MachineLearningEnabled:  false,
		FaceRecognitionEnabled:  false,
		ObjectDetectionEnabled:  false,
		CLIPSearchEnabled:       false,
		VideoTranscodingEnabled: true,
	}

	assert.False(t, cfg.MachineLearningEnabled)
	assert.False(t, cfg.FaceRecognitionEnabled)
	assert.False(t, cfg.ObjectDetectionEnabled)
	assert.False(t, cfg.CLIPSearchEnabled)
	assert.True(t, cfg.VideoTranscodingEnabled)
}

func TestLoadConfigFromFile(t *testing.T) {
	t.Run("loads valid YAML config", func(t *testing.T) {
		// Create a temporary config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		configContent := `
server:
  address: "127.0.0.1:8080"
  grpc_address: "127.0.0.1:9090"
  cors_enabled: true

database:
  url: "postgres://test:test@localhost:5432/test"
  max_open_conns: 10

auth:
  jwt_secret: "0123456789abcdef0123456789abcdef"
  password_min_length: 10
`
		err := os.WriteFile(configPath, []byte(configContent), 0o600)
		require.NoError(t, err)

		cfg, err := LoadConfig(configPath)
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:8080", cfg.Server.Address)
		assert.Equal(t, "0123456789abcdef0123456789abcdef", cfg.Auth.JWTSecret)
	})

	t.Run("handles missing config file gracefully", func(t *testing.T) {
		cfg, err := LoadConfigWithoutValidation(filepath.Join(t.TempDir(), "missing.yaml"))
		require.NoError(t, err)
		assert.Equal(t, "0.0.0.0:8080", cfg.Server.Address)
	})

	t.Run("rejects unknown YAML fields", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("storage:\n  upload_location: ./ignored\n"), 0o600))

		_, err := LoadConfigWithoutValidation(configPath)
		require.ErrorContains(t, err, "field upload_location not found")
	})
}

func TestShippedConfigUsesOnlySupportedFields(t *testing.T) {
	_, err := LoadConfigWithoutValidation(filepath.Join("..", "..", "config.yaml"))
	require.NoError(t, err)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr string
	}{
		{name: "valid 32 byte secret", secret: "0123456789abcdef0123456789abcdef"},
		{name: "missing", wantErr: "AUTH_JWT_SECRET is required"},
		{name: "too short", secret: "short-but-not-a-placeholder", wantErr: "at least 32 bytes"},
		{
			name:    "shipped placeholder",
			secret:  "your-super-secret-jwt-key-change-this-in-production",
			wantErr: "placeholder value",
		},
		{
			name:    "padded change-me placeholder",
			secret:  "CHANGE_ME_32_BYTES_CHANGE_ME_32_BYTES",
			wantErr: "placeholder value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			setDefaults(&cfg)
			cfg.Auth.JWTSecret = tc.secret

			err := validateConfig(&cfg)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestTLSConfig(t *testing.T) {
	t.Run("TLS disabled", func(t *testing.T) {
		cfg := TLSConfig{
			Enabled: false,
		}

		assert.False(t, cfg.Enabled)
		assert.Empty(t, cfg.CertFile)
		assert.Empty(t, cfg.KeyFile)
	})

	t.Run("TLS with cert files", func(t *testing.T) {
		cfg := TLSConfig{
			Enabled:  true,
			CertFile: "/path/to/cert.pem",
			KeyFile:  "/path/to/key.pem",
		}

		assert.True(t, cfg.Enabled)
		assert.Equal(t, "/path/to/cert.pem", cfg.CertFile)
		assert.Equal(t, "/path/to/key.pem", cfg.KeyFile)
	})
}

func TestTLSRequiresCertificatePair(t *testing.T) {
	var cfg Config
	setDefaults(&cfg)
	cfg.Auth.JWTSecret = "0123456789abcdef0123456789abcdef"
	cfg.Server.TLS.Enabled = true

	require.ErrorContains(t, validateConfig(&cfg), "TLS_CERT_FILE and TLS_KEY_FILE")

	cfg.Server.TLS.CertFile = "/tls/cert.pem"
	cfg.Server.TLS.KeyFile = "/tls/key.pem"
	require.NoError(t, validateConfig(&cfg))
}

func TestLoadConfigFromEnv_JobsRetryMaxRetries(t *testing.T) {
	t.Setenv("AUTH_JWT_SECRET", "test-secret-for-env-0123456789abcdef")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("SERVER_ADDRESS", "0.0.0.0:8080")
	t.Setenv("SERVER_GRPC_ADDRESS", "0.0.0.0:9090")
	t.Setenv("IMMICH_JOBS_RETRY_MAX_RETRIES", "25")

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, 25, cfg.Jobs.RetryMaxRetries)
}

func TestDeprecatedMaximumIdleConnectionsSettingIsRejected(t *testing.T) {
	t.Setenv("DATABASE_MAX_IDLE_CONNS", "5")

	_, err := LoadConfigWithoutValidation("")
	require.ErrorContains(t, err, "DATABASE_MIN_IDLE_CONNS")
}
