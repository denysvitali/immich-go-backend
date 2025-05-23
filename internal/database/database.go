package database

import (
	"fmt"

	"github.com/denysvitali/immich-go-backend/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	return db.AutoMigrate(
		&models.User{},
		&models.Album{},
		&models.Asset{},
		&models.Library{},
		&models.APIKey{},
		&models.Session{},
		&models.Tag{},
		&models.Activity{},
		&models.Notification{},
		&models.Partner{},
		&models.Person{},
		&models.SharedLink{},
		&models.Stack{},
	)
}
