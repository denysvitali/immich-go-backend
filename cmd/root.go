package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/database"
	"github.com/denysvitali/immich-go-backend/internal/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "immich-go-backend",
	Short: "Immich Go Backend Server",
	Long:  "A Go-based backend server for Immich photo management system using gRPC-Gateway",
	Run:   runServer,
}

var cfgFile string

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringP("port", "p", "8080", "HTTP port")
	rootCmd.PersistentFlags().StringP("grpc-port", "g", "9090", "gRPC port")
	rootCmd.PersistentFlags().String("database-url", "", "PostgreSQL database URL")

	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("server.port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("server.grpc_port", rootCmd.PersistentFlags().Lookup("grpc-port"))
	viper.BindPFlag("database.url", rootCmd.PersistentFlags().Lookup("database-url"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("IMMICH")

	if err := viper.ReadInConfig(); err == nil {
		logrus.WithField("config", viper.ConfigFileUsed()).Info("Using config file")
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// Initialize configuration
	cfg := config.Load()

	// Setup logger
	setupLogger(cfg.Log.Level)

	logrus.Info("Starting Immich Go Backend Server")

	// Initialize database
	db, err := database.Initialize(cfg.Database.URL)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize database")
	}

	// Create server
	srv := server.NewServer(cfg, db)

	// Start gRPC server
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.Server.GRPCPort))
	if err != nil {
		logrus.WithError(err).Fatal("Failed to listen on gRPC port")
	}

	go func() {
		logrus.WithField("port", cfg.Server.GRPCPort).Info("Starting gRPC server")
		if err := srv.ServeGRPC(grpcListener); err != nil {
			logrus.WithError(err).Fatal("gRPC server failed")
		}
	}()

	// Start HTTP server
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Server.Port),
		Handler: srv.HTTPHandler(),
	}

	go func() {
		logrus.WithField("port", cfg.Server.Port).Info("Starting HTTP server")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logrus.Info("Shutting down servers...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("HTTP server shutdown failed")
	}

	srv.Stop()
	logrus.Info("Servers stopped")
}

func setupLogger(level string) {
	logrus.SetFormatter(&logrus.JSONFormatter{})

	switch level {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}
}
