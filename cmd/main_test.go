package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigForCommandJWTSecretRequirements(t *testing.T) {
	t.Setenv("AUTH_JWT_SECRET", "")

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("auth:\n  jwt_secret: \"\"\n"), 0o600))

	for _, cmd := range []*cobra.Command{versionCmd, migrateCmd, openapiCoverageCmd} {
		t.Run(cmd.Name()+" works without runtime secret", func(t *testing.T) {
			cfg, err := loadConfigForCommand(cmd, configPath)
			require.NoError(t, err)
			require.Empty(t, cfg.Auth.JWTSecret)
		})
	}

	t.Run("serve rejects missing runtime secret", func(t *testing.T) {
		_, err := loadConfigForCommand(serveCmd, configPath)
		require.ErrorContains(t, err, "AUTH_JWT_SECRET is required")
	})
}

func TestConfigureLoggingHonorsOutputAndRejectsInvalidValues(t *testing.T) {
	previousOutput := logrus.StandardLogger().Out
	previousFormatter := logrus.StandardLogger().Formatter
	previousLevel := logrus.GetLevel()
	t.Cleanup(func() {
		logrus.SetOutput(previousOutput)
		logrus.SetFormatter(previousFormatter)
		logrus.SetLevel(previousLevel)
	})

	require.NoError(t, configureLogging(config.LoggingConfig{
		Level: "debug", Format: "text", Output: "stdout",
	}))
	assert.Same(t, os.Stdout, logrus.StandardLogger().Out)
	assert.Equal(t, logrus.DebugLevel, logrus.GetLevel())
	assert.IsType(t, &logrus.TextFormatter{}, logrus.StandardLogger().Formatter)

	require.NoError(t, configureLogging(config.LoggingConfig{
		Level: "info", Format: "json", Output: "stderr",
	}))
	assert.Same(t, os.Stderr, logrus.StandardLogger().Out)
	assert.IsType(t, &logrus.JSONFormatter{}, logrus.StandardLogger().Formatter)

	for _, logging := range []config.LoggingConfig{
		{Level: "verbose", Format: "json", Output: "stdout"},
		{Level: "info", Format: "xml", Output: "stdout"},
		{Level: "info", Format: "json", Output: "file"},
	} {
		assert.Error(t, configureLogging(logging))
	}
}
