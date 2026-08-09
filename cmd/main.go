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

	// cfg is populated by the root command's persistent pre-run hook before
	// any RunE executes. Subcommands read it directly.
	cfg *config.Config
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:               "immich-go-backend",
	Short:             "Immich Go Backend Server",
	Long:              `A Go implementation of the Immich backend server providing photo and video management capabilities.`,
	PersistentPreRunE: initConfig,
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(versionCmd)
}

// initConfig loads the configuration and applies the logging settings. Only
// serve needs full runtime validation: utility commands can parse their inputs
// without a JWT signing key because they never issue or validate tokens.
func initConfig(cmd *cobra.Command, _ []string) error {
	if cfgFile == "" {
		cfgFile = "./config.yaml"
	}

	var err error
	cfg, err = loadConfigForCommand(cmd, cfgFile)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	return configureLogging(cfg.Logging)
}

func configureLogging(logging config.LoggingConfig) error {
	level, err := logrus.ParseLevel(logging.Level)
	if err != nil {
		return fmt.Errorf("invalid log level %q: %w", logging.Level, err)
	}
	logrus.SetLevel(level)

	switch logging.Format {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	default:
		return fmt.Errorf("invalid log format %q; expected json or text", logging.Format)
	}

	switch logging.Output {
	case "stdout":
		logrus.SetOutput(os.Stdout)
	case "stderr":
		logrus.SetOutput(os.Stderr)
	default:
		return fmt.Errorf("unsupported log output %q; expected stdout or stderr", logging.Output)
	}
	return nil
}

func loadConfigForCommand(cmd *cobra.Command, configPath string) (*config.Config, error) {
	if cmd.Name() == serveCmd.Name() {
		return config.LoadConfig(configPath)
	}
	return config.LoadConfigWithoutValidation(configPath)
}
