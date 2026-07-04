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

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/embedded"
	"github.com/denysvitali/immich-go-backend/internal/server"
)

var (
	cfgFile string
	cfg     *config.Config
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "immich-go-backend",
	Short: "Immich Go Backend Server",
	Long:  `A Go implementation of the Immich backend server providing photo and video management capabilities.`,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Immich backend server",
	Long:  `Start the Immich backend server with HTTP and gRPC endpoints.`,
	RunE:  runServer,
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Apply database migrations to set up or update the database schema.`,
	RunE:  runMigrations,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	if cfgFile == "" {
		cfgFile = "./config.yaml"
	}

	var err error
	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	// Setup logging
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	if cfg.Logging.Format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{})
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Optionally start an embedded Postgres before connecting. When enabled
	// (IMMICH_EMBEDDED_DB=true) we replace cfg.Database.URL with the
	// generated DSN and run migrations inside the same lifecycle so a
	// single-binary demo deployment only needs the Immich web bundle as a
	// second artefact.
	var embeddedPG *embedded.Runtime
	if embedded.IsEnabled() {
		runtime, err := embedded.Start(embedded.DefaultConfig())
		if err != nil {
			return fmt.Errorf("embedded postgres: %w", err)
		}
		embeddedPG = runtime
		cfg.Database.URL = runtime.DSN()
	}

	// Connect to database
	database, err := db.New(ctx, cfg.Database.URL)
	if err != nil {
		if embeddedPG != nil {
			_ = embeddedPG.Stop()
		}
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			logrus.WithError(err).Error("Failed to close database connection")
		}
		if embeddedPG != nil {
			if err := embeddedPG.Stop(); err != nil {
				logrus.WithError(err).Error("Failed to stop embedded postgres")
			}
		}
	}()

	// Auto-migrate when configured (default: true). Kept opt-out so
	// production deployments using a managed DB can disable migrations
	// during `serve` and rely on the explicit `migrate` subcommand or a
	// separate release_command step.
	if cfg.Database.AutoMigrate {
		logrus.Info("running database migrations")
		if err := db.RunMigrations(ctx, database.DB()); err != nil {
			return fmt.Errorf("run migrations: %w", err)
		}
	}

	// Create server
	srv, err := server.NewServer(cfg, database)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start gRPC server
	grpcAddr := cfg.Server.GRPCAddress
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", grpcAddr, err)
	}

	go func() {
		logrus.Infof("Starting gRPC server on %s", grpcAddr)
		if err := srv.ServeGRPC(grpcListener); err != nil {
			logrus.WithError(err).Error("gRPC server failed")
		}
	}()

	// Dial the local gRPC server so the HTTP gateway can forward streaming
	// SyncService RPCs over a real gRPC connection.
	grpcClientConn, err := grpc.NewClient(grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to create gRPC client for %s: %w", grpcAddr, err)
	}

	readyCtx, readyCancel := context.WithTimeout(ctx, 10*time.Second)
	defer readyCancel()
	grpcClientConn.Connect()
	for {
		state := grpcClientConn.GetState()
		if state == connectivity.Ready {
			break
		}
		if !grpcClientConn.WaitForStateChange(readyCtx, state) {
			_ = grpcClientConn.Close()
			return fmt.Errorf("gRPC client for %s did not become ready in time", grpcAddr)
		}
	}

	srv.SetGRPCConn(grpcClientConn)

	// Start HTTP server
	httpAddr := cfg.Server.Address
	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      srv.HTTPHandler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logrus.Infof("Starting HTTP server on %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("HTTP server failed")
		}
	}()

	// Wait for signal
	<-sigCh
	logrus.Info("Shutting down servers...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Error("Failed to shutdown HTTP server gracefully")
	}

	srv.Stop()

	return nil
}

func runMigrations(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Connect to database
	database, err := db.New(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			logrus.WithError(err).Error("Failed to close database connection")
		}
	}()

	logrus.Info("Running database migrations...")

	// Run migrations using the migration system
	if err := db.RunMigrations(ctx, database.DB()); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logrus.Info("Migrations completed successfully")

	return nil
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Immich Go Backend\n")
		fmt.Printf("Version: %s\n", server.Version)
		fmt.Printf("Source Commit: %s\n", server.SourceCommit)
		fmt.Printf("Source Ref: %s\n", server.SourceRef)
		fmt.Printf("Source URL: %s\n", server.SourceUrl)
	},
}
