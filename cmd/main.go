// Command immich-go-backend serves the Immich-compatible API.
//
// This file is wiring only: flag parsing, configuration loading, logger setup,
// and command registration. Each subcommand's behaviour lives in its own file
// (serve.go, migrate.go, openapi_coverage.go).
package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/server"
)

var (
	cfgFile string

	// cfg is populated by initConfig before any RunE executes, via
	// cobra.OnInitialize. Subcommands read it directly.
	cfg *config.Config
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

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("Immich Go Backend\n")
		fmt.Printf("Version: %s\n", server.Version)
		fmt.Printf("Source Commit: %s\n", server.SourceCommit)
		fmt.Printf("Source Ref: %s\n", server.SourceRef)
		fmt.Printf("Source URL: %s\n", server.SourceUrl)
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(versionCmd)
}

// initConfig loads the configuration and applies the logging settings. A
// failure here is fatal: every subcommand needs a valid config, and continuing
// with a zero-value one would fail later with a worse message.
func initConfig() {
	if cfgFile == "" {
		cfgFile = "./config.yaml"
	}

	var err error
	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

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
