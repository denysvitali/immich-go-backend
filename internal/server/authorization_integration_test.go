//go:build integration

package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/denysvitali/immich-go-backend/internal/users"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIntegration_AlbumCompatibilityRouteAppliesAuthorization(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	ctx := context.Background()
	tdb := testdb.SetupTestDB(t)
	conn, err := db.New(ctx, tdb.ConnStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	cfg := &config.Config{Auth: config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		JWTRefreshExpiry:    24 * time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}}
	authService := auth.NewService(cfg.Auth, tdb.Queries)
	owner, err := authService.Register(ctx, auth.RegisterRequest{
		Email: "album-http-owner@example.com", Password: "SecurePass123!", Name: "Album Owner",
	})
	require.NoError(t, err)
	viewer, err := authService.Register(ctx, auth.RegisterRequest{
		Email: "album-http-viewer@example.com", Password: "SecurePass123!", Name: "Album Viewer",
	})
	require.NoError(t, err)
	outsider, err := authService.Register(ctx, auth.RegisterRequest{
		Email: "album-http-outsider@example.com", Password: "SecurePass123!", Name: "Album Outsider",
	})
	require.NoError(t, err)

	album, err := tdb.Queries.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:   pgutil.UUIDToPgtype(uuid.MustParse(owner.User.ID)),
		AlbumName: "private compatibility album",
	})
	require.NoError(t, err)
	require.NoError(t, tdb.Queries.AddUserToAlbum(ctx, sqlc.AddUserToAlbumParams{
		AlbumsId: album.ID,
		UsersId:  pgutil.UUIDToPgtype(uuid.MustParse(viewer.User.ID)),
		Role:     "viewer",
	}))

	userService, err := users.NewService(tdb.Queries, cfg)
	require.NoError(t, err)
	srv := &Server{
		config:      cfg,
		db:          conn,
		authService: authService,
		services:    services{userService: userService},
	}
	httpServer := httptest.NewServer(mustHTTPHandler(t, srv))
	t.Cleanup(httpServer.Close)

	for name, tc := range map[string]struct {
		token      string
		wantStatus int
	}{
		"owner":    {token: owner.AccessToken, wantStatus: http.StatusOK},
		"viewer":   {token: viewer.AccessToken, wantStatus: http.StatusOK},
		"outsider": {token: outsider.AccessToken, wantStatus: http.StatusForbidden},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/api/albums/"+album.ID.String(), nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, resp.StatusCode, string(body))
			if tc.wantStatus == http.StatusForbidden {
				assert.NotContains(t, string(body), album.AlbumName)
				assert.NotContains(t, string(body), owner.User.Email)
			}
		})
	}
}

func TestIntegration_AlbumAuthorizationRoles(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	ctx := context.Background()
	tdb := testdb.SetupTestDB(t)
	conn, err := db.New(ctx, tdb.ConnStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	srv := &Server{db: conn}

	ownerID := tdb.CreateTestUser(t, "album-owner@example.com")
	viewerID := tdb.CreateTestUser(t, "album-viewer@example.com")
	editorID := tdb.CreateTestUser(t, "album-editor@example.com")
	outsiderID := tdb.CreateTestUser(t, "album-outsider@example.com")
	album, err := tdb.Queries.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:   pgutil.UUIDToPgtype(ownerID),
		AlbumName: "private album",
	})
	require.NoError(t, err)
	for userID, role := range map[uuid.UUID]string{viewerID: "viewer", editorID: "editor"} {
		require.NoError(t, tdb.Queries.AddUserToAlbum(ctx, sqlc.AddUserToAlbumParams{
			AlbumsId: album.ID,
			UsersId:  pgutil.UUIDToPgtype(userID),
			Role:     role,
		}))
	}

	ctxFor := func(userID uuid.UUID) context.Context {
		return auth.WithClaims(ctx, &auth.Claims{UserID: userID.String()})
	}

	_, _, err = srv.authorizedAlbum(ctxFor(ownerID), album.ID, albumAccessManage)
	require.NoError(t, err)
	_, _, err = srv.authorizedAlbum(ctxFor(viewerID), album.ID, albumAccessView)
	require.NoError(t, err)
	_, _, err = srv.authorizedAlbum(ctxFor(viewerID), album.ID, albumAccessEdit)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	_, _, err = srv.authorizedAlbum(ctxFor(editorID), album.ID, albumAccessEdit)
	require.NoError(t, err)
	_, _, err = srv.authorizedAlbum(ctxFor(editorID), album.ID, albumAccessManage)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	_, _, err = srv.authorizedAlbum(ctxFor(outsiderID), album.ID, albumAccessView)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestIntegration_HLSAssetAuthorization(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	ctx := context.Background()
	tdb := testdb.SetupTestDB(t)
	conn, err := db.New(ctx, tdb.ConnStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	srv := &Server{db: conn}

	ownerID := tdb.CreateTestUser(t, "hls-owner@example.com")
	sharedID := tdb.CreateTestUser(t, "hls-shared@example.com")
	outsiderID := tdb.CreateTestUser(t, "hls-outsider@example.com")
	asset, err := tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "hls-video",
		OwnerId:          pgutil.UUIDToPgtype(ownerID),
		DeviceId:         "test-device",
		Type:             "VIDEO",
		OriginalPath:     "/test/path/hls-video.mp4",
		OriginalFileName: "hls-video.mp4",
		Checksum:         []byte("hls-video-checksum"),
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)
	album, err := tdb.Queries.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:   pgutil.UUIDToPgtype(ownerID),
		AlbumName: "shared video",
	})
	require.NoError(t, err)
	require.NoError(t, tdb.Queries.AddAssetToAlbum(ctx, sqlc.AddAssetToAlbumParams{
		AlbumsId: album.ID,
		AssetsId: asset.ID,
	}))
	require.NoError(t, tdb.Queries.AddUserToAlbum(ctx, sqlc.AddUserToAlbumParams{
		AlbumsId: album.ID,
		UsersId:  pgutil.UUIDToPgtype(sharedID),
		Role:     "viewer",
	}))

	ctxFor := func(userID uuid.UUID) context.Context {
		return auth.WithClaims(ctx, &auth.Claims{UserID: userID.String()})
	}
	_, _, err = srv.hlsAsset(ctxFor(ownerID), asset.ID.String())
	require.NoError(t, err)
	_, _, err = srv.hlsAsset(ctxFor(sharedID), asset.ID.String())
	require.NoError(t, err)
	_, _, err = srv.hlsAsset(ctxFor(outsiderID), asset.ID.String())
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}
