package server

import (
	"encoding/json"
	"testing"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/systemconfig"
)

// These tests pin the behaviour of the systemconfig proto↔Dto conversion pair
// before applyProtoToSystemConfig is split into per-section appliers. They are
// deliberately behavioural, not structural: they say nothing about how the
// conversion is organised, only what it produces.

// TestSystemConfigRoundTripIsIdempotent is the core invariant: converting a
// config to proto and applying it back over itself must not change it. Every
// field the proto can express has to survive both directions unchanged.
func TestSystemConfigRoundTripIsIdempotent(t *testing.T) {
	original := systemconfig.DefaultDto()

	applied := systemconfig.DefaultDto()
	applyProtoToSystemConfig(&applied, systemConfigToProto(original))

	if diff := jsonDiff(t, original, applied); diff != "" {
		t.Errorf("round trip changed the config:\n%s", diff)
	}
}

// TestApplyProtoNilLeavesConfigUntouched pins the nil guard.
func TestApplyProtoNilLeavesConfigUntouched(t *testing.T) {
	before := systemconfig.DefaultDto()
	after := systemconfig.DefaultDto()

	applyProtoToSystemConfig(&after, nil)

	if diff := jsonDiff(t, before, after); diff != "" {
		t.Errorf("nil request modified the config:\n%s", diff)
	}
}

// TestApplyProtoEmptyLeavesConfigUntouched pins the "absent section keeps its
// current value" contract that the web UI's partial updates rely on: an empty
// SystemConfigDto must be a no-op, not a reset to zero values.
func TestApplyProtoEmptyLeavesConfigUntouched(t *testing.T) {
	before := systemconfig.DefaultDto()
	after := systemconfig.DefaultDto()

	applyProtoToSystemConfig(&after, &immichv1.SystemConfigDto{})

	if diff := jsonDiff(t, before, after); diff != "" {
		t.Errorf("empty request modified the config:\n%s", diff)
	}
}

// TestApplyProtoSingleSection pins that one populated section is applied and
// the rest of the document is left alone.
func TestApplyProtoSingleSection(t *testing.T) {
	cfg := systemconfig.DefaultDto()
	untouched := systemconfig.DefaultDto()

	applyProtoToSystemConfig(&cfg, &immichv1.SystemConfigDto{
		Ffmpeg: &immichv1.SystemConfigFFmpegDto{
			Crf:     "27",
			Threads: "3",
			Preset:  "slow",
		},
	})

	if cfg.FFmpeg.CRF != 27 {
		t.Errorf("FFmpeg.CRF = %d, want 27", cfg.FFmpeg.CRF)
	}
	if cfg.FFmpeg.Threads != 3 {
		t.Errorf("FFmpeg.Threads = %d, want 3", cfg.FFmpeg.Threads)
	}
	if cfg.FFmpeg.Preset != "slow" {
		t.Errorf("FFmpeg.Preset = %q, want %q", cfg.FFmpeg.Preset, "slow")
	}
	if cfg.Job != untouched.Job {
		t.Errorf("Job section changed: got %+v, want %+v", cfg.Job, untouched.Job)
	}
	if cfg.StorageTemplate != untouched.StorageTemplate {
		t.Errorf("StorageTemplate changed: got %+v, want %+v", cfg.StorageTemplate, untouched.StorageTemplate)
	}
}

// TestSystemConfigToProtoPopulatesEverySection pins that no section silently
// drops out of the outbound conversion — a nil section here becomes a missing
// object in the JSON the web UI reads.
func TestSystemConfigToProtoPopulatesEverySection(t *testing.T) {
	got := systemConfigToProto(systemconfig.DefaultDto())

	sections := map[string]any{
		"ffmpeg":          got.Ffmpeg,
		"job":             got.Job,
		"logging":         got.Logging,
		"machineLearning": got.MachineLearning,
		"map":             got.Map,
		"newVersionCheck": got.NewVersionCheck,
		"oauth":           got.Oauth,
		"passwordLogin":   got.PasswordLogin,
		"reverseGeocding": got.ReverseGeocoding,
		"server":          got.Server,
		"storageTemplate": got.StorageTemplate,
		"theme":           got.Theme,
		"trash":           got.Trash,
		"user":            got.User,
	}
	for name, section := range sections {
		if isNilSection(section) {
			t.Errorf("section %q is nil in the proto conversion", name)
		}
	}
}

func isNilSection(v any) bool {
	switch s := v.(type) {
	case *immichv1.SystemConfigFFmpegDto:
		return s == nil
	case *immichv1.SystemConfigJobDto:
		return s == nil
	case *immichv1.SystemConfigLoggingDto:
		return s == nil
	case *immichv1.SystemConfigMachineLearningDto:
		return s == nil
	case *immichv1.SystemConfigMapDto:
		return s == nil
	case *immichv1.SystemConfigNewVersionCheckDto:
		return s == nil
	case *immichv1.SystemConfigOAuthDto:
		return s == nil
	case *immichv1.SystemConfigPasswordLoginDto:
		return s == nil
	case *immichv1.SystemConfigReverseGeocodingDto:
		return s == nil
	case *immichv1.SystemConfigServerDto:
		return s == nil
	case *immichv1.SystemConfigStorageTemplateDto:
		return s == nil
	case *immichv1.SystemConfigThemeDto:
		return s == nil
	case *immichv1.SystemConfigTrashDto:
		return s == nil
	case *immichv1.SystemConfigUserDto:
		return s == nil
	default:
		return v == nil
	}
}

func jsonDiff(t *testing.T, want, got systemconfig.Dto) string {
	t.Helper()

	wantJSON, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	if string(wantJSON) == string(gotJSON) {
		return ""
	}
	return "want:\n" + string(wantJSON) + "\n\ngot:\n" + string(gotJSON)
}
