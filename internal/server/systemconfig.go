package server

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

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
		return nil, SanitizedInternal(ctx, "failed to load system config", err)
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
		return nil, SanitizedInternal(ctx, "failed to load system config", err)
	}
	applyProtoToSystemConfig(&cfg, req)

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to encode system config", err)
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
// their current values.
func applyProtoToSystemConfig(cfg *systemconfig.Dto, req *immichv1.SystemConfigDto) {
	if req == nil {
		return
	}
	if f := req.Ffmpeg; f != nil {
		if v, err := strconv.Atoi(f.Crf); err == nil {
			cfg.FFmpeg.CRF = v
		}
		if v, err := strconv.Atoi(f.Threads); err == nil {
			cfg.FFmpeg.Threads = v
		}
		if f.Preset != "" {
			cfg.FFmpeg.Preset = f.Preset
		}
		if f.TargetVideoCodec != "" {
			cfg.FFmpeg.TargetVideoCodec = f.TargetVideoCodec
		}
		if f.AcceptedVideoCodecs != "" {
			cfg.FFmpeg.AcceptedVideoCodecs = strings.Split(f.AcceptedVideoCodecs, ",")
		}
		if f.TargetAudioCodec != "" {
			cfg.FFmpeg.TargetAudioCodec = f.TargetAudioCodec
		}
		if f.AcceptedAudioCodecs != "" {
			cfg.FFmpeg.AcceptedAudioCodecs = strings.Split(f.AcceptedAudioCodecs, ",")
		}
		if f.TargetResolution != "" {
			cfg.FFmpeg.TargetResolution = f.TargetResolution
		}
		if f.MaxBitrate != "" {
			cfg.FFmpeg.MaxBitrate = f.MaxBitrate
		}
		if f.CqMode != "" {
			cfg.FFmpeg.CQMode = f.CqMode
		}
		if f.PreferredHwDevice != "" {
			cfg.FFmpeg.PreferredHwDevice = f.PreferredHwDevice
		}
		if f.Transcode != "" {
			cfg.FFmpeg.Transcode = f.Transcode
		}
		if f.Tonemap != "" {
			cfg.FFmpeg.Tonemap = f.Tonemap
		}
	}
	if j := req.Job; j != nil {
		setConcurrency := func(dst *systemconfig.JobSettingsDto, v int32) {
			if v > 0 {
				dst.Concurrency = int(v)
			}
		}
		setConcurrency(&cfg.Job.BackgroundTask, j.BackgroundTaskConcurrency)
		setConcurrency(&cfg.Job.SmartSearch, j.SmartSearchConcurrency)
		setConcurrency(&cfg.Job.MetadataExtraction, j.MetadataExtractionConcurrency)
		setConcurrency(&cfg.Job.FaceDetection, j.FaceDetectionConcurrency)
		setConcurrency(&cfg.Job.Search, j.SearchConcurrency)
		setConcurrency(&cfg.Job.Sidecar, j.SidecarConcurrency)
		setConcurrency(&cfg.Job.Library, j.LibraryConcurrency)
		setConcurrency(&cfg.Job.Migration, j.MigrationConcurrency)
		setConcurrency(&cfg.Job.ThumbnailGeneration, j.ThumbnailGenerationConcurrency)
		setConcurrency(&cfg.Job.VideoConversion, j.VideoConversionConcurrency)
	}
	if l := req.Library; l != nil {
		if l.Scan != nil {
			cfg.Library.Scan.Enabled = l.Scan.Enabled
			if l.Scan.CronExpression != "" {
				cfg.Library.Scan.CronExpression = l.Scan.CronExpression
			}
		}
		if l.Watch != nil {
			cfg.Library.Watch.Enabled = l.Watch.Enabled
		}
	}
	if l := req.Logging; l != nil {
		cfg.Logging.Enabled = l.Enabled
		if l.Level != "" {
			cfg.Logging.Level = l.Level
		}
	}
	if ml := req.MachineLearning; ml != nil {
		cfg.MachineLearning.Enabled = ml.Enabled
		if ml.Url != "" {
			cfg.MachineLearning.URLs = strings.Split(ml.Url, ",")
		}
		if ml.Clip != nil {
			cfg.MachineLearning.Clip.Enabled = ml.Clip.Enabled
			if ml.Clip.ModelName != "" {
				cfg.MachineLearning.Clip.ModelName = ml.Clip.ModelName
			}
		}
		if ml.DuplicateDetection != nil {
			cfg.MachineLearning.DuplicateDetection.Enabled = ml.DuplicateDetection.Enabled
			if ml.DuplicateDetection.MaxDistance > 0 {
				cfg.MachineLearning.DuplicateDetection.MaxDistance = float64(ml.DuplicateDetection.MaxDistance) / 100
			}
		}
		if fr := ml.FacialRecognition; fr != nil {
			cfg.MachineLearning.FacialRecognition.Enabled = fr.Enabled
			if fr.ModelName != "" {
				cfg.MachineLearning.FacialRecognition.ModelName = fr.ModelName
			}
			if fr.MinScore > 0 {
				cfg.MachineLearning.FacialRecognition.MinScore = float64(fr.MinScore) / 100
			}
			if fr.MaxDistance > 0 {
				cfg.MachineLearning.FacialRecognition.MaxDistance = float64(fr.MaxDistance) / 100
			}
			if fr.MinFaces > 0 {
				cfg.MachineLearning.FacialRecognition.MinFaces = int(fr.MinFaces)
			}
		}
	}
	if m := req.Map; m != nil {
		cfg.Map.Enabled = m.Enabled
		if m.LightStyle != "" {
			cfg.Map.LightStyle = m.LightStyle
		}
		if m.DarkStyle != "" {
			cfg.Map.DarkStyle = m.DarkStyle
		}
	}
	if v := req.NewVersionCheck; v != nil {
		cfg.NewVersionCheck.Enabled = v.Enabled
	}
	if n := req.Notifications; n != nil && n.Smtp != nil {
		cfg.Notifications.SMTP.Enabled = n.Smtp.Enabled
		cfg.Notifications.SMTP.From = n.Smtp.From
		cfg.Notifications.SMTP.ReplyTo = n.Smtp.ReplyTo
		if t := n.Smtp.Transport; t != nil {
			cfg.Notifications.SMTP.Transport.Host = t.Host
			cfg.Notifications.SMTP.Transport.Port = int(t.Port)
			cfg.Notifications.SMTP.Transport.Username = t.Username
			cfg.Notifications.SMTP.Transport.Password = t.Password
			cfg.Notifications.SMTP.Transport.IgnoreCert = t.IgnoreCert
		}
	}
	if o := req.Oauth; o != nil {
		cfg.OAuth.Enabled = o.Enabled
		cfg.OAuth.IssuerURL = o.IssuerUrl
		cfg.OAuth.ClientID = o.ClientId
		cfg.OAuth.ClientSecret = o.ClientSecret
		cfg.OAuth.Scope = o.Scope
		cfg.OAuth.StorageLabelClaim = o.StorageLabelClaim
		cfg.OAuth.StorageQuotaClaim = o.StorageQuotaClaim
		cfg.OAuth.AutoRegister = o.AutoRegister
		cfg.OAuth.AutoLaunch = o.AutoLaunch
		cfg.OAuth.ButtonText = o.ButtonText
		cfg.OAuth.MobileOverrideEnabled = o.MobileOverrideEnabled
		cfg.OAuth.MobileRedirectURI = o.MobileRedirectUri
		if o.DefaultStorageQuota != "" {
			if v, err := strconv.ParseInt(o.DefaultStorageQuota, 10, 64); err == nil {
				cfg.OAuth.DefaultStorageQuota = &v
			}
		}
	}
	if p := req.PasswordLogin; p != nil {
		cfg.PasswordLogin.Enabled = p.Enabled
	}
	if r := req.ReverseGeocoding; r != nil {
		cfg.ReverseGeocoding.Enabled = r.Enabled
	}
	if sv := req.Server; sv != nil {
		cfg.Server.ExternalDomain = sv.ExternalDomain
		cfg.Server.LoginPageMessage = sv.LoginPageMessage
	}
	if st := req.StorageTemplate; st != nil {
		cfg.StorageTemplate.Enabled = st.Enabled
		cfg.StorageTemplate.HashVerificationEnabled = st.HashVerificationEnabled
		if st.Template != "" {
			cfg.StorageTemplate.Template = st.Template
		}
	}
	if t := req.Theme; t != nil {
		cfg.Theme.CustomCSS = t.CustomCss
	}
	if t := req.Trash; t != nil {
		cfg.Trash.Enabled = t.Enabled
		if t.Days > 0 {
			cfg.Trash.Days = int(t.Days)
		}
	}
	if u := req.User; u != nil {
		if u.DeleteDelay > 0 {
			cfg.User.DeleteDelay = int(u.DeleteDelay)
		}
	}
}
