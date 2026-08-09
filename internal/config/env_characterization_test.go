package config

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef"

func loadWithEnv(t *testing.T, kv ...string) *Config {
	t.Helper()
	t.Setenv("AUTH_JWT_SECRET", testJWTSecret)
	for i := 0; i+1 < len(kv); i += 2 {
		t.Setenv(kv[i], kv[i+1])
	}

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

// TestHelmEnvironmentVariablesAreBound makes the Helm/binary contract
// executable. Any new literal environment variable in a chart template must
// be added to configEnvBindings or deliberately removed from the chart.
func TestHelmEnvironmentVariablesAreBound(t *testing.T) {
	templateFiles, err := filepath.Glob(filepath.Join("..", "..", "deploy", "helm", "immich-go-backend", "templates", "*.yaml"))
	if err != nil {
		t.Fatalf("glob Helm templates: %v", err)
	}
	if len(templateFiles) == 0 {
		t.Fatal("no Helm YAML templates found")
	}

	// Matches both ConfigMap/Secret keys ("  DATABASE_URL:") and explicit
	// container entries ("- name: DATABASE_URL"). Dynamic config.extra keys
	// are intentionally outside the chart's typed contract.
	envPattern := regexp.MustCompile(`(?m)^\s*(?:-\s+name:\s+)?([A-Z][A-Z0-9_]+)(?::|\s*$)`)
	emitted := make(map[string]struct{})
	for _, path := range templateFiles {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		for _, match := range envPattern.FindAllStringSubmatch(string(contents), -1) {
			emitted[match[1]] = struct{}{}
		}
	}

	bound := make(map[string]struct{}, len(configEnvBindings))
	for _, binding := range configEnvBindings {
		if _, duplicate := bound[binding.name]; duplicate {
			t.Errorf("duplicate config environment binding %q", binding.name)
		}
		bound[binding.name] = struct{}{}
	}

	var missing []string
	for name := range emitted {
		if _, ok := bound[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) != 0 {
		t.Fatalf("Helm emits environment variables the binary does not bind: %s", strings.Join(missing, ", "))
	}
}

func TestHelmSensitiveEnvironmentVariablesStayOutOfConfigMap(t *testing.T) {
	templateDir := filepath.Join("..", "..", "deploy", "helm", "immich-go-backend", "templates")
	configMap, err := os.ReadFile(filepath.Join(templateDir, "configmap.yaml"))
	if err != nil {
		t.Fatalf("read Helm ConfigMap template: %v", err)
	}
	secret, err := os.ReadFile(filepath.Join(templateDir, "secret.yaml"))
	if err != nil {
		t.Fatalf("read Helm Secret template: %v", err)
	}

	for _, name := range []string{
		"DATABASE_URL",
		"AUTH_JWT_SECRET",
		"JOBS_REDIS_URL",
		"S3_ACCESS_KEY_ID",
		"S3_SECRET_ACCESS_KEY",
	} {
		keyPattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(name) + `:`)
		if keyPattern.Match(configMap) {
			t.Errorf("sensitive environment variable %s must not be emitted by the Helm ConfigMap", name)
		}
		if !keyPattern.Match(secret) {
			t.Errorf("sensitive environment variable %s is missing from the Helm Secret", name)
		}
	}
}

// TestHelmEnvironmentOverrides verifies the destinations and parsers for the
// complete set of chart-managed settings. It deliberately uses non-default
// values so a silently ignored variable cannot pass.
func TestHelmEnvironmentOverrides(t *testing.T) {
	cfg := loadWithEnv(t,
		"SERVER_ADDRESS", "127.0.0.1:3101",
		"SERVER_GRPC_ADDRESS", "127.0.0.1:3102",
		"SERVER_READ_TIMEOUT", "41s",
		"SERVER_WRITE_TIMEOUT", "42s",
		"SERVER_IDLE_TIMEOUT", "43s",
		"SERVER_SHUTDOWN_TIMEOUT", "44s",
		"SERVER_CORS_ENABLED", "false",
		"SERVER_CORS_ALLOWED_ORIGINS", "https://photos.example, https://admin.example",
		"SERVER_REQUEST_LOGGING", "false",
		"SERVER_METRICS_ENABLED", "false",
		"SERVER_METRICS_PATH", "/internal/metrics",
		"SERVER_HEALTH_CHECK_ENABLED", "false",
		"SERVER_HEALTH_CHECK_PATH", "/internal/health",
		"TLS_ENABLED", "true",
		"TLS_CERT_FILE", "/tls/cert.pem",
		"TLS_KEY_FILE", "/tls/key.pem",
		"DATABASE_URL", "postgres://db.example/immich",
		"DATABASE_MAX_OPEN_CONNS", "31",
		"DATABASE_MIN_IDLE_CONNS", "7",
		"DATABASE_MAX_CONN_LIFETIME", "2h",
		"DATABASE_CONNECT_TIMEOUT", "17s",
		"DATABASE_QUERY_TIMEOUT", "19s",
		"DATABASE_LOG_QUERIES", "true",
		"IMMICH_DATABASE_AUTO_MIGRATE", "false",
		"AUTH_JWT_EXPIRY", "25h",
		"AUTH_JWT_REFRESH_EXPIRY", "169h",
		"AUTH_JWT_ISSUER", "photos.example",
		"AUTH_REGISTRATION_ENABLED", "false",
		"STORAGE_BACKEND", "s3",
		"STORAGE_LOCAL_ROOT", "/media",
		"UPLOAD_TEMP_DIR", "/media/tmp",
		"S3_ENABLED", "true",
		"S3_DIRECT_UPLOAD", "true",
		"S3_ENDPOINT", "https://s3.example",
		"S3_REGION", "eu-west-1",
		"S3_BUCKET", "photos",
		"S3_ACCESS_KEY_ID", "access",
		"S3_SECRET_ACCESS_KEY", "secret",
		"S3_USE_SSL", "false",
		"S3_PATH_PREFIX", "immich/assets",
		"S3_FORCE_PATH_STYLE", "true",
		"S3_PRESIGNED_URL_EXPIRY", "23m",
		"JOBS_ENABLED", "false",
		"JOBS_REDIS_URL", "redis://redis.example:6379/2",
		"JOBS_WORKERS", "13",
		"IMMICH_JOBS_RETRY_MAX_RETRIES", "14",
		"JOBS_JOB_TIMEOUT", "47m",
		"LOG_LEVEL", "debug",
		"LOG_FORMAT", "text",
		"LOG_OUTPUT", "stderr",
		"FEATURE_MACHINE_LEARNING_ENABLED", "true",
		"FEATURE_FACE_RECOGNITION_ENABLED", "true",
		"FEATURE_OBJECT_DETECTION_ENABLED", "true",
		"FEATURE_CLIP_SEARCH_ENABLED", "true",
		"FEATURE_VIDEO_TRANSCODING_ENABLED", "true",
		"FEATURE_THUMBNAIL_GENERATION_ENABLED", "false",
		"FEATURE_EXIF_EXTRACTION_ENABLED", "false",
		"FEATURE_DUPLICATE_DETECTION_ENABLED", "true",
		"FEATURE_BACKUP_SYNC_ENABLED", "false",
		"FEATURE_SHARING_ENABLED", "false",
		"MACHINE_LEARNING_ENABLED", "true",
		"MACHINE_LEARNING_URL", "http://ml:3003",
		"MACHINE_LEARNING_TIMEOUT", "71s",
		"IMMICH_WEBUI_DIR", "/opt/immich/web",
	)

	if got, want := cfg.Server, (ServerConfig{
		Address: "127.0.0.1:3101", GRPCAddress: "127.0.0.1:3102",
		ReadTimeout: 41 * time.Second, WriteTimeout: 42 * time.Second,
		IdleTimeout: 43 * time.Second, ShutdownTimeout: 44 * time.Second,
		CORSEnabled: false, CORSAllowedOrigins: []string{"https://photos.example", "https://admin.example"},
		RequestLogging: false, MetricsEnabled: false, MetricsPath: "/internal/metrics",
		HealthCheckEnabled: false, HealthCheckPath: "/internal/health",
		TLS: TLSConfig{Enabled: true, CertFile: "/tls/cert.pem", KeyFile: "/tls/key.pem"},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("Server config mismatch:\n got: %+v\nwant: %+v", got, want)
	}

	if got, want := cfg.Database, (DatabaseConfig{
		URL: "postgres://db.example/immich", MaxOpenConns: 31, MinIdleConns: 7,
		MaxConnLifetime: 2 * time.Hour, ConnectTimeout: 17 * time.Second,
		QueryTimeout: 19 * time.Second, LogQueries: true, AutoMigrate: false,
	}); got != want {
		t.Errorf("Database config mismatch:\n got: %+v\nwant: %+v", got, want)
	}

	if cfg.Auth.JWTExpiry != 25*time.Hour || cfg.Auth.JWTRefreshExpiry != 169*time.Hour ||
		cfg.Auth.JWTIssuer != "photos.example" || cfg.Auth.RegistrationEnabled {
		t.Errorf("Auth chart overrides not applied: %+v", cfg.Auth)
	}
	if cfg.Storage.Backend != "s3" || cfg.Storage.Local.RootPath != "/media" || cfg.Storage.Upload.TempDir != "/media/tmp" {
		t.Errorf("Storage chart overrides not applied: %+v", cfg.Storage)
	}
	if got := cfg.Storage.S3; !got.Enabled || !got.DirectUpload || got.Endpoint != "https://s3.example" ||
		got.Region != "eu-west-1" || got.Bucket != "photos" || got.AccessKeyID != "access" ||
		got.SecretAccessKey != "secret" || got.UseSSL || got.PathPrefix != "immich/assets" ||
		!got.ForcePathStyle || got.PresignedURLExpiry != 23*time.Minute {
		t.Errorf("S3 chart overrides not applied: %+v", got)
	}
	if cfg.Jobs.Enabled || cfg.Jobs.RedisURL != "redis://redis.example:6379/2" || cfg.Jobs.Workers != 13 ||
		cfg.Jobs.RetryMaxRetries != 14 || cfg.Jobs.JobTimeout != 47*time.Minute {
		t.Errorf("Jobs chart overrides not applied: %+v", cfg.Jobs)
	}
	if cfg.Logging.Level != "debug" || cfg.Logging.Format != "text" || cfg.Logging.Output != "stderr" {
		t.Errorf("Logging chart overrides not applied: %+v", cfg.Logging)
	}
	wantFeatures := FeatureConfig{
		MachineLearningEnabled: true, FaceRecognitionEnabled: true, ObjectDetectionEnabled: true,
		CLIPSearchEnabled: true, VideoTranscodingEnabled: true, ThumbnailGenerationEnabled: false,
		EXIFExtractionEnabled: false, DuplicateDetectionEnabled: true, BackupSyncEnabled: false,
		SharingEnabled: false,
	}
	if cfg.Features != wantFeatures {
		t.Errorf("Feature chart overrides not applied:\n got: %+v\nwant: %+v", cfg.Features, wantFeatures)
	}
	if !cfg.MachineLearning.Enabled || cfg.MachineLearning.URL != "http://ml:3003" || cfg.MachineLearning.Timeout != 71*time.Second {
		t.Errorf("Machine-learning chart overrides not applied: %+v", cfg.MachineLearning)
	}
	if cfg.WebUIDir != "/opt/immich/web" {
		t.Errorf("IMMICH_WEBUI_DIR not applied: %q", cfg.WebUIDir)
	}
}

func TestEnvironmentParsingErrorsAreActionable(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
		want  string
	}{
		{name: "boolean", env: "JOBS_ENABLED", value: "sometimes", want: "JOBS_ENABLED must be a boolean"},
		{name: "integer", env: "JOBS_WORKERS", value: "many", want: "JOBS_WORKERS must be an integer"},
		{name: "duration", env: "AUTH_JWT_EXPIRY", value: "tomorrow", want: "AUTH_JWT_EXPIRY must be a Go duration"},
		{name: "list", env: "SERVER_CORS_ALLOWED_ORIGINS", value: "https://one.example,,https://two.example", want: "SERVER_CORS_ALLOWED_ORIGINS must be a comma-separated list"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AUTH_JWT_SECRET", testJWTSecret)
			t.Setenv(tc.env, tc.value)
			_, err := LoadConfig("")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadConfig error = %v, want message containing %q", err, tc.want)
			}
		})
	}
}

func TestEnvironmentCanSelectS3Backend(t *testing.T) {
	cfg := loadWithEnv(t,
		"STORAGE_BACKEND", "s3",
		"S3_ENABLED", "true",
		"S3_BUCKET", "photos",
		"S3_REGION", "us-east-1",
		"S3_ACCESS_KEY_ID", "access",
		"S3_SECRET_ACCESS_KEY", "secret",
	)
	if cfg.Storage.Backend != "s3" || !cfg.Storage.S3.Enabled {
		t.Fatalf("S3 environment configuration not selected: %+v", cfg.Storage)
	}
}
