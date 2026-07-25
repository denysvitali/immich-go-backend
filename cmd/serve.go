package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/embedded"
	"github.com/denysvitali/immich-go-backend/internal/server"
)

// grpcReadyTimeout bounds how long serve waits for its own gRPC listener to
// accept the loopback connection the HTTP gateway needs for streaming RPCs. It
// is a local dial, so anything slower than this means the listener never came
// up and failing fast beats serving a gateway that cannot reach SyncService.
const grpcReadyTimeout = 10 * time.Second

// httpShutdownTimeout bounds the graceful HTTP drain on SIGTERM.
const httpShutdownTimeout = 10 * time.Second

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Immich backend server",
	Long:  `Start the Immich backend server with HTTP and gRPC endpoints.`,
	RunE:  runServer,
}

// runServer is the serve lifecycle: open storage, build the server, start both
// listeners, then block until a signal arrives. Each stage is a function below
// so the order of operations is readable in one screen.
func runServer(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	database, closeDatabase, err := openDatabase(ctx)
	if err != nil {
		return err
	}
	defer closeDatabase()

	srv, err := server.NewServer(cfg, database)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Registered before either listener starts so a signal arriving during
	// startup is not lost.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	grpcAddr := cfg.Server.GRPCAddress
	if err := startGRPC(srv, grpcAddr); err != nil {
		return err
	}

	grpcClientConn, err := dialLocalGRPC(ctx, grpcAddr)
	if err != nil {
		return err
	}
	srv.SetGRPCConn(grpcClientConn)

	httpServer := startHTTP(srv, cfg.Server.Address)

	<-sigCh
	shutdown(httpServer, srv)
	return nil
}

// openDatabase starts the embedded PostgreSQL when enabled, connects, and runs
// migrations if configured. The returned function releases both the connection
// and the embedded runtime, in that order, and must be called by the caller.
func openDatabase(ctx context.Context) (*db.Conn, func(), error) {
	// When enabled (IMMICH_EMBEDDED_DB=true) we replace cfg.Database.URL with
	// the generated DSN and run migrations inside the same lifecycle, so a
	// single-binary demo deployment only needs the Immich web bundle as a
	// second artefact.
	var embeddedPG *embedded.Runtime
	if embedded.IsEnabled() {
		runtime, err := embedded.Start(embedded.DefaultConfig())
		if err != nil {
			return nil, nil, fmt.Errorf("embedded postgres: %w", err)
		}
		embeddedPG = runtime
		cfg.Database.URL = runtime.DSN()
	}

	database, err := db.New(ctx, cfg.Database.URL)
	if err != nil {
		if embeddedPG != nil {
			_ = embeddedPG.Stop()
		}
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	cleanup := func() {
		if err := database.Close(); err != nil {
			logrus.WithError(err).Error("Failed to close database connection")
		}
		if embeddedPG != nil {
			if err := embeddedPG.Stop(); err != nil {
				logrus.WithError(err).Error("Failed to stop embedded postgres")
			}
		}
	}

	// Auto-migrate when configured (default: true). Kept opt-out so production
	// deployments using a managed DB can disable migrations during `serve` and
	// rely on the explicit `migrate` subcommand or a separate release_command
	// step.
	if cfg.Database.AutoMigrate {
		logrus.Info("running database migrations")
		if err := db.RunMigrations(ctx, database.DB()); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("run migrations: %w", err)
		}
	}

	return database, cleanup, nil
}

// startGRPC binds the gRPC listener and serves it in the background. A serve
// error after startup is logged, not returned: by then the HTTP listener owns
// the process lifetime and the signal handler decides when to stop.
func startGRPC(srv *server.Server, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	go func() {
		logrus.Infof("Starting gRPC server on %s", addr)
		if err := srv.ServeGRPC(listener); err != nil {
			logrus.WithError(err).Error("gRPC server failed")
		}
	}()

	return nil
}

// dialLocalGRPC connects back to our own gRPC listener so the HTTP gateway can
// forward streaming SyncService RPCs over a real gRPC connection, and waits
// until that connection is usable.
func dialLocalGRPC(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client for %s: %w", addr, err)
	}

	readyCtx, cancel := context.WithTimeout(ctx, grpcReadyTimeout)
	defer cancel()

	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return conn, nil
		}
		if !conn.WaitForStateChange(readyCtx, state) {
			_ = conn.Close()
			return nil, fmt.Errorf("gRPC client for %s did not become ready in time", addr)
		}
	}
}

// startHTTP serves the REST gateway in the background and returns the server so
// the caller can drain it on shutdown.
func startHTTP(srv *server.Server, addr string) *http.Server {
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv.HTTPHandler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logrus.Infof("Starting HTTP server on %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("HTTP server failed")
		}
	}()

	return httpServer
}

// shutdown drains HTTP first so in-flight requests can finish against a still
// running gRPC server, then stops gRPC and the job workers.
func shutdown(httpServer *http.Server, srv *server.Server) {
	logrus.Info("Shutting down servers...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Error("Failed to shutdown HTTP server gracefully")
	}

	srv.Stop()
}
