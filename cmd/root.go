package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/server"
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
	rootCmd.PersistentFlags().StringP("log-format", "f", "json", "log format (text, json)")
	rootCmd.PersistentFlags().StringP("port", "p", "8080", "HTTP port")
	rootCmd.PersistentFlags().StringP("grpc-port", "g", "9090", "gRPC port")
	rootCmd.PersistentFlags().String("database-url", "", "PostgreSQL database URL")

	_ = viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("log.format", rootCmd.PersistentFlags().Lookup("log-format"))
	_ = viper.BindPFlag("server.port", rootCmd.PersistentFlags().Lookup("port"))
	_ = viper.BindPFlag("server.grpc_port", rootCmd.PersistentFlags().Lookup("grpc-port"))
	_ = viper.BindPFlag("database.url", rootCmd.PersistentFlags().Lookup("database-url"))
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
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err == nil {
		logrus.WithField("config", viper.ConfigFileUsed()).Info("Using config file")
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// Initialize configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	// Setup logger
	setupLogger(cfg.Logging.Level, cfg.Logging.Format)

	logrus.Info("Starting Immich Go Backend Server")

	ctx := context.Background()
	conn, err := db.New(ctx, cfg.Database.URL)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to database")
	}
	defer conn.Close()

	// Create server
	srv := server.NewServer(cfg, conn)

	// Start gRPC server
	grpcListener, err := net.Listen("tcp", cfg.Server.GRPCAddress)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to listen on gRPC address")
	}

	go func() {
		logrus.WithField("address", cfg.Server.GRPCAddress).Info("Starting gRPC server")
		if err := srv.ServeGRPC(grpcListener); err != nil {
			logrus.WithError(err).Fatal("gRPC server failed")
		}
	}()

	// Start HTTP server
	httpServer := &http.Server{
		Addr:    cfg.Server.Address,
		Handler: cors(srv.HTTPHandler()),
	}

	go func() {
		logrus.WithField("address", cfg.Server.Address).Info("Starting HTTP server")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

func cors(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func setupLogger(level string, format string) {
	switch strings.ToLower(format) {
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	default:
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

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
