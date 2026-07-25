package server

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/denysvitali/immich-go-backend/internal/grpcutil"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/systemconfig"
)

func (s *Server) GetSystemConfig(ctx context.Context, _ *emptypb.Empty) (*immichv1.SystemConfigDto, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	cfg, err := s.systemConfigService.GetConfigDto(ctx)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to load system config", err)
	}
	return systemConfigToProto(cfg), nil
}

func (s *Server) GetSystemConfigDefaults(ctx context.Context, _ *emptypb.Empty) (*immichv1.SystemConfigDto, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}
	return systemConfigToProto(systemconfig.DefaultDto()), nil
}

func (s *Server) UpdateSystemConfig(ctx context.Context, req *immichv1.SystemConfigDto) (*immichv1.SystemConfigDto, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	// Merge the proto payload over the current effective config, then persist
	// the result as a full document (the same contract the web UI uses).
	cfg, err := s.systemConfigService.GetConfigDto(ctx)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to load system config", err)
	}
	applyProtoToSystemConfig(&cfg, req)

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to encode system config", err)
	}

	updated, err := s.systemConfigService.UpdateConfigDto(ctx, raw)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to update system config: %v", err)
	}
	return systemConfigToProto(updated), nil
}

func (s *Server) GetSystemConfigTemplate(ctx context.Context, _ *immichv1.GetSystemConfigTemplateRequest) (*immichv1.SystemConfigTemplateStorageOptionDto, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}

	opts := systemconfig.GetStorageTemplateStorageOptions()
	// The proto message models the token groups as counts; the exact string
	// lists are served on the REST route (see http_frontend.go).
	return &immichv1.SystemConfigTemplateStorageOptionDto{
		YearOptions:   int32(len(opts.YearOptions)),
		MonthOptions:  int32(len(opts.MonthOptions)),
		DayOptions:    int32(len(opts.DayOptions)),
		HourOptions:   int32(len(opts.HourOptions)),
		MinuteOptions: int32(len(opts.MinuteOptions)),
		SecondOptions: int32(len(opts.SecondOptions)),
		PresetOptions: opts.PresetOptions,
	}, nil
}

func systemConfigToProto(cfg systemconfig.Dto) *immichv1.SystemConfigDto {
	return &immichv1.SystemConfigDto{
		Ffmpeg: &immichv1.SystemConfigFFmpegDto{
			Crf:                 strconv.Itoa(cfg.FFmpeg.CRF),
			Threads:             strconv.Itoa(cfg.FFmpeg.Threads),
			Preset:              cfg.FFmpeg.Preset,
			TargetVideoCodec:    cfg.FFmpeg.TargetVideoCodec,
			AcceptedVideoCodecs: strings.Join(cfg.FFmpeg.AcceptedVideoCodecs, ","),
			TargetAudioCodec:    cfg.FFmpeg.TargetAudioCodec,
			AcceptedAudioCodecs: strings.Join(cfg.FFmpeg.AcceptedAudioCodecs, ","),
			TargetResolution:    cfg.FFmpeg.TargetResolution,
			MaxBitrate:          cfg.FFmpeg.MaxBitrate,
			Bframes:             strconv.Itoa(cfg.FFmpeg.BFrames),
			Refs:                strconv.Itoa(cfg.FFmpeg.Refs),
			GopSize:             strconv.Itoa(cfg.FFmpeg.GopSize),
			TemporalAq:          strconv.FormatBool(cfg.FFmpeg.TemporalAQ),
			CqMode:              cfg.FFmpeg.CQMode,
			TwoPass:             strconv.FormatBool(cfg.FFmpeg.TwoPass),
			PreferredHwDevice:   cfg.FFmpeg.PreferredHwDevice,
			Transcode:           cfg.FFmpeg.Transcode,
			Tonemap:             cfg.FFmpeg.Tonemap,
		},
		Job: &immichv1.SystemConfigJobDto{
			BackgroundTaskConcurrency:           int32(cfg.Job.BackgroundTask.Concurrency),
			ClipEncodingConcurrency:             int32(cfg.Job.SmartSearch.Concurrency),
			DuplicateDetectionConcurrency:       int32(cfg.Job.BackgroundTask.Concurrency),
			FaceDetectionConcurrency:            int32(cfg.Job.FaceDetection.Concurrency),
			FacialRecognitionConcurrency:        int32(cfg.Job.FaceDetection.Concurrency),
			LibraryConcurrency:                  int32(cfg.Job.Library.Concurrency),
			MetadataExtractionConcurrency:       int32(cfg.Job.MetadataExtraction.Concurrency),
			MigrationConcurrency:                int32(cfg.Job.Migration.Concurrency),
			SearchConcurrency:                   int32(cfg.Job.Search.Concurrency),
			SidecarConcurrency:                  int32(cfg.Job.Sidecar.Concurrency),
			SmartSearchConcurrency:              int32(cfg.Job.SmartSearch.Concurrency),
			StorageTemplateMigrationConcurrency: int32(cfg.Job.Migration.Concurrency),
			ThumbnailGenerationConcurrency:      int32(cfg.Job.ThumbnailGeneration.Concurrency),
			VideoConversionConcurrency:          int32(cfg.Job.VideoConversion.Concurrency),
		},
		Library: &immichv1.SystemConfigLibraryDto{
			Scan: &immichv1.SystemConfigLibraryScanDto{
				Enabled:        cfg.Library.Scan.Enabled,
				CronExpression: cfg.Library.Scan.CronExpression,
			},
			Watch: &immichv1.SystemConfigLibraryWatchDto{
				Enabled: cfg.Library.Watch.Enabled,
			},
		},
		Logging: &immichv1.SystemConfigLoggingDto{
			Enabled: cfg.Logging.Enabled,
			Level:   cfg.Logging.Level,
		},
		MachineLearning: &immichv1.SystemConfigMachineLearningDto{
			Enabled: cfg.MachineLearning.Enabled,
			Url:     strings.Join(cfg.MachineLearning.URLs, ","),
			Clip: &immichv1.SystemConfigMachineLearningClipDto{
				Enabled:   cfg.MachineLearning.Clip.Enabled,
				ModelName: cfg.MachineLearning.Clip.ModelName,
			},
			DuplicateDetection: &immichv1.SystemConfigMachineLearningDuplicateDetectionDto{
				Enabled:     cfg.MachineLearning.DuplicateDetection.Enabled,
				MaxDistance: int32(cfg.MachineLearning.DuplicateDetection.MaxDistance * 100),
			},
			FacialRecognition: &immichv1.SystemConfigMachineLearningFacialRecognitionDto{
				Enabled:     cfg.MachineLearning.FacialRecognition.Enabled,
				ModelName:   cfg.MachineLearning.FacialRecognition.ModelName,
				MinScore:    int32(cfg.MachineLearning.FacialRecognition.MinScore * 100),
				MaxDistance: int32(cfg.MachineLearning.FacialRecognition.MaxDistance * 100),
				MinFaces:    int32(cfg.MachineLearning.FacialRecognition.MinFaces),
			},
		},
		Map: &immichv1.SystemConfigMapDto{
			Enabled:    cfg.Map.Enabled,
			LightStyle: cfg.Map.LightStyle,
			DarkStyle:  cfg.Map.DarkStyle,
		},
		NewVersionCheck: &immichv1.SystemConfigNewVersionCheckDto{
			Enabled: cfg.NewVersionCheck.Enabled,
		},
		Notifications: &immichv1.SystemConfigNotificationsDto{
			Smtp: &immichv1.SystemConfigNotificationsSmtpDto{
				Enabled: cfg.Notifications.SMTP.Enabled,
				From:    cfg.Notifications.SMTP.From,
				ReplyTo: cfg.Notifications.SMTP.ReplyTo,
				Transport: &immichv1.SystemConfigSmtpTransportDto{
					Host:       cfg.Notifications.SMTP.Transport.Host,
					Port:       int32(cfg.Notifications.SMTP.Transport.Port),
					Username:   cfg.Notifications.SMTP.Transport.Username,
					Password:   cfg.Notifications.SMTP.Transport.Password,
					IgnoreCert: cfg.Notifications.SMTP.Transport.IgnoreCert,
					Secure:     cfg.Notifications.SMTP.Transport.Secure,
				},
			},
		},
		Oauth: &immichv1.SystemConfigOAuthDto{
			Enabled:               cfg.OAuth.Enabled,
			IssuerUrl:             cfg.OAuth.IssuerURL,
			ClientId:              cfg.OAuth.ClientID,
			ClientSecret:          cfg.OAuth.ClientSecret,
			Scope:                 cfg.OAuth.Scope,
			StorageLabelClaim:     cfg.OAuth.StorageLabelClaim,
			StorageQuotaClaim:     cfg.OAuth.StorageQuotaClaim,
			DefaultStorageQuota:   formatQuota(cfg.OAuth.DefaultStorageQuota),
			AutoRegister:          cfg.OAuth.AutoRegister,
			AutoLaunch:            cfg.OAuth.AutoLaunch,
			ButtonText:            cfg.OAuth.ButtonText,
			MobileOverrideEnabled: cfg.OAuth.MobileOverrideEnabled,
			MobileRedirectUri:     cfg.OAuth.MobileRedirectURI,
		},
		PasswordLogin: &immichv1.SystemConfigPasswordLoginDto{
			Enabled: cfg.PasswordLogin.Enabled,
		},
		ReverseGeocoding: &immichv1.SystemConfigReverseGeocodingDto{
			Enabled: cfg.ReverseGeocoding.Enabled,
		},
		Server: &immichv1.SystemConfigServerDto{
			ExternalDomain:   cfg.Server.ExternalDomain,
			LoginPageMessage: cfg.Server.LoginPageMessage,
		},
		StorageTemplate: &immichv1.SystemConfigStorageTemplateDto{
			Enabled:                 cfg.StorageTemplate.Enabled,
			HashVerificationEnabled: cfg.StorageTemplate.HashVerificationEnabled,
			Template:                cfg.StorageTemplate.Template,
		},
		Theme: &immichv1.SystemConfigThemeDto{
			CustomCss: cfg.Theme.CustomCSS,
		},
		Trash: &immichv1.SystemConfigTrashDto{
			Enabled: cfg.Trash.Enabled,
			Days:    int32(cfg.Trash.Days),
		},
		User: &immichv1.SystemConfigUserDto{
			DeleteDelay: int64(cfg.User.DeleteDelay),
		},
	}
}

func formatQuota(quota *int64) string {
	if quota == nil {
		return ""
	}
	return strconv.FormatInt(*quota, 10)
}

// applyProtoToSystemConfig copies the fields representable in the proto DTO
// onto the full configuration document. Sections absent from the proto keep
// their current values, which is what makes the web UI's partial updates work.
//
// Each section is applied by its own function below; they nil-check their own
// argument so this stays a checklist.
func applyProtoToSystemConfig(cfg *systemconfig.Dto, req *immichv1.SystemConfigDto) {
	if req == nil {
		return
	}

	applyFFmpegConfig(&cfg.FFmpeg, req.Ffmpeg)
	applyJobConfig(&cfg.Job, req.Job)
	applyLibraryConfig(&cfg.Library, req.Library)
	applyLoggingConfig(&cfg.Logging, req.Logging)
	applyMachineLearningConfig(&cfg.MachineLearning, req.MachineLearning)
	applyMapConfig(&cfg.Map, req.Map)
	applyNotificationsConfig(&cfg.Notifications, req.Notifications)
	applyOAuthConfig(&cfg.OAuth, req.Oauth)
	applyServerConfig(&cfg.Server, req.Server)
	applyStorageTemplateConfig(&cfg.StorageTemplate, req.StorageTemplate)
	applyTrashConfig(&cfg.Trash, req.Trash)

	if v := req.NewVersionCheck; v != nil {
		cfg.NewVersionCheck.Enabled = v.Enabled
	}
	if p := req.PasswordLogin; p != nil {
		cfg.PasswordLogin.Enabled = p.Enabled
	}
	if r := req.ReverseGeocoding; r != nil {
		cfg.ReverseGeocoding.Enabled = r.Enabled
	}
	if t := req.Theme; t != nil {
		cfg.Theme.CustomCSS = t.CustomCss
	}
	if u := req.User; u != nil && u.DeleteDelay > 0 {
		cfg.User.DeleteDelay = int(u.DeleteDelay)
	}
}

// setIfNotEmpty assigns v to *dst unless v is empty. An empty string in the
// proto means "field not sent", so it must not clear the stored value.
func setIfNotEmpty(dst *string, v string) {
	if v != "" {
		*dst = v
	}
}

// setListIfNotEmpty assigns the comma-separated members of v to *dst unless v
// is empty. The proto models these lists as one comma-joined string.
func setListIfNotEmpty(dst *[]string, v string) {
	if v != "" {
		*dst = strings.Split(v, ",")
	}
}

func applyFFmpegConfig(cfg *systemconfig.FFmpegDto, dto *immichv1.SystemConfigFFmpegDto) {
	if dto == nil {
		return
	}

	// CRF and Threads are strings in the proto; an unparseable value is
	// ignored rather than rejected, matching the previous behaviour.
	if v, err := strconv.Atoi(dto.Crf); err == nil {
		cfg.CRF = v
	}
	if v, err := strconv.Atoi(dto.Threads); err == nil {
		cfg.Threads = v
	}

	setIfNotEmpty(&cfg.Preset, dto.Preset)
	setIfNotEmpty(&cfg.TargetVideoCodec, dto.TargetVideoCodec)
	setIfNotEmpty(&cfg.TargetAudioCodec, dto.TargetAudioCodec)
	setIfNotEmpty(&cfg.TargetResolution, dto.TargetResolution)
	setIfNotEmpty(&cfg.MaxBitrate, dto.MaxBitrate)
	setIfNotEmpty(&cfg.CQMode, dto.CqMode)
	setIfNotEmpty(&cfg.PreferredHwDevice, dto.PreferredHwDevice)
	setIfNotEmpty(&cfg.Transcode, dto.Transcode)
	setIfNotEmpty(&cfg.Tonemap, dto.Tonemap)
	setListIfNotEmpty(&cfg.AcceptedVideoCodecs, dto.AcceptedVideoCodecs)
	setListIfNotEmpty(&cfg.AcceptedAudioCodecs, dto.AcceptedAudioCodecs)
}

func applyJobConfig(cfg *systemconfig.JobDto, dto *immichv1.SystemConfigJobDto) {
	if dto == nil {
		return
	}

	// Zero means "not sent"; a queue's concurrency is never legitimately 0.
	setConcurrency := func(dst *systemconfig.JobSettingsDto, v int32) {
		if v > 0 {
			dst.Concurrency = int(v)
		}
	}

	setConcurrency(&cfg.BackgroundTask, dto.BackgroundTaskConcurrency)
	setConcurrency(&cfg.SmartSearch, dto.SmartSearchConcurrency)
	setConcurrency(&cfg.MetadataExtraction, dto.MetadataExtractionConcurrency)
	setConcurrency(&cfg.FaceDetection, dto.FaceDetectionConcurrency)
	setConcurrency(&cfg.Search, dto.SearchConcurrency)
	setConcurrency(&cfg.Sidecar, dto.SidecarConcurrency)
	setConcurrency(&cfg.Library, dto.LibraryConcurrency)
	setConcurrency(&cfg.Migration, dto.MigrationConcurrency)
	setConcurrency(&cfg.ThumbnailGeneration, dto.ThumbnailGenerationConcurrency)
	setConcurrency(&cfg.VideoConversion, dto.VideoConversionConcurrency)
}

func applyLibraryConfig(cfg *systemconfig.LibraryDto, dto *immichv1.SystemConfigLibraryDto) {
	if dto == nil {
		return
	}

	if dto.Scan != nil {
		cfg.Scan.Enabled = dto.Scan.Enabled
		setIfNotEmpty(&cfg.Scan.CronExpression, dto.Scan.CronExpression)
	}
	if dto.Watch != nil {
		cfg.Watch.Enabled = dto.Watch.Enabled
	}
}

func applyLoggingConfig(cfg *systemconfig.LoggingDto, dto *immichv1.SystemConfigLoggingDto) {
	if dto == nil {
		return
	}

	cfg.Enabled = dto.Enabled
	setIfNotEmpty(&cfg.Level, dto.Level)
}

func applyMachineLearningConfig(cfg *systemconfig.MachineLearningDto, dto *immichv1.SystemConfigMachineLearningDto) {
	if dto == nil {
		return
	}

	cfg.Enabled = dto.Enabled
	setListIfNotEmpty(&cfg.URLs, dto.Url)

	if clip := dto.Clip; clip != nil {
		cfg.Clip.Enabled = clip.Enabled
		setIfNotEmpty(&cfg.Clip.ModelName, clip.ModelName)
	}
	if dd := dto.DuplicateDetection; dd != nil {
		cfg.DuplicateDetection.Enabled = dd.Enabled
		// The proto carries distances as percentages; the document stores
		// them as fractions.
		if dd.MaxDistance > 0 {
			cfg.DuplicateDetection.MaxDistance = float64(dd.MaxDistance) / 100
		}
	}
	if fr := dto.FacialRecognition; fr != nil {
		cfg.FacialRecognition.Enabled = fr.Enabled
		setIfNotEmpty(&cfg.FacialRecognition.ModelName, fr.ModelName)
		if fr.MinScore > 0 {
			cfg.FacialRecognition.MinScore = float64(fr.MinScore) / 100
		}
		if fr.MaxDistance > 0 {
			cfg.FacialRecognition.MaxDistance = float64(fr.MaxDistance) / 100
		}
		if fr.MinFaces > 0 {
			cfg.FacialRecognition.MinFaces = int(fr.MinFaces)
		}
	}
}

func applyMapConfig(cfg *systemconfig.MapDto, dto *immichv1.SystemConfigMapDto) {
	if dto == nil {
		return
	}

	cfg.Enabled = dto.Enabled
	setIfNotEmpty(&cfg.LightStyle, dto.LightStyle)
	setIfNotEmpty(&cfg.DarkStyle, dto.DarkStyle)
}

func applyNotificationsConfig(cfg *systemconfig.NotificationsDto, dto *immichv1.SystemConfigNotificationsDto) {
	if dto == nil || dto.Smtp == nil {
		return
	}

	// SMTP credentials are replaced wholesale, not merged: an empty username
	// or password is a legitimate way to clear them.
	cfg.SMTP.Enabled = dto.Smtp.Enabled
	cfg.SMTP.From = dto.Smtp.From
	cfg.SMTP.ReplyTo = dto.Smtp.ReplyTo

	if t := dto.Smtp.Transport; t != nil {
		cfg.SMTP.Transport.Host = t.Host
		cfg.SMTP.Transport.Port = int(t.Port)
		cfg.SMTP.Transport.Username = t.Username
		cfg.SMTP.Transport.Password = t.Password
		cfg.SMTP.Transport.IgnoreCert = t.IgnoreCert
		cfg.SMTP.Transport.Secure = t.Secure
	}
}

func applyOAuthConfig(cfg *systemconfig.OAuthDto, dto *immichv1.SystemConfigOAuthDto) {
	if dto == nil {
		return
	}

	// Every field is replaced wholesale: clearing the client secret or the
	// issuer URL is how an admin disables an OAuth provider from the UI.
	cfg.Enabled = dto.Enabled
	cfg.IssuerURL = dto.IssuerUrl
	cfg.ClientID = dto.ClientId
	cfg.ClientSecret = dto.ClientSecret
	cfg.Scope = dto.Scope
	cfg.StorageLabelClaim = dto.StorageLabelClaim
	cfg.StorageQuotaClaim = dto.StorageQuotaClaim
	cfg.AutoRegister = dto.AutoRegister
	cfg.AutoLaunch = dto.AutoLaunch
	cfg.ButtonText = dto.ButtonText
	cfg.MobileOverrideEnabled = dto.MobileOverrideEnabled
	cfg.MobileRedirectURI = dto.MobileRedirectUri

	if dto.DefaultStorageQuota != "" {
		if v, err := strconv.ParseInt(dto.DefaultStorageQuota, 10, 64); err == nil {
			cfg.DefaultStorageQuota = &v
		}
	}
}

func applyServerConfig(cfg *systemconfig.ServerDto, dto *immichv1.SystemConfigServerDto) {
	if dto == nil {
		return
	}

	cfg.ExternalDomain = dto.ExternalDomain
	cfg.LoginPageMessage = dto.LoginPageMessage
}

func applyStorageTemplateConfig(cfg *systemconfig.StorageTemplateDto, dto *immichv1.SystemConfigStorageTemplateDto) {
	if dto == nil {
		return
	}

	cfg.Enabled = dto.Enabled
	cfg.HashVerificationEnabled = dto.HashVerificationEnabled
	setIfNotEmpty(&cfg.Template, dto.Template)
}

func applyTrashConfig(cfg *systemconfig.TrashDto, dto *immichv1.SystemConfigTrashDto) {
	if dto == nil {
		return
	}

	cfg.Enabled = dto.Enabled
	if dto.Days > 0 {
		cfg.Days = int(dto.Days)
	}
}
