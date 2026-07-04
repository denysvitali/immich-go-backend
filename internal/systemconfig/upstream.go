package systemconfig

// This file mirrors the upstream Immich v2.4.0 SystemConfigDto
// (server/src/config.ts + open-api SystemConfigDto). The web UI reads every
// key of this object on the admin settings page, so the JSON shape must match
// upstream exactly — all sections present, camelCase keys, no omissions.

// Dto is the complete system configuration exchanged with the web UI.
type Dto struct {
	Backup           BackupDto           `json:"backup"`
	FFmpeg           FFmpegDto           `json:"ffmpeg"`
	Image            ImageDto            `json:"image"`
	Job              JobDto              `json:"job"`
	Library          LibraryDto          `json:"library"`
	Logging          LoggingDto          `json:"logging"`
	MachineLearning  MachineLearningDto  `json:"machineLearning"`
	Map              MapDto              `json:"map"`
	Metadata         MetadataDto         `json:"metadata"`
	NewVersionCheck  NewVersionCheckDto  `json:"newVersionCheck"`
	NightlyTasks     NightlyTasksDto     `json:"nightlyTasks"`
	Notifications    NotificationsDto    `json:"notifications"`
	OAuth            OAuthDto            `json:"oauth"`
	PasswordLogin    PasswordLoginDto    `json:"passwordLogin"`
	ReverseGeocoding ReverseGeocodingDto `json:"reverseGeocoding"`
	Server           ServerDto           `json:"server"`
	StorageTemplate  StorageTemplateDto  `json:"storageTemplate"`
	Templates        TemplatesDto        `json:"templates"`
	Theme            ThemeDto            `json:"theme"`
	Trash            TrashDto            `json:"trash"`
	User             UserDto             `json:"user"`
}

type BackupDto struct {
	Database DatabaseBackupDto `json:"database"`
}

type DatabaseBackupDto struct {
	Enabled        bool   `json:"enabled"`
	CronExpression string `json:"cronExpression"`
	KeepLastAmount int    `json:"keepLastAmount"`
}

type FFmpegDto struct {
	CRF                 int      `json:"crf"`
	Threads             int      `json:"threads"`
	Preset              string   `json:"preset"`
	TargetVideoCodec    string   `json:"targetVideoCodec"`
	AcceptedVideoCodecs []string `json:"acceptedVideoCodecs"`
	TargetAudioCodec    string   `json:"targetAudioCodec"`
	AcceptedAudioCodecs []string `json:"acceptedAudioCodecs"`
	AcceptedContainers  []string `json:"acceptedContainers"`
	TargetResolution    string   `json:"targetResolution"`
	MaxBitrate          string   `json:"maxBitrate"`
	BFrames             int      `json:"bframes"`
	Refs                int      `json:"refs"`
	GopSize             int      `json:"gopSize"`
	TemporalAQ          bool     `json:"temporalAQ"`
	CQMode              string   `json:"cqMode"`
	TwoPass             bool     `json:"twoPass"`
	PreferredHwDevice   string   `json:"preferredHwDevice"`
	Transcode           string   `json:"transcode"`
	Accel               string   `json:"accel"`
	AccelDecode         bool     `json:"accelDecode"`
	Tonemap             string   `json:"tonemap"`
}

type ImageDto struct {
	Thumbnail       ImageOptionsDto  `json:"thumbnail"`
	Preview         ImageOptionsDto  `json:"preview"`
	Colorspace      string           `json:"colorspace"`
	ExtractEmbedded bool             `json:"extractEmbedded"`
	Fullsize        FullsizeImageDto `json:"fullsize"`
}

type ImageOptionsDto struct {
	Format  string `json:"format"`
	Size    int    `json:"size"`
	Quality int    `json:"quality"`
}

type FullsizeImageDto struct {
	Enabled bool   `json:"enabled"`
	Format  string `json:"format"`
	Quality int    `json:"quality"`
}

type JobDto struct {
	BackgroundTask      JobSettingsDto `json:"backgroundTask"`
	SmartSearch         JobSettingsDto `json:"smartSearch"`
	MetadataExtraction  JobSettingsDto `json:"metadataExtraction"`
	FaceDetection       JobSettingsDto `json:"faceDetection"`
	Search              JobSettingsDto `json:"search"`
	Sidecar             JobSettingsDto `json:"sidecar"`
	Library             JobSettingsDto `json:"library"`
	Migration           JobSettingsDto `json:"migration"`
	ThumbnailGeneration JobSettingsDto `json:"thumbnailGeneration"`
	VideoConversion     JobSettingsDto `json:"videoConversion"`
	Notifications       JobSettingsDto `json:"notifications"`
	OCR                 JobSettingsDto `json:"ocr"`
	Workflow            JobSettingsDto `json:"workflow"`
}

type JobSettingsDto struct {
	Concurrency int `json:"concurrency"`
}

type LibraryDto struct {
	Scan  LibraryScanDto  `json:"scan"`
	Watch LibraryWatchDto `json:"watch"`
}

type LibraryScanDto struct {
	Enabled        bool   `json:"enabled"`
	CronExpression string `json:"cronExpression"`
}

type LibraryWatchDto struct {
	Enabled bool `json:"enabled"`
}

type LoggingDto struct {
	Enabled bool   `json:"enabled"`
	Level   string `json:"level"`
}

type MachineLearningDto struct {
	Enabled            bool                  `json:"enabled"`
	URLs               []string              `json:"urls"`
	AvailabilityChecks AvailabilityChecksDto `json:"availabilityChecks"`
	Clip               ClipDto               `json:"clip"`
	DuplicateDetection DuplicateDetectionDto `json:"duplicateDetection"`
	FacialRecognition  FacialRecognitionDto  `json:"facialRecognition"`
	OCR                OCRDto                `json:"ocr"`
}

type AvailabilityChecksDto struct {
	Enabled  bool `json:"enabled"`
	Timeout  int  `json:"timeout"`
	Interval int  `json:"interval"`
}

type ClipDto struct {
	Enabled   bool   `json:"enabled"`
	ModelName string `json:"modelName"`
}

type DuplicateDetectionDto struct {
	Enabled     bool    `json:"enabled"`
	MaxDistance float64 `json:"maxDistance"`
}

type FacialRecognitionDto struct {
	Enabled     bool    `json:"enabled"`
	ModelName   string  `json:"modelName"`
	MinScore    float64 `json:"minScore"`
	MinFaces    int     `json:"minFaces"`
	MaxDistance float64 `json:"maxDistance"`
}

type OCRDto struct {
	Enabled             bool    `json:"enabled"`
	ModelName           string  `json:"modelName"`
	MinDetectionScore   float64 `json:"minDetectionScore"`
	MinRecognitionScore float64 `json:"minRecognitionScore"`
	MaxResolution       int     `json:"maxResolution"`
}

type MapDto struct {
	Enabled    bool   `json:"enabled"`
	LightStyle string `json:"lightStyle"`
	DarkStyle  string `json:"darkStyle"`
}

type MetadataDto struct {
	Faces MetadataFacesDto `json:"faces"`
}

type MetadataFacesDto struct {
	Import bool `json:"import"`
}

type NewVersionCheckDto struct {
	Enabled bool `json:"enabled"`
}

type NightlyTasksDto struct {
	StartTime         string `json:"startTime"`
	DatabaseCleanup   bool   `json:"databaseCleanup"`
	MissingThumbnails bool   `json:"missingThumbnails"`
	ClusterNewFaces   bool   `json:"clusterNewFaces"`
	GenerateMemories  bool   `json:"generateMemories"`
	SyncQuotaUsage    bool   `json:"syncQuotaUsage"`
}

type NotificationsDto struct {
	SMTP SMTPDto `json:"smtp"`
}

type SMTPDto struct {
	Enabled   bool             `json:"enabled"`
	From      string           `json:"from"`
	ReplyTo   string           `json:"replyTo"`
	Transport SMTPTransportDto `json:"transport"`
}

type SMTPTransportDto struct {
	IgnoreCert bool   `json:"ignoreCert"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Secure     bool   `json:"secure"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type OAuthDto struct {
	AutoLaunch              bool   `json:"autoLaunch"`
	AutoRegister            bool   `json:"autoRegister"`
	ButtonText              string `json:"buttonText"`
	ClientID                string `json:"clientId"`
	ClientSecret            string `json:"clientSecret"`
	DefaultStorageQuota     *int64 `json:"defaultStorageQuota"`
	Enabled                 bool   `json:"enabled"`
	IssuerURL               string `json:"issuerUrl"`
	MobileOverrideEnabled   bool   `json:"mobileOverrideEnabled"`
	MobileRedirectURI       string `json:"mobileRedirectUri"`
	Scope                   string `json:"scope"`
	SigningAlgorithm        string `json:"signingAlgorithm"`
	ProfileSigningAlgorithm string `json:"profileSigningAlgorithm"`
	TokenEndpointAuthMethod string `json:"tokenEndpointAuthMethod"`
	Timeout                 int    `json:"timeout"`
	StorageLabelClaim       string `json:"storageLabelClaim"`
	StorageQuotaClaim       string `json:"storageQuotaClaim"`
	RoleClaim               string `json:"roleClaim"`
}

type PasswordLoginDto struct {
	Enabled bool `json:"enabled"`
}

type ReverseGeocodingDto struct {
	Enabled bool `json:"enabled"`
}

type ServerDto struct {
	ExternalDomain   string `json:"externalDomain"`
	LoginPageMessage string `json:"loginPageMessage"`
	PublicUsers      bool   `json:"publicUsers"`
}

type StorageTemplateDto struct {
	Enabled                 bool   `json:"enabled"`
	HashVerificationEnabled bool   `json:"hashVerificationEnabled"`
	Template                string `json:"template"`
}

type TemplatesDto struct {
	Email EmailTemplatesDto `json:"email"`
}

type EmailTemplatesDto struct {
	WelcomeTemplate     string `json:"welcomeTemplate"`
	AlbumInviteTemplate string `json:"albumInviteTemplate"`
	AlbumUpdateTemplate string `json:"albumUpdateTemplate"`
}

type ThemeDto struct {
	CustomCSS string `json:"customCss"`
}

type TrashDto struct {
	Enabled bool `json:"enabled"`
	Days    int  `json:"days"`
}

type UserDto struct {
	DeleteDelay int `json:"deleteDelay"`
}

// DefaultDto returns the upstream v2.4.0 default configuration
// (server/src/config.ts `defaults`).
func DefaultDto() Dto {
	return Dto{
		Backup: BackupDto{
			Database: DatabaseBackupDto{
				Enabled:        true,
				CronExpression: "0 02 * * *",
				KeepLastAmount: 14,
			},
		},
		FFmpeg: FFmpegDto{
			CRF:                 23,
			Threads:             0,
			Preset:              "ultrafast",
			TargetVideoCodec:    "h264",
			AcceptedVideoCodecs: []string{"h264"},
			TargetAudioCodec:    "aac",
			AcceptedAudioCodecs: []string{"aac", "mp3", "libopus"},
			AcceptedContainers:  []string{"mov", "ogg", "webm"},
			TargetResolution:    "720",
			MaxBitrate:          "0",
			BFrames:             -1,
			Refs:                0,
			GopSize:             0,
			TemporalAQ:          false,
			CQMode:              "auto",
			TwoPass:             false,
			PreferredHwDevice:   "auto",
			Transcode:           "required",
			Accel:               "disabled",
			AccelDecode:         false,
			Tonemap:             "hable",
		},
		Image: ImageDto{
			Thumbnail:       ImageOptionsDto{Format: "webp", Size: 250, Quality: 80},
			Preview:         ImageOptionsDto{Format: "jpeg", Size: 1440, Quality: 80},
			Colorspace:      "p3",
			ExtractEmbedded: false,
			Fullsize:        FullsizeImageDto{Enabled: false, Format: "jpeg", Quality: 80},
		},
		Job: JobDto{
			BackgroundTask:      JobSettingsDto{Concurrency: 5},
			SmartSearch:         JobSettingsDto{Concurrency: 2},
			MetadataExtraction:  JobSettingsDto{Concurrency: 5},
			FaceDetection:       JobSettingsDto{Concurrency: 2},
			Search:              JobSettingsDto{Concurrency: 5},
			Sidecar:             JobSettingsDto{Concurrency: 5},
			Library:             JobSettingsDto{Concurrency: 5},
			Migration:           JobSettingsDto{Concurrency: 5},
			ThumbnailGeneration: JobSettingsDto{Concurrency: 3},
			VideoConversion:     JobSettingsDto{Concurrency: 1},
			Notifications:       JobSettingsDto{Concurrency: 5},
			OCR:                 JobSettingsDto{Concurrency: 1},
			Workflow:            JobSettingsDto{Concurrency: 5},
		},
		Library: LibraryDto{
			Scan:  LibraryScanDto{Enabled: true, CronExpression: "0 0 * * *"},
			Watch: LibraryWatchDto{Enabled: false},
		},
		Logging: LoggingDto{Enabled: true, Level: "log"},
		MachineLearning: MachineLearningDto{
			Enabled:            false,
			URLs:               []string{},
			AvailabilityChecks: AvailabilityChecksDto{Enabled: true, Timeout: 2000, Interval: 30000},
			Clip:               ClipDto{Enabled: true, ModelName: "ViT-B-32__openai"},
			DuplicateDetection: DuplicateDetectionDto{Enabled: true, MaxDistance: 0.01},
			FacialRecognition: FacialRecognitionDto{
				Enabled:     true,
				ModelName:   "buffalo_l",
				MinScore:    0.7,
				MaxDistance: 0.5,
				MinFaces:    3,
			},
			OCR: OCRDto{
				Enabled:             true,
				ModelName:           "PP-OCRv5_mobile",
				MinDetectionScore:   0.5,
				MinRecognitionScore: 0.8,
				MaxResolution:       736,
			},
		},
		Map: MapDto{
			Enabled:    true,
			LightStyle: "https://tiles.immich.cloud/v1/style/light.json",
			DarkStyle:  "https://tiles.immich.cloud/v1/style/dark.json",
		},
		Metadata:        MetadataDto{Faces: MetadataFacesDto{Import: false}},
		NewVersionCheck: NewVersionCheckDto{Enabled: true},
		NightlyTasks: NightlyTasksDto{
			StartTime:         "00:00",
			DatabaseCleanup:   true,
			MissingThumbnails: true,
			ClusterNewFaces:   true,
			GenerateMemories:  true,
			SyncQuotaUsage:    true,
		},
		Notifications: NotificationsDto{
			SMTP: SMTPDto{
				Enabled: false,
				From:    "",
				ReplyTo: "",
				Transport: SMTPTransportDto{
					IgnoreCert: false,
					Host:       "",
					Port:       587,
					Secure:     false,
					Username:   "",
					Password:   "",
				},
			},
		},
		OAuth: OAuthDto{
			AutoLaunch:              false,
			AutoRegister:            true,
			ButtonText:              "Login with OAuth",
			ClientID:                "",
			ClientSecret:            "",
			DefaultStorageQuota:     nil,
			Enabled:                 false,
			IssuerURL:               "",
			MobileOverrideEnabled:   false,
			MobileRedirectURI:       "",
			Scope:                   "openid email profile",
			SigningAlgorithm:        "RS256",
			ProfileSigningAlgorithm: "none",
			TokenEndpointAuthMethod: "client-secret-post",
			Timeout:                 30000,
			StorageLabelClaim:       "preferred_username",
			StorageQuotaClaim:       "immich_quota",
			RoleClaim:               "immich_role",
		},
		PasswordLogin:    PasswordLoginDto{Enabled: true},
		ReverseGeocoding: ReverseGeocodingDto{Enabled: true},
		Server: ServerDto{
			ExternalDomain:   "",
			LoginPageMessage: "",
			PublicUsers:      true,
		},
		StorageTemplate: StorageTemplateDto{
			Enabled:                 false,
			HashVerificationEnabled: true,
			Template:                "{{y}}/{{y}}-{{MM}}-{{dd}}/{{filename}}",
		},
		Templates: TemplatesDto{
			Email: EmailTemplatesDto{
				WelcomeTemplate:     "",
				AlbumInviteTemplate: "",
				AlbumUpdateTemplate: "",
			},
		},
		Theme: ThemeDto{CustomCSS: ""},
		Trash: TrashDto{Enabled: true, Days: 30},
		User:  UserDto{DeleteDelay: 7},
	}
}

// StorageTemplateOptions mirrors upstream SystemConfigTemplateStorageOptionDto.
type StorageTemplateOptions struct {
	YearOptions   []string `json:"yearOptions"`
	MonthOptions  []string `json:"monthOptions"`
	WeekOptions   []string `json:"weekOptions"`
	DayOptions    []string `json:"dayOptions"`
	HourOptions   []string `json:"hourOptions"`
	MinuteOptions []string `json:"minuteOptions"`
	SecondOptions []string `json:"secondOptions"`
	PresetOptions []string `json:"presetOptions"`
}

// GetStorageTemplateStorageOptions returns the token lists the storage
// template editor offers (upstream storage-template.service.ts).
func GetStorageTemplateStorageOptions() StorageTemplateOptions {
	return StorageTemplateOptions{
		YearOptions:   []string{"y", "yy"},
		MonthOptions:  []string{"M", "MM", "MMM", "MMMM"},
		WeekOptions:   []string{"W", "WW"},
		DayOptions:    []string{"d", "dd"},
		HourOptions:   []string{"h", "hh", "H", "HH"},
		MinuteOptions: []string{"m", "mm"},
		SecondOptions: []string{"s", "ss", "SSS"},
		PresetOptions: []string{
			"{{y}}/{{y}}-{{MM}}-{{dd}}/{{filename}}",
			"{{y}}/{{MM}}-{{dd}}/{{filename}}",
			"{{y}}/{{MMMM}}-{{dd}}/{{filename}}",
			"{{y}}/{{MM}}/{{filename}}",
			"{{y}}/{{#if album}}{{album}}{{else}}Other/{{MM}}{{/if}}/{{filename}}",
			"{{#if album}}{{album-startDate-y}}/{{album}}{{else}}{{y}}/Other/{{MM}}{{/if}}/{{filename}}",
			"{{y}}/{{MMM}}/{{filename}}",
			"{{y}}/{{MMMM}}/{{filename}}",
			"{{y}}/{{MM}}/{{dd}}/{{filename}}",
			"{{y}}/{{MMMM}}/{{dd}}/{{filename}}",
			"{{y}}/{{y}}-{{MM}}/{{y}}-{{MM}}-{{dd}}/{{filename}}",
			"{{y}}-{{MM}}-{{dd}}/{{filename}}",
			"{{y}}-{{MMM}}-{{dd}}/{{filename}}",
			"{{y}}-{{MMMM}}-{{dd}}/{{filename}}",
			"{{y}}/{{y}}-{{MM}}/{{filename}}",
			"{{y}}/{{y}}-{{WW}}/{{filename}}",
			"{{y}}/{{y}}-{{MM}}-{{dd}}/{{assetId}}",
			"{{y}}/{{y}}-{{MM}}/{{assetId}}",
			"{{y}}/{{y}}-{{WW}}/{{assetId}}",
			"{{album}}/{{filename}}",
		},
	}
}
