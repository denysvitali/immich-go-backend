package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
)

type Conn struct {
	pool *pgxpool.Pool
	*sqlc.Queries
}

func (c *Conn) Close() {
	c.pool.Close()
}

func New(ctx context.Context, databaseURL string) (*Conn, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		return nil, err
	}

	return &Conn{pool: pool, Queries: sqlc.New(conn)}, nil
}
