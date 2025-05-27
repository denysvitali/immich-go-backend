package database

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/denysvitali/immich-go-backend/internal/models"
)

func Initialize(databaseURL string) (*gorm.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	logrus.Info("Connecting to database...")

	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(postgres.Open(databaseURL), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tables
	logrus.Info("Running database migrations...")
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	logrus.Info("Database initialized successfully")
	return db, nil
}

func migrate(db *gorm.DB) error {
	// First, migrate tables that don't depend on Asset
	if err := db.AutoMigrate(
		&models.User{},
		&models.Library{},
		&models.APIKey{},
		&models.Session{},
		&models.Notification{},
		&models.Partner{},
	); err != nil {
		return fmt.Errorf("failed to migrate basic tables: %w", err)
	}

	// Now migrate tables that depend on Asset
	if err := db.AutoMigrate(
		&models.Album{},
		&models.Tag{},
		&models.Activity{},
		&models.Person{},
		&models.SharedLink{},
	); err != nil {
		return fmt.Errorf("failed to migrate asset-dependent tables: %w", err)
	}

	return nil
}
