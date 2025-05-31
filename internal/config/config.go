package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// Server configuration
	Server ServerConfig `yaml:"server"`
	
	// Database configuration
	Database DatabaseConfig `yaml:"database"`
	
	// Storage configuration
	Storage storage.StorageConfig `yaml:"storage"`
	
	// Authentication configuration
	Auth AuthConfig `yaml:"auth"`
	
	// Telemetry configuration
	Telemetry telemetry.Config `yaml:"telemetry"`
	
	// Job queue configuration
	Jobs JobsConfig `yaml:"jobs"`
	
	// Logging configuration
	Logging LoggingConfig `yaml:"logging"`
	
	// Feature flags
	Features FeatureConfig `yaml:"features"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	// HTTP server address
	Address string `yaml:"address" env:"SERVER_ADDRESS" default:"0.0.0.0:8080"`
	
	// gRPC server address
	GRPCAddress string `yaml:"grpc_address" env:"SERVER_GRPC_ADDRESS" default:"0.0.0.0:9090"`
	
	// Read timeout
	ReadTimeout time.Duration `yaml:"read_timeout" env:"SERVER_READ_TIMEOUT" default:"30s"`
	
	// Write timeout
	WriteTimeout time.Duration `yaml:"write_timeout" env:"SERVER_WRITE_TIMEOUT" default:"30s"`
	
	// Idle timeout
	IdleTimeout time.Duration `yaml:"idle_timeout" env:"SERVER_IDLE_TIMEOUT" default:"120s"`
	
	// Shutdown timeout
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"SERVER_SHUTDOWN_TIMEOUT" default:"30s"`
	
	// Enable CORS
	CORSEnabled bool `yaml:"cors_enabled" env:"SERVER_CORS_ENABLED" default:"true"`
	
	// CORS allowed origins
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins" env:"SERVER_CORS_ALLOWED_ORIGINS"`
	
	// Enable request logging
	RequestLogging bool `yaml:"request_logging" env:"SERVER_REQUEST_LOGGING" default:"true"`
	
	// Enable metrics endpoint
	MetricsEnabled bool `yaml:"metrics_enabled" env:"SERVER_METRICS_ENABLED" default:"true"`
	
	// Metrics endpoint path
	MetricsPath string `yaml:"metrics_path" env:"SERVER_METRICS_PATH" default:"/metrics"`
	
	// Enable health check endpoint
	HealthCheckEnabled bool `yaml:"health_check_enabled" env:"SERVER_HEALTH_CHECK_ENABLED" default:"true"`
	
	// Health check endpoint path
	HealthCheckPath string `yaml:"health_check_path" env:"SERVER_HEALTH_CHECK_PATH" default:"/health"`
	
	// TLS configuration
	TLS TLSConfig `yaml:"tls"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	// Enable TLS
	Enabled bool `yaml:"enabled" env:"TLS_ENABLED" default:"false"`
	
	// Certificate file path
	CertFile string `yaml:"cert_file" env:"TLS_CERT_FILE"`
	
	// Private key file path
	KeyFile string `yaml:"key_file" env:"TLS_KEY_FILE"`
	
	// Auto-generate self-signed certificate
	AutoGenerate bool `yaml:"auto_generate" env:"TLS_AUTO_GENERATE" default:"false"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	// Database URL
	URL string `yaml:"url" env:"DATABASE_URL" default:"postgres://immich:immich@localhost:5432/immich?sslmode=disable"`
	
	// Maximum number of open connections
	MaxOpenConns int `yaml:"max_open_conns" env:"DATABASE_MAX_OPEN_CONNS" default:"25"`
	
	// Maximum number of idle connections
	MaxIdleConns int `yaml:"max_idle_conns" env:"DATABASE_MAX_IDLE_CONNS" default:"5"`
	
	// Maximum connection lifetime
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime" env:"DATABASE_MAX_CONN_LIFETIME" default:"1h"`
	
	// Connection timeout
	ConnectTimeout time.Duration `yaml:"connect_timeout" env:"DATABASE_CONNECT_TIMEOUT" default:"30s"`
	
	// Query timeout
	QueryTimeout time.Duration `yaml:"query_timeout" env:"DATABASE_QUERY_TIMEOUT" default:"30s"`
	
	// Enable query logging
	LogQueries bool `yaml:"log_queries" env:"DATABASE_LOG_QUERIES" default:"false"`
	
	// Enable migrations
	AutoMigrate bool `yaml:"auto_migrate" env:"DATABASE_AUTO_MIGRATE" default:"true"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	// JWT secret key
	JWTSecret string `yaml:"jwt_secret" env:"AUTH_JWT_SECRET"`
	
	// JWT token expiry
	JWTExpiry time.Duration `yaml:"jwt_expiry" env:"AUTH_JWT_EXPIRY" default:"24h"`
	
	// JWT refresh token expiry
	JWTRefreshExpiry time.Duration `yaml:"jwt_refresh_expiry" env:"AUTH_JWT_REFRESH_EXPIRY" default:"168h"` // 7 days
	
	// JWT issuer
	JWTIssuer string `yaml:"jwt_issuer" env:"AUTH_JWT_ISSUER" default:"immich-go-backend"`
	
	// Enable user registration
	RegistrationEnabled bool `yaml:"registration_enabled" env:"AUTH_REGISTRATION_ENABLED" default:"true"`
	
	// Require email verification
	EmailVerificationRequired bool `yaml:"email_verification_required" env:"AUTH_EMAIL_VERIFICATION_REQUIRED" default:"false"`
	
	// Password minimum length
	PasswordMinLength int `yaml:"password_min_length" env:"AUTH_PASSWORD_MIN_LENGTH" default:"8"`
	
	// Password complexity requirements
	PasswordRequireUppercase bool `yaml:"password_require_uppercase" env:"AUTH_PASSWORD_REQUIRE_UPPERCASE" default:"false"`
	PasswordRequireLowercase bool `yaml:"password_require_lowercase" env:"AUTH_PASSWORD_REQUIRE_LOWERCASE" default:"false"`
	PasswordRequireNumbers   bool `yaml:"password_require_numbers" env:"AUTH_PASSWORD_REQUIRE_NUMBERS" default:"false"`
	PasswordRequireSymbols   bool `yaml:"password_require_symbols" env:"AUTH_PASSWORD_REQUIRE_SYMBOLS" default:"false"`
	
	// Session configuration
	SessionTimeout time.Duration `yaml:"session_timeout" env:"AUTH_SESSION_TIMEOUT" default:"24h"`
	
	// Rate limiting
	LoginRateLimit    int           `yaml:"login_rate_limit" env:"AUTH_LOGIN_RATE_LIMIT" default:"5"`
	LoginRateWindow   time.Duration `yaml:"login_rate_window" env:"AUTH_LOGIN_RATE_WINDOW" default:"15m"`
	
	// OAuth configuration
	OAuth OAuthConfig `yaml:"oauth"`
}

// OAuthConfig represents OAuth configuration
type OAuthConfig struct {
	// Enable OAuth
	Enabled bool `yaml:"enabled" env:"OAUTH_ENABLED" default:"false"`
	
	// Google OAuth
	Google OAuthProviderConfig `yaml:"google"`
	
	// GitHub OAuth
	GitHub OAuthProviderConfig `yaml:"github"`
	
	// Microsoft OAuth
	Microsoft OAuthProviderConfig `yaml:"microsoft"`
}

// OAuthProviderConfig represents OAuth provider configuration
type OAuthProviderConfig struct {
	// Enable this provider
	Enabled bool `yaml:"enabled" env:"OAUTH_{PROVIDER}_ENABLED" default:"false"`
	
	// Client ID
	ClientID string `yaml:"client_id" env:"OAUTH_{PROVIDER}_CLIENT_ID"`
	
	// Client secret
	ClientSecret string `yaml:"client_secret" env:"OAUTH_{PROVIDER}_CLIENT_SECRET"`
	
	// Redirect URL
	RedirectURL string `yaml:"redirect_url" env:"OAUTH_{PROVIDER}_REDIRECT_URL"`
	
	// Scopes
	Scopes []string `yaml:"scopes" env:"OAUTH_{PROVIDER}_SCOPES"`
}

// JobsConfig represents job queue configuration
type JobsConfig struct {
	// Enable job processing
	Enabled bool `yaml:"enabled" env:"JOBS_ENABLED" default:"true"`
	
	// Redis URL for job queue
	RedisURL string `yaml:"redis_url" env:"JOBS_REDIS_URL" default:"redis://localhost:6379/0"`
	
	// Number of worker goroutines
	Workers int `yaml:"workers" env:"JOBS_WORKERS" default:"4"`
	
	// Job retry attempts
	MaxRetries int `yaml:"max_retries" env:"JOBS_MAX_RETRIES" default:"3"`
	
	// Job timeout
	JobTimeout time.Duration `yaml:"job_timeout" env:"JOBS_JOB_TIMEOUT" default:"30m"`
	
	// Queue names and priorities
	Queues map[string]int `yaml:"queues" env:"JOBS_QUEUES"`
	
	// Cleanup configuration
	CleanupEnabled  bool          `yaml:"cleanup_enabled" env:"JOBS_CLEANUP_ENABLED" default:"true"`
	CleanupInterval time.Duration `yaml:"cleanup_interval" env:"JOBS_CLEANUP_INTERVAL" default:"1h"`
	RetentionPeriod time.Duration `yaml:"retention_period" env:"JOBS_RETENTION_PERIOD" default:"168h"` // 7 days
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	// Log level (debug, info, warn, error)
	Level string `yaml:"level" env:"LOG_LEVEL" default:"info"`
	
	// Log format (json, text)
	Format string `yaml:"format" env:"LOG_FORMAT" default:"json"`
	
	// Log output (stdout, stderr, file)
	Output string `yaml:"output" env:"LOG_OUTPUT" default:"stdout"`
	
	// Log file path (when output is file)
	FilePath string `yaml:"file_path" env:"LOG_FILE_PATH" default:"./logs/immich.log"`
	
	// Enable log rotation
	RotationEnabled bool `yaml:"rotation_enabled" env:"LOG_ROTATION_ENABLED" default:"true"`
	
	// Maximum log file size
	MaxSize int `yaml:"max_size" env:"LOG_MAX_SIZE" default:"100"` // MB
	
	// Maximum number of old log files
	MaxBackups int `yaml:"max_backups" env:"LOG_MAX_BACKUPS" default:"3"`
	
	// Maximum age of log files
	MaxAge int `yaml:"max_age" env:"LOG_MAX_AGE" default:"28"` // days
	
	// Compress old log files
	Compress bool `yaml:"compress" env:"LOG_COMPRESS" default:"true"`
}

// FeatureConfig represents feature flags
type FeatureConfig struct {
	// Enable machine learning features
	MachineLearningEnabled bool `yaml:"machine_learning_enabled" env:"FEATURE_MACHINE_LEARNING_ENABLED" default:"false"`
	
	// Enable face recognition
	FaceRecognitionEnabled bool `yaml:"face_recognition_enabled" env:"FEATURE_FACE_RECOGNITION_ENABLED" default:"false"`
	
	// Enable object detection
	ObjectDetectionEnabled bool `yaml:"object_detection_enabled" env:"FEATURE_OBJECT_DETECTION_ENABLED" default:"false"`
	
	// Enable CLIP search
	CLIPSearchEnabled bool `yaml:"clip_search_enabled" env:"FEATURE_CLIP_SEARCH_ENABLED" default:"false"`
	
	// Enable video transcoding
	VideoTranscodingEnabled bool `yaml:"video_transcoding_enabled" env:"FEATURE_VIDEO_TRANSCODING_ENABLED" default:"false"`
	
	// Enable thumbnail generation
	ThumbnailGenerationEnabled bool `yaml:"thumbnail_generation_enabled" env:"FEATURE_THUMBNAIL_GENERATION_ENABLED" default:"true"`
	
	// Enable EXIF extraction
	EXIFExtractionEnabled bool `yaml:"exif_extraction_enabled" env:"FEATURE_EXIF_EXTRACTION_ENABLED" default:"true"`
	
	// Enable duplicate detection
	DuplicateDetectionEnabled bool `yaml:"duplicate_detection_enabled" env:"FEATURE_DUPLICATE_DETECTION_ENABLED" default:"false"`
	
	// Enable backup/sync
	BackupSyncEnabled bool `yaml:"backup_sync_enabled" env:"FEATURE_BACKUP_SYNC_ENABLED" default:"true"`
	
	// Enable sharing
	SharingEnabled bool `yaml:"sharing_enabled" env:"FEATURE_SHARING_ENABLED" default:"true"`
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}
	
	// Set defaults
	setDefaults(config)
	
	// Load from file if provided
	if configPath != "" {
		if err := loadFromFile(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}
	
	// Override with environment variables
	if err := loadFromEnv(config); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}
	
	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return config, nil
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	config.Server = ServerConfig{
		Address:            "0.0.0.0:8080",
		GRPCAddress:        "0.0.0.0:9090",
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		IdleTimeout:        120 * time.Second,
		ShutdownTimeout:    30 * time.Second,
		CORSEnabled:        true,
		CORSAllowedOrigins: []string{"*"},
		RequestLogging:     true,
		MetricsEnabled:     true,
		MetricsPath:        "/metrics",
		HealthCheckEnabled: true,
		HealthCheckPath:    "/health",
	}
	
	config.Database = DatabaseConfig{
		URL:             "postgres://immich:immich@localhost:5432/immich?sslmode=disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		MaxConnLifetime: time.Hour,
		ConnectTimeout:  30 * time.Second,
		QueryTimeout:    30 * time.Second,
		LogQueries:      false,
		AutoMigrate:     true,
	}
	
	config.Storage = storage.GetDefaultStorageConfig()
	
	config.Auth = AuthConfig{
		JWTExpiry:                 24 * time.Hour,
		JWTRefreshExpiry:          168 * time.Hour,
		JWTIssuer:                 "immich-go-backend",
		RegistrationEnabled:       true,
		EmailVerificationRequired: false,
		PasswordMinLength:         8,
		SessionTimeout:            24 * time.Hour,
		LoginRateLimit:            5,
		LoginRateWindow:           15 * time.Minute,
	}
	
	config.Telemetry = telemetry.GetDefaultConfig()
	
	config.Jobs = JobsConfig{
		Enabled:         true,
		RedisURL:        "redis://localhost:6379/0",
		Workers:         4,
		MaxRetries:      3,
		JobTimeout:      30 * time.Minute,
		CleanupEnabled:  true,
		CleanupInterval: time.Hour,
		RetentionPeriod: 168 * time.Hour,
		Queues: map[string]int{
			"default":    1,
			"thumbnails": 2,
			"ml":         3,
			"backup":     1,
		},
	}
	
	config.Logging = LoggingConfig{
		Level:           "info",
		Format:          "json",
		Output:          "stdout",
		FilePath:        "./logs/immich.log",
		RotationEnabled: true,
		MaxSize:         100,
		MaxBackups:      3,
		MaxAge:          28,
		Compress:        true,
	}
	
	config.Features = FeatureConfig{
		ThumbnailGenerationEnabled: true,
		EXIFExtractionEnabled:      true,
		BackupSyncEnabled:          true,
		SharingEnabled:             true,
	}
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(config *Config, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	
	return yaml.Unmarshal(data, config)
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(config *Config) error {
	// This is a simplified implementation
	// In a real application, you might want to use a library like viper
	// or implement a more sophisticated environment variable mapping
	
	if val := os.Getenv("SERVER_ADDRESS"); val != "" {
		config.Server.Address = val
	}
	
	if val := os.Getenv("DATABASE_URL"); val != "" {
		config.Database.URL = val
	}
	
	if val := os.Getenv("AUTH_JWT_SECRET"); val != "" {
		config.Auth.JWTSecret = val
	}
	
	if val := os.Getenv("STORAGE_BACKEND"); val != "" {
		config.Storage.Backend = val
	}
	
	// Add more environment variable mappings as needed
	
	return nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate JWT secret
	if config.Auth.JWTSecret == "" {
		return fmt.Errorf("AUTH_JWT_SECRET is required")
	}
	
	// Validate database URL
	if config.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	
	// Validate storage configuration
	if err := storage.ValidateStorageConfig(config.Storage); err != nil {
		return fmt.Errorf("invalid storage configuration: %w", err)
	}
	
	// Validate server addresses
	if config.Server.Address == "" {
		return fmt.Errorf("SERVER_ADDRESS is required")
	}
	
	if config.Server.GRPCAddress == "" {
		return fmt.Errorf("SERVER_GRPC_ADDRESS is required")
	}
	
	return nil
}

// GetEnvOrDefault returns the value of an environment variable or a default value
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvOrDefaultInt returns the value of an environment variable as an integer or a default value
func GetEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetEnvOrDefaultBool returns the value of an environment variable as a boolean or a default value
func GetEnvOrDefaultBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// GetEnvOrDefaultDuration returns the value of an environment variable as a duration or a default value
func GetEnvOrDefaultDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// GetEnvOrDefaultStringSlice returns the value of an environment variable as a string slice or a default value
func GetEnvOrDefaultStringSlice(key string, defaultValue []string, separator string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, separator)
	}
	return defaultValue
}
