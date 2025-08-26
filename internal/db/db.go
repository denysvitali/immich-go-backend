package db

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
)

type Conn struct {
	pool *pgxpool.Pool
	*sqlc.Queries
}

func (c *Conn) Close() {
	c.pool.Close()
}

// DB returns a standard database/sql DB for migrations
func (c *Conn) DB() *sql.DB {
	return stdlib.OpenDBFromPool(c.pool)
}

func New(ctx context.Context, databaseURL string) (*Conn, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &Conn{pool: pool, Queries: sqlc.New(pool)}, nil
}
