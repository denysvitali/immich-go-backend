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

	// Create Asset and Stack tables with circular dependency handling
	if err := migrateAssetAndStack(db); err != nil {
		return fmt.Errorf("failed to migrate asset and stack tables: %w", err)
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

func migrateAssetAndStack(db *gorm.DB) error {
	// For PostgreSQL, we need to handle circular dependencies by creating tables
	// without foreign key constraints first, then adding constraints later
	
	// Check if tables already exist
	if db.Migrator().HasTable(&models.Asset{}) && db.Migrator().HasTable(&models.Stack{}) {
		// Tables exist, just run normal migration to update schema
		return db.AutoMigrate(&models.Asset{}, &models.Stack{})
	}

	// Create Asset table without foreign key constraints by using raw SQL
	assetTableSQL := `
		CREATE TABLE IF NOT EXISTS assets (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			user_id UUID NOT NULL,
			library_id UUID,
			device_asset_id VARCHAR(255) NOT NULL,
			device_id VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			original_file_name VARCHAR(255) NOT NULL,
			original_path TEXT NOT NULL,
			resize_path TEXT,
			web_path TEXT,
			thumbnail_path TEXT,
			thumbhash TEXT,
			encoded_video_path TEXT,
			duration VARCHAR(255),
			is_visible BOOLEAN DEFAULT true,
			is_archived BOOLEAN DEFAULT false,
			is_favorite BOOLEAN DEFAULT false,
			is_read_only BOOLEAN DEFAULT false,
			is_external BOOLEAN DEFAULT false,
			is_offline BOOLEAN DEFAULT false,
			checksum BYTEA,
			file_created_at TIMESTAMPTZ,
			file_modified_at TIMESTAMPTZ,
			local_date_time TIMESTAMPTZ,
			updated_at_utc TIMESTAMPTZ,
			is_motion BOOLEAN DEFAULT false,
			is_live BOOLEAN DEFAULT false,
			live_photo_video_id UUID,
			stack_id UUID
		);
	`
	
	if err := db.Exec(assetTableSQL).Error; err != nil {
		return fmt.Errorf("failed to create assets table: %w", err)
	}

	// Create Stack table without circular foreign key constraint
	stackTableSQL := `
		CREATE TABLE IF NOT EXISTS stacks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			primary_asset_id UUID NOT NULL,
			asset_count BIGINT
		);
	`
	
	if err := db.Exec(stackTableSQL).Error; err != nil {
		return fmt.Errorf("failed to create stacks table: %w", err)
	}

	// Now add the foreign key constraints
	constraintSQL := []string{
		`ALTER TABLE assets ADD CONSTRAINT fk_assets_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`,
		`ALTER TABLE assets ADD CONSTRAINT fk_assets_library FOREIGN KEY (library_id) REFERENCES libraries(id)`,
		`ALTER TABLE assets ADD CONSTRAINT fk_assets_live_photo_video FOREIGN KEY (live_photo_video_id) REFERENCES assets(id)`,
		`ALTER TABLE assets ADD CONSTRAINT fk_assets_stack FOREIGN KEY (stack_id) REFERENCES stacks(id)`,
		`ALTER TABLE stacks ADD CONSTRAINT fk_stacks_primary_asset FOREIGN KEY (primary_asset_id) REFERENCES assets(id)`,
	}

	for _, sql := range constraintSQL {
		if err := db.Exec(sql).Error; err != nil {
			// Log the error but don't fail - some constraints might already exist or fail due to missing data
			logrus.WithError(err).Debugf("Failed to add constraint: %s", sql)
		}
	}

	// Finally, run AutoMigrate to catch any remaining schema differences
	if err := db.AutoMigrate(&models.Asset{}, &models.Stack{}); err != nil {
		return fmt.Errorf("failed to complete asset/stack migration: %w", err)
	}

	return nil
}
