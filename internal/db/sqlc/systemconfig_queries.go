package sqlc

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// SystemConfig represents a system configuration entry
type SystemConfig struct {
	Key   string      `db:"key"`
	Value pgtype.Text `db:"value"`
}

// GetAllSystemConfig retrieves all system configuration entries
func (q *Queries) GetAllSystemConfig(ctx context.Context) ([]SystemConfig, error) {
	query := `SELECT key, value FROM system_config ORDER BY key`

	rows, err := q.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []SystemConfig
	for rows.Next() {
		var config SystemConfig
		if err := rows.Scan(&config.Key, &config.Value); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// GetSystemConfig retrieves a specific system configuration value
func (q *Queries) GetSystemConfig(ctx context.Context, key string) (pgtype.Text, error) {
	query := `SELECT value FROM system_config WHERE key = $1`

	var value pgtype.Text
	err := q.db.QueryRow(ctx, query, key).Scan(&value)
	return value, err
}

// UpsertSystemConfigParams contains parameters for upserting system config
type UpsertSystemConfigParams struct {
	Key   string
	Value string
}

// UpsertSystemConfig inserts or updates a system configuration entry
func (q *Queries) UpsertSystemConfig(ctx context.Context, arg UpsertSystemConfigParams) error {
	query := `
	INSERT INTO system_config (key, value, "createdAt", "updatedAt")
	VALUES ($1, $2, NOW(), NOW())
	ON CONFLICT (key) DO UPDATE
	SET value = $2, "updatedAt" = NOW()`

	_, err := q.db.Exec(ctx, query, arg.Key, arg.Value)
	return err
}

// DeleteSystemConfig deletes a system configuration entry
func (q *Queries) DeleteSystemConfig(ctx context.Context, key string) error {
	query := `DELETE FROM system_config WHERE key = $1`
	_, err := q.db.Exec(ctx, query, key)
	return err
}
