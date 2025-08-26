package systemconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
)

// Service handles system configuration operations
type Service struct {
	db            *sqlc.Queries
	defaultConfig *SystemConfig
}

// NewService creates a new system configuration service
func NewService(db *sqlc.Queries) *Service {
	return &Service{
		db:            db,
		defaultConfig: GetDefaultConfig(),
	}
}

// SystemConfig represents the complete system configuration
type SystemConfig struct {
	FFmpeg           FFmpegConfig           `json:"ffmpeg"`
	Job              JobConfig              `json:"job"`
	Library          LibraryConfig          `json:"library"`
	Logging          LoggingConfig          `json:"logging"`
	MachineLearning  MachineLearningConfig  `json:"machineLearning"`
	Map              MapConfig              `json:"map"`
	NewVersionCheck  NewVersionCheckConfig  `json:"newVersionCheck"`
	OAuth            OAuthConfig            `json:"oauth"`
	PasswordLogin    PasswordLoginConfig    `json:"passwordLogin"`
	ReverseGeocoding ReverseGeocodingConfig `json:"reverseGeocoding"`
	Server           ServerConfig           `json:"server"`
	StorageTemplate  StorageTemplateConfig  `json:"storageTemplate"`
	Thumbnail        ThumbnailConfig        `json:"thumbnail"`
	Trash            TrashConfig            `json:"trash"`
}

// FFmpegConfig represents FFmpeg configuration
type FFmpegConfig struct {
	CRF                 int      `json:"crf"`
	Threads             int      `json:"threads"`
	Preset              string   `json:"preset"`
	TargetVideoCodec    string   `json:"targetVideoCodec"`
	AcceptedVideoCodecs []string `json:"acceptedVideoCodecs"`
	TargetAudioCodec    string   `json:"targetAudioCodec"`
	AcceptedAudioCodecs []string `json:"acceptedAudioCodecs"`
	TargetResolution    string   `json:"targetResolution"`
	MaxBitrate          string   `json:"maxBitrate"`
	BFrames             int      `json:"bframes"`
	Refs                int      `json:"refs"`
	GopSize             int      `json:"gopSize"`
	NPL                 int      `json:"npl"`
	TemporalAQ          bool     `json:"temporalAQ"`
	CqMode              string   `json:"cqMode"`
	TwoPass             bool     `json:"twoPass"`
	PreferredHwDevice   string   `json:"preferredHwDevice"`
	Transcode           string   `json:"transcode"`
	AccelDecode         bool     `json:"accelDecode"`
	AccelEncode         bool     `json:"accelEncode"`
	ToneMappingMode     string   `json:"toneMappingMode"`
}

// JobConfig represents job configuration
type JobConfig struct {
	BackgroundTask           JobSettingsConfig `json:"backgroundTask"`
	ClipEncoding             JobSettingsConfig `json:"clipEncoding"`
	MetadataExtraction       JobSettingsConfig `json:"metadataExtraction"`
	ObjectTagging            JobSettingsConfig `json:"objectTagging"`
	RecognizeFaces           JobSettingsConfig `json:"recognizeFaces"`
	Search                   JobSettingsConfig `json:"search"`
	Sidecar                  JobSettingsConfig `json:"sidecar"`
	SmartSearch              JobSettingsConfig `json:"smartSearch"`
	StorageTemplateMigration JobSettingsConfig `json:"storageTemplateMigration"`
	ThumbnailGeneration      JobSettingsConfig `json:"thumbnailGeneration"`
	VideoConversion          JobSettingsConfig `json:"videoConversion"`
}

// JobSettingsConfig represents job settings
type JobSettingsConfig struct {
	Concurrency int `json:"concurrency"`
}

// LibraryConfig represents library configuration
type LibraryConfig struct {
	Scan  LibraryScanConfig  `json:"scan"`
	Watch LibraryWatchConfig `json:"watch"`
}

// LibraryScanConfig represents library scan configuration
type LibraryScanConfig struct {
	Enabled        bool   `json:"enabled"`
	CronExpression string `json:"cronExpression"`
}

// LibraryWatchConfig represents library watch configuration
type LibraryWatchConfig struct {
	Enabled bool `json:"enabled"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Enabled bool   `json:"enabled"`
	Level   string `json:"level"`
}

// MachineLearningConfig represents machine learning configuration
type MachineLearningConfig struct {
	Enabled           bool          `json:"enabled"`
	URL               string        `json:"url"`
	Clip              MLModelConfig `json:"clip"`
	FacialRecognition MLModelConfig `json:"facialRecognition"`
}

// MLModelConfig represents ML model configuration
type MLModelConfig struct {
	Enabled     bool    `json:"enabled"`
	ModelName   string  `json:"modelName"`
	MinScore    float64 `json:"minScore"`
	MaxDistance float64 `json:"maxDistance,omitempty"`
	MinFaces    int     `json:"minFaces,omitempty"`
}

// MapConfig represents map configuration
type MapConfig struct {
	Enabled    bool   `json:"enabled"`
	LightStyle string `json:"lightStyle"`
	DarkStyle  string `json:"darkStyle"`
}

// NewVersionCheckConfig represents version check configuration
type NewVersionCheckConfig struct {
	Enabled bool `json:"enabled"`
}

// OAuthConfig represents OAuth configuration
type OAuthConfig struct {
	Enabled               bool   `json:"enabled"`
	IssuerURL             string `json:"issuerUrl"`
	ClientID              string `json:"clientId"`
	ClientSecret          string `json:"clientSecret"`
	MobileOverrideEnabled bool   `json:"mobileOverrideEnabled"`
	MobileRedirectURI     string `json:"mobileRedirectUri"`
	Scope                 string `json:"scope"`
	StorageLabelClaim     string `json:"storageLabelClaim"`
	StorageQuotaClaim     string `json:"storageQuotaClaim"`
	DefaultStorageQuota   int    `json:"defaultStorageQuota"`
	ButtonText            string `json:"buttonText"`
	AutoRegister          bool   `json:"autoRegister"`
	AutoLaunch            bool   `json:"autoLaunch"`
}

// PasswordLoginConfig represents password login configuration
type PasswordLoginConfig struct {
	Enabled bool `json:"enabled"`
}

// ReverseGeocodingConfig represents reverse geocoding configuration
type ReverseGeocodingConfig struct {
	Enabled bool `json:"enabled"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	ExternalDomain   string `json:"externalDomain"`
	LoginPageMessage string `json:"loginPageMessage"`
}

// StorageTemplateConfig represents storage template configuration
type StorageTemplateConfig struct {
	Enabled                 bool   `json:"enabled"`
	HashVerificationEnabled bool   `json:"hashVerificationEnabled"`
	Template                string `json:"template"`
}

// ThumbnailConfig represents thumbnail configuration
type ThumbnailConfig struct {
	WebpSize   int    `json:"webpSize"`
	JpegSize   int    `json:"jpegSize"`
	Quality    int    `json:"quality"`
	ColorSpace string `json:"colorspace"`
}

// TrashConfig represents trash configuration
type TrashConfig struct {
	Enabled bool `json:"enabled"`
	Days    int  `json:"days"`
}

// GetSystemConfig retrieves the current system configuration
func (s *Service) GetSystemConfig(ctx context.Context) (*SystemConfig, error) {
	// Load config from database
	configs, err := s.db.GetAllSystemConfig(ctx)
	if err != nil {
		// If no config exists, return defaults
		return s.defaultConfig, nil
	}

	// Start with defaults
	config := GetDefaultConfig()

	// Apply stored configuration
	for _, cfg := range configs {
		s.applyConfigValue(config, cfg.Key, cfg.Value.String)
	}

	return config, nil
}

// UpdateSystemConfig updates the system configuration
func (s *Service) UpdateSystemConfig(ctx context.Context, updates map[string]interface{}) (*SystemConfig, error) {
	// Validate configuration
	if err := s.validateConfig(updates); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Update each configuration value
	for key, value := range updates {
		valueStr, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config value: %w", err)
		}

		err = s.db.UpsertSystemConfig(ctx, sqlc.UpsertSystemConfigParams{
			Key:   key,
			Value: string(valueStr),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update config: %w", err)
		}
	}

	// Return updated configuration
	return s.GetSystemConfig(ctx)
}

// GetConfigDefaults returns the default system configuration
func (s *Service) GetConfigDefaults() *SystemConfig {
	return GetDefaultConfig()
}

// GetStorageTemplateOptions returns available storage template variables
func (s *Service) GetStorageTemplateOptions() []TemplateOption {
	return []TemplateOption{
		{Variable: "{{y}}", Description: "Year (4 digits)"},
		{Variable: "{{M}}", Description: "Month (2 digits)"},
		{Variable: "{{d}}", Description: "Day (2 digits)"},
		{Variable: "{{h}}", Description: "Hour (2 digits)"},
		{Variable: "{{m}}", Description: "Minute (2 digits)"},
		{Variable: "{{s}}", Description: "Second (2 digits)"},
		{Variable: "{{filename}}", Description: "Original filename without extension"},
		{Variable: "{{ext}}", Description: "File extension"},
		{Variable: "{{album}}", Description: "Album name"},
		{Variable: "{{assetId}}", Description: "Asset ID"},
	}
}

// TemplateOption represents a storage template variable option
type TemplateOption struct {
	Variable    string `json:"variable"`
	Description string `json:"description"`
}

// applyConfigValue applies a stored configuration value to the config object
func (s *Service) applyConfigValue(config *SystemConfig, key string, value string) {
	// This would parse the key path and set the value in the config struct
	// For example: "ffmpeg.crf" would set config.FFmpeg.CRF
	// Implementation would use reflection or a switch statement
}

// validateConfig validates configuration updates
func (s *Service) validateConfig(updates map[string]interface{}) error {
	// Validate configuration values
	// Check ranges, formats, required fields etc.
	return nil
}

// GetDefaultConfig returns the default system configuration
func GetDefaultConfig() *SystemConfig {
	return &SystemConfig{
		FFmpeg: FFmpegConfig{
			CRF:                 23,
			Threads:             0,
			Preset:              "ultrafast",
			TargetVideoCodec:    "h264",
			AcceptedVideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
			TargetAudioCodec:    "aac",
			AcceptedAudioCodecs: []string{"aac", "mp3", "libopus"},
			TargetResolution:    "720",
			MaxBitrate:          "0",
			BFrames:             -1,
			Refs:                0,
			GopSize:             0,
			NPL:                 0,
			TemporalAQ:          false,
			CqMode:              "auto",
			TwoPass:             false,
			PreferredHwDevice:   "auto",
			Transcode:           "required",
			AccelDecode:         false,
			AccelEncode:         false,
			ToneMappingMode:     "hable",
		},
		Job: JobConfig{
			BackgroundTask:           JobSettingsConfig{Concurrency: 5},
			ClipEncoding:             JobSettingsConfig{Concurrency: 2},
			MetadataExtraction:       JobSettingsConfig{Concurrency: 5},
			ObjectTagging:            JobSettingsConfig{Concurrency: 2},
			RecognizeFaces:           JobSettingsConfig{Concurrency: 2},
			Search:                   JobSettingsConfig{Concurrency: 5},
			Sidecar:                  JobSettingsConfig{Concurrency: 5},
			SmartSearch:              JobSettingsConfig{Concurrency: 2},
			StorageTemplateMigration: JobSettingsConfig{Concurrency: 5},
			ThumbnailGeneration:      JobSettingsConfig{Concurrency: 5},
			VideoConversion:          JobSettingsConfig{Concurrency: 1},
		},
		Library: LibraryConfig{
			Scan: LibraryScanConfig{
				Enabled:        true,
				CronExpression: "0 0 * * *",
			},
			Watch: LibraryWatchConfig{
				Enabled: true,
			},
		},
		Logging: LoggingConfig{
			Enabled: true,
			Level:   "log",
		},
		MachineLearning: MachineLearningConfig{
			Enabled: false,
			URL:     "",
			Clip: MLModelConfig{
				Enabled:   true,
				ModelName: "ViT-B-32__openai",
				MinScore:  0.0,
			},
			FacialRecognition: MLModelConfig{
				Enabled:     true,
				ModelName:   "buffalo_l",
				MinScore:    0.7,
				MaxDistance: 0.5,
				MinFaces:    3,
			},
		},
		Map: MapConfig{
			Enabled:    true,
			LightStyle: "",
			DarkStyle:  "",
		},
		NewVersionCheck: NewVersionCheckConfig{
			Enabled: true,
		},
		OAuth: OAuthConfig{
			Enabled:               false,
			IssuerURL:             "",
			ClientID:              "",
			ClientSecret:          "",
			MobileOverrideEnabled: false,
			MobileRedirectURI:     "",
			Scope:                 "openid email profile",
			StorageLabelClaim:     "",
			StorageQuotaClaim:     "",
			DefaultStorageQuota:   0,
			ButtonText:            "Login with OAuth",
			AutoRegister:          true,
			AutoLaunch:            false,
		},
		PasswordLogin: PasswordLoginConfig{
			Enabled: true,
		},
		ReverseGeocoding: ReverseGeocodingConfig{
			Enabled: true,
		},
		Server: ServerConfig{
			ExternalDomain:   "",
			LoginPageMessage: "",
		},
		StorageTemplate: StorageTemplateConfig{
			Enabled:                 false,
			HashVerificationEnabled: true,
			Template:                "{{y}}/{{M}}/{{d}}/{{filename}}.{{ext}}",
		},
		Thumbnail: ThumbnailConfig{
			WebpSize:   250,
			JpegSize:   1440,
			Quality:    80,
			ColorSpace: "p3",
		},
		Trash: TrashConfig{
			Enabled: true,
			Days:    30,
		},
	}
}

// ServerInfo represents server information
type ServerInfo struct {
	Version       string   `json:"version"`
	LatestVersion string   `json:"latestVersion"`
	IsInitialized bool     `json:"isInitialized"`
	Features      Features `json:"features"`
}

// Features represents enabled server features
type Features struct {
	ClipEncode        bool `json:"clipEncode"`
	ConfigFile        bool `json:"configFile"`
	FacialRecognition bool `json:"facialRecognition"`
	Map               bool `json:"map"`
	MLEnabled         bool `json:"mlEnabled"`
	OAuth             bool `json:"oauth"`
	OAuthAutoLaunch   bool `json:"oauthAutoLaunch"`
	PasswordLogin     bool `json:"passwordLogin"`
	ReverseGeocoding  bool `json:"reverseGeocoding"`
	Search            bool `json:"search"`
	Sidecar           bool `json:"sidecar"`
	SmartSearch       bool `json:"smartSearch"`
	Trash             bool `json:"trash"`
}

// GetServerInfo retrieves server information
func (s *Service) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	config, err := s.GetSystemConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &ServerInfo{
		Version:       "1.0.0", // This would come from build info
		LatestVersion: "1.0.0", // This would be fetched from GitHub/update service
		IsInitialized: true,
		Features: Features{
			ClipEncode:        config.MachineLearning.Clip.Enabled,
			ConfigFile:        true,
			FacialRecognition: config.MachineLearning.FacialRecognition.Enabled,
			Map:               config.Map.Enabled,
			MLEnabled:         config.MachineLearning.Enabled,
			OAuth:             config.OAuth.Enabled,
			OAuthAutoLaunch:   config.OAuth.AutoLaunch,
			PasswordLogin:     config.PasswordLogin.Enabled,
			ReverseGeocoding:  config.ReverseGeocoding.Enabled,
			Search:            true,
			Sidecar:           true,
			SmartSearch:       config.MachineLearning.Clip.Enabled,
			Trash:             config.Trash.Enabled,
		},
	}, nil
}
