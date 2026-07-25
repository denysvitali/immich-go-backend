package main

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/denysvitali/immich-go-backend/internal/db"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Apply database migrations to set up or update the database schema.`,
	RunE:  runMigrations,
}

func runMigrations(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

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

	if err := db.RunMigrations(ctx, database.DB()); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logrus.Info("Migrations completed successfully")

	return nil
}
