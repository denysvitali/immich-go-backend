//go:build integration
// +build integration

package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/denysvitali/immich-go-backend/internal/storage"
)

func newSyncGRPCIntegrationTestEnv(t *testing.T) (*Server, *db.Conn, func()) {
	t.Helper()

	tdb := testdb.SetupTestDB(t)

	database, err := db.New(context.Background(), tdb.ConnStr)
	require.NoError(t, err)

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address:     "127.0.0.1:0",
			GRPCAddress: "127.0.0.1:0",
		},
		Database: config.DatabaseConfig{
			URL:            tdb.ConnStr,
			AutoMigrate:    false,
			ConnectTimeout: 30 * time.Second,
		},
		Storage: storage.StorageConfig{
			Backend: "local",
			Local: storage.LocalConfig{
				RootPath: tmpDir,
				FileMode: "0644",
				DirMode:  "0755",
			},
		},
		Auth: config.AuthConfig{
			JWTSecret:           "test-secret-for-sync-grpc-integration",
			JWTExpiry:           time.Hour,
			RegistrationEnabled: true,
		},
		Jobs: config.JobsConfig{
			Enabled: false,
		},
	}

	srv, err := NewServer(cfg, database)
	require.NoError(t, err)

	cleanup := func() {
		srv.Stop()
		_ = database.Close()
	}

	return srv, database, cleanup
}

func startTestGRPCServer(t *testing.T, srv *Server) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		if err := srv.ServeGRPC(listener); err != nil {
			t.Logf("gRPC server returned: %v", err)
		}
	}()
	return listener.Addr().String()
}

func dialTestGRPC(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			break
		}
		if !conn.WaitForStateChange(ctx, state) {
			_ = conn.Close()
			t.Fatalf("gRPC client for %s did not become ready in time", addr)
		}
	}
	return conn
}

func createSyncTestUser(t *testing.T, ctx context.Context, database *db.Conn) (uuid.UUID, string) {
	t.Helper()

	userID := uuid.New()
	var userUUID pgtype.UUID
	require.NoError(t, userUUID.Scan(userID.String()))
	_, err := database.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "sync-grpc-" + userID.String() + "@example.com",
		Name:     "Sync gRPC Test User",
		Password: "hashed-password-not-used-in-tests",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	token, err := auth.NewService(config.AuthConfig{
		JWTSecret: "test-secret-for-sync-grpc-integration",
		JWTExpiry: time.Hour,
	}, database.Queries).GenerateToken(userID.String(), "sync-grpc-"+userID.String()+"@example.com", time.Hour)
	require.NoError(t, err)

	return userID, token
}

func TestSyncServiceHandlerUsesClientConn(t *testing.T) {
	if os.Getenv("SKIP_DOCKER_TESTS") != "" {
		t.Skip("Docker tests disabled")
	}

	srv, database, cleanup := newSyncGRPCIntegrationTestEnv(t)
	defer cleanup()

	grpcAddr := startTestGRPCServer(t, srv)
	conn := dialTestGRPC(t, grpcAddr)
	defer func() { _ = conn.Close() }()

	srv.SetGRPCConn(conn)

	server := httptest.NewServer(srv.HTTPHandler())
	defer server.Close()

	_, token := createSyncTestUser(t, context.Background(), database)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/sync/stream", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode, "sync stream endpoint should be registered: %s", body)
	assert.NotEqual(t, http.StatusNotImplemented, resp.StatusCode, "streaming should be forwarded over gRPC client connection, not in-process: %s", body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "authenticated stream request should succeed: %s", body)
}

func TestSyncServiceHandlerFallsBackToServer(t *testing.T) {
	if os.Getenv("SKIP_DOCKER_TESTS") != "" {
		t.Skip("Docker tests disabled")
	}

	srv, _, cleanup := newSyncGRPCIntegrationTestEnv(t)
	defer cleanup()

	_ = startTestGRPCServer(t, srv)
	// Intentionally do NOT call srv.SetGRPCConn so the gateway falls back to
	// the in-process RegisterSyncServiceHandlerServer, which reports streaming
	// RPCs as unimplemented.

	server := httptest.NewServer(srv.HTTPHandler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/sync/stream", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode, "in-process sync stream should be unimplemented: %s", body)
}

func TestSetGRPCConnIsClosedOnStop(t *testing.T) {
	if os.Getenv("SKIP_DOCKER_TESTS") != "" {
		t.Skip("Docker tests disabled")
	}

	srv, _, cleanup := newSyncGRPCIntegrationTestEnv(t)
	defer cleanup()

	grpcAddr := startTestGRPCServer(t, srv)
	conn := dialTestGRPC(t, grpcAddr)

	srv.SetGRPCConn(conn)
	srv.Stop()

	state := conn.GetState()
	assert.Equal(t, connectivity.Shutdown, state, "client connection should be closed after Stop")
}
