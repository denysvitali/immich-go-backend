package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/denysvitali/immich-go-backend/internal/config"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/sync"
)

func TestSetGRPCConnChangesSyncRegistration(t *testing.T) {
	// Build a minimal Server so we can inspect the HTTP gateway behaviour
	// without a database. Only SyncService is registered with a real
	// implementation; the rest are left as nil because their handlers are
	// only invoked when their routes are hit.
	srv := &Server{
		config:     &config.Config{},
		grpcServer: grpc.NewServer(),
		syncServer: sync.NewServer(sync.NewService(nil, nil)),
	}
	immichv1.RegisterSyncServiceServer(srv.grpcServer, srv.syncServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.ServeGRPC(lis) }()
	defer srv.Stop()

	// Without SetGRPCConn the in-process gateway registration is used, and
	// streaming RPCs are reported as unimplemented.
	serverWithoutConn := httptest.NewServer(srv.HTTPHandler())
	defer serverWithoutConn.Close()

	resp, err := http.Post(serverWithoutConn.URL+"/sync/stream", "application/json", nil)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)

	// After setting a real client connection, SyncService is registered via
	// the grpc-gateway client path, which supports streaming.
	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	srv.SetGRPCConn(conn)

	serverWithConn := httptest.NewServer(srv.HTTPHandler())
	defer serverWithConn.Close()

	resp, err = http.Post(serverWithConn.URL+"/sync/stream", "application/json", nil)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.NotEqual(t, http.StatusNotImplemented, resp.StatusCode, "sync stream should no longer be in-process unimplemented")
}

func TestSetGRPCConnClosesOnStop(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := &Server{grpcServer: grpc.NewServer()}
	go func() { _ = srv.ServeGRPC(lis) }()

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	srv.SetGRPCConn(conn)
	srv.Stop()

	// Stop must close the client connection.
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_ = ctx
	assert.Equal(t, connectivity.Shutdown, conn.GetState())
}
