package main

import (
	"context"
	"crypto/tls"
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

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/embedded"
	"github.com/denysvitali/immich-go-backend/internal/server"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
)

// grpcReadyTimeout bounds how long serve waits for its own gRPC listener to
// accept the loopback connection the HTTP gateway needs for streaming RPCs. It
// is a local dial, so anything slower than this means the listener never came
// up and failing fast beats serving a gateway that cannot reach SyncService.
const grpcReadyTimeout = 10 * time.Second

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

	telemetryProvider, err := telemetry.NewProvider(
		cfg.Telemetry,
		telemetry.WithPrometheusMetrics(cfg.Server.MetricsEnabled),
	)
	if err != nil {
		return fmt.Errorf("initialize telemetry: %w", err)
	}
	defer shutdownTelemetry(telemetryProvider, cfg.Server.ShutdownTimeout)

	database, closeDatabase, err := openDatabase(ctx)
	if err != nil {
		return err
	}
	defer closeDatabase()

	srv, err := server.NewServer(cfg, database)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	srv.SetMetricsHandler(telemetryProvider.MetricsHandler())

	// Registered before either listener starts so a signal arriving during
	// startup is not lost.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	serverStopped := false
	defer func() {
		if !serverStopped {
			srv.Stop()
		}
	}()

	grpcAddr := cfg.Server.GRPCAddress
	grpcErrCh, err := startGRPC(srv, grpcAddr)
	if err != nil {
		return err
	}

	grpcClientConn, err := dialLocalGRPC(ctx, grpcAddr)
	if err != nil {
		return err
	}
	srv.SetGRPCConn(grpcClientConn)

	httpServer, httpErrCh, err := startHTTP(srv, cfg.Server)
	if err != nil {
		return err
	}

	select {
	case <-sigCh:
		shutdown(httpServer, srv, cfg.Server.ShutdownTimeout)
		serverStopped = true
		return nil
	case err := <-grpcErrCh:
		shutdown(httpServer, srv, cfg.Server.ShutdownTimeout)
		serverStopped = true
		return unexpectedServerStop("gRPC", err)
	case err := <-httpErrCh:
		shutdown(httpServer, srv, cfg.Server.ShutdownTimeout)
		serverStopped = true
		return unexpectedServerStop("HTTP", err)
	}
}

func shutdownTelemetry(provider *telemetry.Provider, timeout time.Duration) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := provider.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("Failed to shutdown telemetry providers")
	}
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

	database, err := db.NewWithOptions(ctx, cfg.Database.URL, databaseOptions())
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

func databaseOptions() db.Options {
	return db.Options{
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MinIdleConns:    cfg.Database.MinIdleConns,
		MaxConnLifetime: cfg.Database.MaxConnLifetime,
		ConnectTimeout:  cfg.Database.ConnectTimeout,
		QueryTimeout:    cfg.Database.QueryTimeout,
		LogQueries:      cfg.Database.LogQueries,
	}
}

// startGRPC binds the gRPC listener and returns asynchronous Serve failures to
// the lifecycle owner so a half-dead process never remains healthy forever.
func startGRPC(srv *server.Server, addr string) (<-chan error, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	errCh := make(chan error, 1)
	go func() {
		logrus.Infof("Starting gRPC server on %s", addr)
		errCh <- srv.ServeGRPC(listener)
	}()

	return errCh, nil
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
// the caller can drain it on shutdown. Building the handler can fail when a
// service's gateway registration is rejected, which would leave that service
// unreachable over REST — so it aborts startup instead.
func startHTTP(srv *server.Server, serverConfig config.ServerConfig) (*http.Server, <-chan error, error) {
	listener, err := net.Listen("tcp", serverConfig.Address)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen on %s: %w", serverConfig.Address, err)
	}

	handler, err := srv.HTTPHandler()
	if err != nil {
		_ = listener.Close()
		return nil, nil, fmt.Errorf("build HTTP handler: %w", err)
	}

	httpServer := &http.Server{
		Addr:         serverConfig.Address,
		Handler:      handler,
		ReadTimeout:  serverConfig.ReadTimeout,
		WriteTimeout: serverConfig.WriteTimeout,
		IdleTimeout:  serverConfig.IdleTimeout,
	}
	tlsEnabled := serverConfig.TLS.Enabled
	if tlsEnabled {
		certificate, err := tls.LoadX509KeyPair(serverConfig.TLS.CertFile, serverConfig.TLS.KeyFile)
		if err != nil {
			_ = listener.Close()
			return nil, nil, fmt.Errorf("load HTTP TLS certificate: %w", err)
		}
		tlsConfig := &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{certificate},
		}
		httpServer.TLSConfig = tlsConfig
	}

	errCh := make(chan error, 1)
	go func() {
		logrus.Infof("Starting HTTP server on %s", serverConfig.Address)
		if tlsEnabled {
			errCh <- httpServer.ServeTLS(listener, "", "")
			return
		}
		errCh <- httpServer.Serve(listener)
	}()

	return httpServer, errCh, nil
}

// shutdown drains HTTP first so in-flight requests can finish against a still
// running gRPC server, then stops gRPC and the job workers.
func shutdown(httpServer *http.Server, srv *server.Server, timeout time.Duration) {
	logrus.Info("Shutting down servers...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Error("Failed to shutdown HTTP server gracefully")
	}

	srv.Stop()
}

func unexpectedServerStop(name string, err error) error {
	if err == nil {
		return fmt.Errorf("%s server stopped unexpectedly", name)
	}
	return fmt.Errorf("%s server stopped unexpectedly: %w", name, err)
}
