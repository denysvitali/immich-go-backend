package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// SetMaintenanceMode sets the maintenance mode
func (s *Server) SetMaintenanceMode(ctx context.Context, req *immichv1.SetMaintenanceModeRequest) (*immichv1.SetMaintenanceModeResponse, error) {
	claims, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	switch req.Action {
	case immichv1.MaintenanceAction_MAINTENANCE_ACTION_START:
		token, err := s.maintenanceService.StartMaintenance(ctx, claims.Email)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to start maintenance mode", err)
		}
		return &immichv1.SetMaintenanceModeResponse{
			IsMaintenanceMode: true,
			Token:             &token,
		}, nil

	case immichv1.MaintenanceAction_MAINTENANCE_ACTION_STOP:
		err := s.maintenanceService.StopMaintenance(ctx)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to stop maintenance mode", err)
		}
		return &immichv1.SetMaintenanceModeResponse{
			IsMaintenanceMode: false,
		}, nil

	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid maintenance action")
	}
}

// MaintenanceLogin logs into maintenance mode
func (s *Server) MaintenanceLogin(ctx context.Context, req *immichv1.MaintenanceLoginRequest) (*immichv1.MaintenanceAuthResponse, error) {
	// Check if we're in maintenance mode
	state, err := s.maintenanceService.GetMaintenanceMode(ctx)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get maintenance mode state", err)
	}

	if !state.IsMaintenanceMode {
		return nil, status.Errorf(codes.FailedPrecondition, "not in maintenance mode")
	}

	// Validate the token if provided
	token := ""
	if req.Token != nil {
		token = *req.Token
	}

	if token == "" {
		return nil, status.Errorf(codes.InvalidArgument, "maintenance token required")
	}

	claims, err := s.maintenanceService.ValidateMaintenanceToken(ctx, token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid maintenance token: %v", err)
	}

	return &immichv1.MaintenanceAuthResponse{
		Username:        claims.Username,
		IsAuthenticated: true,
	}, nil
}

// GetMaintenanceStatus returns the current maintenance mode status
func (s *Server) GetMaintenanceStatus(ctx context.Context, _ *emptypb.Empty) (*immichv1.MaintenanceStatusResponse, error) {
	state, err := s.maintenanceService.GetMaintenanceMode(ctx)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get maintenance mode state", err)
	}

	response := &immichv1.MaintenanceStatusResponse{
		IsMaintenanceMode: state.IsMaintenanceMode,
	}

	if state.IsMaintenanceMode {
		response.StartedBy = &state.StartedBy
		startedAt := state.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		response.StartedAt = &startedAt
	}

	return response, nil
}

// GetApkLinks returns APK download links for mobile apps
func (s *Server) GetApkLinks(ctx context.Context, _ *emptypb.Empty) (*immichv1.ServerApkLinksResponse, error) {
	// Return links to official Immich APK releases
	// In production, this would fetch from a configuration or external source
	links := []*immichv1.ApkLink{
		{
			Name:    "Immich Android App",
			Url:     "https://github.com/immich-app/immich/releases/latest/download/app-release.apk",
			Version: Version,
		},
		{
			Name:    "Immich Android App (F-Droid)",
			Url:     "https://f-droid.org/packages/app.alextran.immich/",
			Version: Version,
		},
	}

	return &immichv1.ServerApkLinksResponse{
		Links: links,
	}, nil
}

const (
	versionCheckURL      = "https://api.github.com/repos/immich-app/immich/releases/latest"
	versionCheckCacheTTL = time.Hour
	versionCheckTimeout  = 5 * time.Second
)

// versionCheckHTTPClient is used for the GitHub releases lookup; overridable in tests.
var versionCheckHTTPClient = &http.Client{Timeout: versionCheckTimeout}

// versionCheckCache holds the last successful version check result.
var versionCheckCache struct {
	sync.Mutex
	response  *immichv1.ServerVersionCheckResponse
	url       string
	fetchedAt time.Time
}

// CheckVersion checks for available version updates against the upstream
// Immich GitHub releases. Results are cached in memory; failures degrade
// gracefully to a "no update available" response instead of erroring.
func (s *Server) CheckVersion(ctx context.Context, _ *emptypb.Empty) (*immichv1.ServerVersionCheckResponse, error) {
	currentVersion := Version
	if currentVersion == "" {
		currentVersion = "dev"
	}

	noUpdate := func() *immichv1.ServerVersionCheckResponse {
		return &immichv1.ServerVersionCheckResponse{
			CurrentVersion:    currentVersion,
			LatestVersion:     currentVersion,
			IsUpdateAvailable: false,
			ReleaseNotesUrl:   "https://github.com/immich-app/immich/releases/latest",
			CheckedAt:         timestamppb.Now(),
		}
	}

	// Respect the newVersionCheck.enabled system config flag.
	if s.systemConfigService != nil {
		if cfg, err := s.systemConfigService.GetSystemConfig(ctx); err != nil {
			logrus.WithError(err).Warn("version check: failed to load system config, proceeding with check")
		} else if !cfg.NewVersionCheck.Enabled {
			return noUpdate(), nil
		}
	}

	// Serve from cache while fresh.
	versionCheckCache.Lock()
	if versionCheckCache.response != nil && versionCheckCache.url == versionCheckURL &&
		time.Since(versionCheckCache.fetchedAt) < versionCheckCacheTTL {
		cached := proto.Clone(versionCheckCache.response).(*immichv1.ServerVersionCheckResponse)
		versionCheckCache.Unlock()
		return cached, nil
	}
	versionCheckCache.Unlock()

	latest, err := fetchLatestRelease(ctx, versionCheckURL, currentVersion)
	if err != nil {
		// Offline or API failure: degrade gracefully instead of failing the request.
		logrus.WithError(err).Warn("version check: unable to reach GitHub releases API")
		return noUpdate(), nil
	}

	response := &immichv1.ServerVersionCheckResponse{
		CurrentVersion:    currentVersion,
		LatestVersion:     latest.TagName,
		IsUpdateAvailable: isVersionNewer(latest.TagName, currentVersion),
		ReleaseNotesUrl:   latest.HTMLURL,
		CheckedAt:         timestamppb.Now(),
	}

	versionCheckCache.Lock()
	versionCheckCache.response = proto.Clone(response).(*immichv1.ServerVersionCheckResponse)
	versionCheckCache.url = versionCheckURL
	versionCheckCache.fetchedAt = time.Now()
	versionCheckCache.Unlock()

	return response, nil
}

type latestRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func fetchLatestRelease(ctx context.Context, url, currentVersion string) (*latestRelease, error) {
	checkCtx, cancel := context.WithTimeout(ctx, versionCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "immich-go-backend/"+currentVersion)

	resp, err := versionCheckHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, status.Errorf(codes.Unavailable, "version check failed with status %d", resp.StatusCode)
	}

	var latest latestRelease
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return nil, err
	}
	if latest.TagName == "" {
		return nil, status.Error(codes.Unavailable, "version check response did not include a release tag")
	}
	if latest.HTMLURL == "" {
		latest.HTMLURL = "https://github.com/immich-app/immich/releases/latest"
	}
	return &latest, nil
}

func isVersionNewer(latest, current string) bool {
	latestParts, latestOK := parseVersionParts(latest)
	currentParts, currentOK := parseVersionParts(current)
	if !latestOK || !currentOK {
		return false
	}

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

func parseVersionParts(version string) ([3]int, bool) {
	var parts [3]int
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	fields := strings.Split(v, ".")
	if len(fields) < 2 {
		return parts, false
	}
	for i := 0; i < len(parts) && i < len(fields); i++ {
		n, err := strconv.Atoi(fields[i])
		if err != nil {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}
