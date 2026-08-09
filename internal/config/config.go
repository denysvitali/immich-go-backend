package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"gopkg.in/yaml.v3"
)

// Config is the fully-resolved application configuration.
//
// Values come from exactly three sources, in this order:
//
//  1. setDefaults, which is the authoritative list of defaults.
//  2. the YAML document, if a path was given to LoadConfig.
//  3. loadFromEnv, which is the authoritative list of environment overrides.
//     Only the variables named in that function have any effect; there is no
//     reflective tag-based binding.
//
// LoadConfig then runs validateConfig, so an invalid combination fails at
// startup rather than at first use.
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

	// MachineLearning configures the external Immich ML service.
	// Off by default; also gated by Features.MachineLearningEnabled.
	MachineLearning MachineLearningConfig `yaml:"machine_learning"`

	// WebUIDir is the directory containing a static frontend build (e.g.
	// the Immich web dist). When empty, the API is exposed directly with
	// no HTML UI. Set via IMMICH_WEBUI_DIR or `webui_dir` in YAML.
	WebUIDir string `yaml:"webui_dir"`
}

// MachineLearningConfig controls the external Immich ML container.
type MachineLearningConfig struct {
	// Enabled allows the backend to call the ML service when feature flags permit.
	Enabled bool `yaml:"enabled"`

	// URL is the Immich ML base URL (e.g. http://immich-machine-learning:3003).
	URL string `yaml:"url"`

	// Timeout for each ML HTTP request.
	Timeout time.Duration `yaml:"timeout"`

	Clip               ClipMLConfig               `yaml:"clip"`
	FacialRecognition  FacialRecognitionMLConfig  `yaml:"facial_recognition"`
	DuplicateDetection DuplicateDetectionMLConfig `yaml:"duplicate_detection"`
}

// ClipMLConfig configures CLIP smart search.
type ClipMLConfig struct {
	Enabled     bool    `yaml:"enabled"`
	ModelName   string  `yaml:"model_name"`
	MaxDistance float64 `yaml:"max_distance"`
}

// FacialRecognitionMLConfig configures face detection/recognition.
type FacialRecognitionMLConfig struct {
	Enabled     bool    `yaml:"enabled"`
	ModelName   string  `yaml:"model_name"`
	MinScore    float64 `yaml:"min_score"`
	MaxDistance float64 `yaml:"max_distance"`
	MinFaces    int     `yaml:"min_faces"`
}

// DuplicateDetectionMLConfig configures CLIP-based near-duplicate grouping.
type DuplicateDetectionMLConfig struct {
	Enabled     bool    `yaml:"enabled"`
	MaxDistance float64 `yaml:"max_distance"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	// HTTP server address
	Address string `yaml:"address"`

	// gRPC server address
	GRPCAddress string `yaml:"grpc_address"`

	// Read timeout
	ReadTimeout time.Duration `yaml:"read_timeout"`

	// Write timeout
	WriteTimeout time.Duration `yaml:"write_timeout"`

	// Idle timeout
	IdleTimeout time.Duration `yaml:"idle_timeout"`

	// Shutdown timeout
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`

	// Enable CORS
	CORSEnabled bool `yaml:"cors_enabled"`

	// CORS allowed origins
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`

	// Enable request logging
	RequestLogging bool `yaml:"request_logging"`

	// Enable metrics endpoint
	MetricsEnabled bool `yaml:"metrics_enabled"`

	// Metrics endpoint path
	MetricsPath string `yaml:"metrics_path"`

	// Enable health check endpoint
	HealthCheckEnabled bool `yaml:"health_check_enabled"`

	// Health check endpoint path
	HealthCheckPath string `yaml:"health_check_path"`

	// TLS configuration
	TLS TLSConfig `yaml:"tls"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	// Enable TLS
	Enabled bool `yaml:"enabled"`

	// Certificate file path
	CertFile string `yaml:"cert_file"`

	// Private key file path
	KeyFile string `yaml:"key_file"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	// Database URL
	URL string `yaml:"url"`

	// Maximum number of open connections
	MaxOpenConns int `yaml:"max_open_conns"`

	// Minimum number of idle connections retained by pgxpool
	MinIdleConns int `yaml:"min_idle_conns"`

	// Maximum connection lifetime
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime"`

	// Connection timeout
	ConnectTimeout time.Duration `yaml:"connect_timeout"`

	// Query timeout
	QueryTimeout time.Duration `yaml:"query_timeout"`

	// Enable query logging
	LogQueries bool `yaml:"log_queries"`

	// Enable migrations
	AutoMigrate bool `yaml:"auto_migrate"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	// JWT secret key
	JWTSecret string `yaml:"jwt_secret"`

	// JWT token expiry
	JWTExpiry time.Duration `yaml:"jwt_expiry"`

	// JWT refresh token expiry
	JWTRefreshExpiry time.Duration `yaml:"jwt_refresh_expiry"` // 7 days

	// JWT issuer
	JWTIssuer string `yaml:"jwt_issuer"`

	// Enable user registration
	RegistrationEnabled bool `yaml:"registration_enabled"`

	// Require email verification
	EmailVerificationRequired bool `yaml:"email_verification_required"`

	// Password minimum length
	PasswordMinLength int `yaml:"password_min_length"`

	// Password complexity requirements
	PasswordRequireUppercase bool `yaml:"password_require_uppercase"`
	PasswordRequireLowercase bool `yaml:"password_require_lowercase"`
	PasswordRequireNumbers   bool `yaml:"password_require_numbers"`
	PasswordRequireSymbols   bool `yaml:"password_require_symbols"`

	// Session configuration
	SessionTimeout time.Duration `yaml:"session_timeout"`

	// Rate limiting
	LoginRateLimit  int           `yaml:"login_rate_limit"`
	LoginRateWindow time.Duration `yaml:"login_rate_window"`

	// OAuth configuration
	OAuth OAuthConfig `yaml:"oauth"`
}

// OAuthConfig represents OAuth configuration
type OAuthConfig struct {
	// Enable OAuth
	Enabled bool `yaml:"enabled"`

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
	Enabled bool `yaml:"enabled"`

	// Client ID
	ClientID string `yaml:"client_id"`

	// Client secret
	ClientSecret string `yaml:"client_secret"`

	// Redirect URL
	RedirectURL string `yaml:"redirect_url"`

	// Scopes
	Scopes []string `yaml:"scopes"`
}

// JobsConfig represents job queue configuration
type JobsConfig struct {
	// Enable job processing
	Enabled bool `yaml:"enabled"`

	// Redis URL for job queue
	RedisURL string `yaml:"redis_url"`

	// Number of worker goroutines
	Workers int `yaml:"workers"`

	// Job retry attempts
	RetryMaxRetries int `yaml:"retry_max_retries"`

	// Job timeout
	JobTimeout time.Duration `yaml:"job_timeout"`

	// Queue names and priorities
	Queues map[string]int `yaml:"queues"`

	// Cleanup configuration
	CleanupEnabled  bool          `yaml:"cleanup_enabled"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	RetentionPeriod time.Duration `yaml:"retention_period"` // 7 days
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	// Log level (debug, info, warn, error)
	Level string `yaml:"level"`

	// Log format (json, text)
	Format string `yaml:"format"`

	// Log output (stdout, stderr, file)
	Output string `yaml:"output"`

	// Log file path (when output is file)
	FilePath string `yaml:"file_path"`

	// Enable log rotation
	RotationEnabled bool `yaml:"rotation_enabled"`

	// Maximum log file size
	MaxSize int `yaml:"max_size"` // MB

	// Maximum number of old log files
	MaxBackups int `yaml:"max_backups"`

	// Maximum age of log files
	MaxAge int `yaml:"max_age"` // days

	// Compress old log files
	Compress bool `yaml:"compress"`
}

// FeatureConfig represents feature flags
type FeatureConfig struct {
	// Enable machine learning features
	MachineLearningEnabled bool `yaml:"machine_learning_enabled"`

	// Enable face recognition
	FaceRecognitionEnabled bool `yaml:"face_recognition_enabled"`

	// Enable object detection
	ObjectDetectionEnabled bool `yaml:"object_detection_enabled"`

	// Enable CLIP search
	CLIPSearchEnabled bool `yaml:"clip_search_enabled"`

	// Enable video transcoding
	VideoTranscodingEnabled bool `yaml:"video_transcoding_enabled"`

	// Enable thumbnail generation
	ThumbnailGenerationEnabled bool `yaml:"thumbnail_generation_enabled"`

	// Enable EXIF extraction
	EXIFExtractionEnabled bool `yaml:"exif_extraction_enabled"`

	// Enable duplicate detection
	DuplicateDetectionEnabled bool `yaml:"duplicate_detection_enabled"`

	// Enable backup/sync
	BackupSyncEnabled bool `yaml:"backup_sync_enabled"`

	// Enable sharing
	SharingEnabled bool `yaml:"sharing_enabled"`
}

const minJWTSecretBytes = 32

// LoadConfig loads configuration from file and environment variables and
// validates that it is safe and complete enough to start the server.
func LoadConfig(configPath string) (*Config, error) {
	config, err := LoadConfigWithoutValidation(configPath)
	if err != nil {
		return nil, err
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// LoadConfigWithoutValidation loads and resolves configuration without
// applying server-runtime validation. It is intended for utility commands
// such as version, migrations, and OpenAPI coverage that do not sign tokens or
// start network listeners. Server startup must use LoadConfig.
func LoadConfigWithoutValidation(configPath string) (*Config, error) {
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
		MetricsEnabled:     false,
		MetricsPath:        "/metrics",
		HealthCheckEnabled: true,
		HealthCheckPath:    "/health",
	}

	config.Database = DatabaseConfig{
		URL:             "postgres://immich:immich@localhost:5432/immich?sslmode=disable",
		MaxOpenConns:    25,
		MinIdleConns:    5,
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
		RetryMaxRetries: 10,
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
		// ML-related features stay off until explicitly enabled.
		MachineLearningEnabled:     false,
		FaceRecognitionEnabled:     false,
		ObjectDetectionEnabled:     false,
		CLIPSearchEnabled:          false,
		DuplicateDetectionEnabled:  false,
		ThumbnailGenerationEnabled: true,
		EXIFExtractionEnabled:      true,
		BackupSyncEnabled:          true,
		SharingEnabled:             true,
	}

	config.MachineLearning = MachineLearningConfig{
		Enabled: false,
		URL:     "",
		Timeout: 60 * time.Second,
		Clip: ClipMLConfig{
			Enabled:     true,
			ModelName:   "ViT-B-32__openai",
			MaxDistance: 0.6,
		},
		FacialRecognition: FacialRecognitionMLConfig{
			Enabled:     true,
			ModelName:   "buffalo_l",
			MinScore:    0.7,
			MaxDistance: 0.5,
			MinFaces:    3,
		},
		DuplicateDetection: DuplicateDetectionMLConfig{
			Enabled:     true,
			MaxDistance: 0.01,
		},
	}
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(config *Config, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// A missing config file is not fatal: env-driven deployments
		// (e.g. the Fly demo) supply everything via defaults + env vars
		// and ship no config.yaml in the image. Only a present-but-broken
		// file is an error.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(config); err != nil {
		return fmt.Errorf("decode configuration (unknown fields are not allowed): %w", err)
	}
	return nil
}

// configEnvBinding is one environment variable in the public runtime contract.
// Keeping the name and setter together lets tests prove that every literal
// variable emitted by the Helm chart is understood by the binary.
type configEnvBinding struct {
	name  string
	apply func(*Config, string) error
}

func stringEnv(name string, set func(*Config, string)) configEnvBinding {
	return configEnvBinding{
		name: name,
		apply: func(config *Config, value string) error {
			set(config, value)
			return nil
		},
	}
}

func boolEnv(name string, set func(*Config, bool)) configEnvBinding {
	return configEnvBinding{
		name: name,
		apply: func(config *Config, value string) error {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("%s must be a boolean: %w", name, err)
			}
			set(config, parsed)
			return nil
		},
	}
}

func intEnv(name string, set func(*Config, int)) configEnvBinding {
	return configEnvBinding{
		name: name,
		apply: func(config *Config, value string) error {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("%s must be an integer: %w", name, err)
			}
			set(config, parsed)
			return nil
		},
	}
}

func durationEnv(name string, set func(*Config, time.Duration)) configEnvBinding {
	return configEnvBinding{
		name: name,
		apply: func(config *Config, value string) error {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("%s must be a Go duration such as 30s or 1h: %w", name, err)
			}
			set(config, parsed)
			return nil
		},
	}
}

func stringListEnv(name string, set func(*Config, []string)) configEnvBinding {
	return configEnvBinding{
		name: name,
		apply: func(config *Config, value string) error {
			if strings.TrimSpace(value) == "" {
				set(config, nil)
				return nil
			}

			items := strings.Split(value, ",")
			for i := range items {
				items[i] = strings.TrimSpace(items[i])
				if items[i] == "" {
					return fmt.Errorf("%s must be a comma-separated list without empty entries", name)
				}
			}
			set(config, items)
			return nil
		},
	}
}

var configEnvBindings = []configEnvBinding{
	stringEnv("SERVER_ADDRESS", func(c *Config, v string) { c.Server.Address = v }),
	stringEnv("SERVER_GRPC_ADDRESS", func(c *Config, v string) { c.Server.GRPCAddress = v }),
	durationEnv("SERVER_READ_TIMEOUT", func(c *Config, v time.Duration) { c.Server.ReadTimeout = v }),
	durationEnv("SERVER_WRITE_TIMEOUT", func(c *Config, v time.Duration) { c.Server.WriteTimeout = v }),
	durationEnv("SERVER_IDLE_TIMEOUT", func(c *Config, v time.Duration) { c.Server.IdleTimeout = v }),
	durationEnv("SERVER_SHUTDOWN_TIMEOUT", func(c *Config, v time.Duration) { c.Server.ShutdownTimeout = v }),
	boolEnv("SERVER_CORS_ENABLED", func(c *Config, v bool) { c.Server.CORSEnabled = v }),
	stringListEnv("SERVER_CORS_ALLOWED_ORIGINS", func(c *Config, v []string) { c.Server.CORSAllowedOrigins = v }),
	boolEnv("SERVER_REQUEST_LOGGING", func(c *Config, v bool) { c.Server.RequestLogging = v }),
	boolEnv("SERVER_METRICS_ENABLED", func(c *Config, v bool) { c.Server.MetricsEnabled = v }),
	stringEnv("SERVER_METRICS_PATH", func(c *Config, v string) { c.Server.MetricsPath = v }),
	boolEnv("SERVER_HEALTH_CHECK_ENABLED", func(c *Config, v bool) { c.Server.HealthCheckEnabled = v }),
	stringEnv("SERVER_HEALTH_CHECK_PATH", func(c *Config, v string) { c.Server.HealthCheckPath = v }),
	boolEnv("TLS_ENABLED", func(c *Config, v bool) { c.Server.TLS.Enabled = v }),
	stringEnv("TLS_CERT_FILE", func(c *Config, v string) { c.Server.TLS.CertFile = v }),
	stringEnv("TLS_KEY_FILE", func(c *Config, v string) { c.Server.TLS.KeyFile = v }),

	stringEnv("DATABASE_URL", func(c *Config, v string) { c.Database.URL = v }),
	intEnv("DATABASE_MAX_OPEN_CONNS", func(c *Config, v int) { c.Database.MaxOpenConns = v }),
	intEnv("DATABASE_MIN_IDLE_CONNS", func(c *Config, v int) { c.Database.MinIdleConns = v }),
	{
		name: "DATABASE_MAX_IDLE_CONNS",
		apply: func(*Config, string) error {
			return fmt.Errorf("DATABASE_MAX_IDLE_CONNS is unsupported because pgxpool has no maximum-idle setting; use DATABASE_MIN_IDLE_CONNS")
		},
	},
	durationEnv("DATABASE_MAX_CONN_LIFETIME", func(c *Config, v time.Duration) { c.Database.MaxConnLifetime = v }),
	durationEnv("DATABASE_CONNECT_TIMEOUT", func(c *Config, v time.Duration) { c.Database.ConnectTimeout = v }),
	durationEnv("DATABASE_QUERY_TIMEOUT", func(c *Config, v time.Duration) { c.Database.QueryTimeout = v }),
	boolEnv("DATABASE_LOG_QUERIES", func(c *Config, v bool) { c.Database.LogQueries = v }),
	boolEnv("IMMICH_DATABASE_AUTO_MIGRATE", func(c *Config, v bool) { c.Database.AutoMigrate = v }),

	stringEnv("AUTH_JWT_SECRET", func(c *Config, v string) { c.Auth.JWTSecret = v }),
	durationEnv("AUTH_JWT_EXPIRY", func(c *Config, v time.Duration) { c.Auth.JWTExpiry = v }),
	durationEnv("AUTH_JWT_REFRESH_EXPIRY", func(c *Config, v time.Duration) { c.Auth.JWTRefreshExpiry = v }),
	stringEnv("AUTH_JWT_ISSUER", func(c *Config, v string) { c.Auth.JWTIssuer = v }),
	boolEnv("AUTH_REGISTRATION_ENABLED", func(c *Config, v bool) { c.Auth.RegistrationEnabled = v }),
	intEnv("AUTH_LOGIN_RATE_LIMIT", func(c *Config, v int) { c.Auth.LoginRateLimit = v }),
	durationEnv("AUTH_LOGIN_RATE_WINDOW", func(c *Config, v time.Duration) { c.Auth.LoginRateWindow = v }),

	stringEnv("STORAGE_BACKEND", func(c *Config, v string) { c.Storage.Backend = v }),
	stringEnv("STORAGE_LOCAL_ROOT", func(c *Config, v string) { c.Storage.Local.RootPath = v }),
	stringEnv("UPLOAD_TEMP_DIR", func(c *Config, v string) { c.Storage.Upload.TempDir = v }),
	boolEnv("S3_ENABLED", func(c *Config, v bool) { c.Storage.S3.Enabled = v }),
	boolEnv("S3_DIRECT_UPLOAD", func(c *Config, v bool) { c.Storage.S3.DirectUpload = v }),
	stringEnv("S3_ENDPOINT", func(c *Config, v string) { c.Storage.S3.Endpoint = v }),
	stringEnv("S3_REGION", func(c *Config, v string) { c.Storage.S3.Region = v }),
	stringEnv("S3_BUCKET", func(c *Config, v string) { c.Storage.S3.Bucket = v }),
	stringEnv("S3_ACCESS_KEY_ID", func(c *Config, v string) { c.Storage.S3.AccessKeyID = v }),
	stringEnv("S3_SECRET_ACCESS_KEY", func(c *Config, v string) { c.Storage.S3.SecretAccessKey = v }),
	boolEnv("S3_USE_SSL", func(c *Config, v bool) { c.Storage.S3.UseSSL = v }),
	stringEnv("S3_PATH_PREFIX", func(c *Config, v string) { c.Storage.S3.PathPrefix = v }),
	boolEnv("S3_FORCE_PATH_STYLE", func(c *Config, v bool) { c.Storage.S3.ForcePathStyle = v }),
	durationEnv("S3_PRESIGNED_URL_EXPIRY", func(c *Config, v time.Duration) { c.Storage.S3.PresignedURLExpiry = v }),

	boolEnv("JOBS_ENABLED", func(c *Config, v bool) { c.Jobs.Enabled = v }),
	stringEnv("JOBS_REDIS_URL", func(c *Config, v string) { c.Jobs.RedisURL = v }),
	intEnv("JOBS_WORKERS", func(c *Config, v int) { c.Jobs.Workers = v }),
	intEnv("IMMICH_JOBS_RETRY_MAX_RETRIES", func(c *Config, v int) { c.Jobs.RetryMaxRetries = v }),
	durationEnv("JOBS_JOB_TIMEOUT", func(c *Config, v time.Duration) { c.Jobs.JobTimeout = v }),

	stringEnv("LOG_LEVEL", func(c *Config, v string) { c.Logging.Level = v }),
	stringEnv("LOG_FORMAT", func(c *Config, v string) { c.Logging.Format = v }),
	stringEnv("LOG_OUTPUT", func(c *Config, v string) { c.Logging.Output = v }),

	boolEnv("FEATURE_MACHINE_LEARNING_ENABLED", func(c *Config, v bool) { c.Features.MachineLearningEnabled = v }),
	boolEnv("FEATURE_FACE_RECOGNITION_ENABLED", func(c *Config, v bool) { c.Features.FaceRecognitionEnabled = v }),
	boolEnv("FEATURE_OBJECT_DETECTION_ENABLED", func(c *Config, v bool) { c.Features.ObjectDetectionEnabled = v }),
	boolEnv("FEATURE_CLIP_SEARCH_ENABLED", func(c *Config, v bool) { c.Features.CLIPSearchEnabled = v }),
	boolEnv("FEATURE_VIDEO_TRANSCODING_ENABLED", func(c *Config, v bool) { c.Features.VideoTranscodingEnabled = v }),
	boolEnv("FEATURE_THUMBNAIL_GENERATION_ENABLED", func(c *Config, v bool) { c.Features.ThumbnailGenerationEnabled = v }),
	boolEnv("FEATURE_EXIF_EXTRACTION_ENABLED", func(c *Config, v bool) { c.Features.EXIFExtractionEnabled = v }),
	boolEnv("FEATURE_DUPLICATE_DETECTION_ENABLED", func(c *Config, v bool) { c.Features.DuplicateDetectionEnabled = v }),
	boolEnv("FEATURE_BACKUP_SYNC_ENABLED", func(c *Config, v bool) { c.Features.BackupSyncEnabled = v }),
	boolEnv("FEATURE_SHARING_ENABLED", func(c *Config, v bool) { c.Features.SharingEnabled = v }),

	boolEnv("MACHINE_LEARNING_ENABLED", func(c *Config, v bool) { c.MachineLearning.Enabled = v }),
	stringEnv("MACHINE_LEARNING_URL", func(c *Config, v string) { c.MachineLearning.URL = v }),
	durationEnv("MACHINE_LEARNING_TIMEOUT", func(c *Config, v time.Duration) { c.MachineLearning.Timeout = v }),
	stringEnv("IMMICH_WEBUI_DIR", func(c *Config, v string) { c.WebUIDir = v }),
}

// loadFromEnv applies environment overrides. A present variable is always
// authoritative, including an explicitly empty string. Invalid typed values
// are rejected instead of silently falling back to a different configuration.
func loadFromEnv(config *Config) error {
	for _, binding := range configEnvBindings {
		value, ok := os.LookupEnv(binding.name)
		if !ok {
			continue
		}
		if err := binding.apply(config, value); err != nil {
			return err
		}
	}
	return nil
}

// MLActive reports whether the external ML service should be used at all.
func (c *Config) MLActive() bool {
	if c == nil {
		return false
	}
	return c.Features.MachineLearningEnabled && c.MachineLearning.Enabled && c.MachineLearning.URL != ""
}

// CLIPActive reports whether CLIP smart search / encoding should run.
func (c *Config) CLIPActive() bool {
	return c.MLActive() && c.Features.CLIPSearchEnabled && c.MachineLearning.Clip.Enabled
}

// FaceRecognitionActive reports whether face detection/recognition should run.
func (c *Config) FaceRecognitionActive() bool {
	return c.MLActive() && c.Features.FaceRecognitionEnabled && c.MachineLearning.FacialRecognition.Enabled
}

// DuplicateDetectionActive reports whether duplicate detection jobs should run.
func (c *Config) DuplicateDetectionActive() bool {
	if c == nil {
		return false
	}
	if c.Features.DuplicateDetectionEnabled {
		return true
	}
	// CLIP-based near-duplicates also require CLIP.
	return c.CLIPActive() && c.MachineLearning.DuplicateDetection.Enabled
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if err := validateJWTSecret(config.Auth.JWTSecret); err != nil {
		return err
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
	if config.Server.TLS.Enabled && (config.Server.TLS.CertFile == "" || config.Server.TLS.KeyFile == "") {
		return fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE are required when TLS is enabled")
	}

	return nil
}

func validateJWTSecret(secret string) error {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return fmt.Errorf("AUTH_JWT_SECRET is required")
	}

	// Normalize separators so common variants such as CHANGE_ME,
	// change-me, and "change me" cannot be made to look production-safe by
	// padding or repetition.
	normalized := strings.ToLower(trimmed)
	compact := strings.NewReplacer("-", "", "_", "", " ", "").Replace(normalized)
	if normalized == "your-super-secret-jwt-key-change-this-in-production" ||
		strings.Contains(compact, "changeme") ||
		strings.Contains(compact, "replaceme") ||
		strings.Contains(compact, "yourjwtsecret") {
		return fmt.Errorf("AUTH_JWT_SECRET must not be a placeholder value")
	}

	if len(trimmed) < minJWTSecretBytes {
		return fmt.Errorf("AUTH_JWT_SECRET must be at least %d bytes", minJWTSecretBytes)
	}

	return nil
}
