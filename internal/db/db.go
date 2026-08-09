package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
)

type Conn struct {
	pool *pgxpool.Pool
	*sqlc.Queries
}

type Options struct {
	MaxOpenConns    int
	MinIdleConns    int
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
	LogQueries      bool
}

func (c *Conn) Close() error {
	c.pool.Close()
	return nil
}

// DB returns a standard database/sql DB for migrations
func (c *Conn) DB() *sql.DB {
	return stdlib.OpenDBFromPool(c.pool)
}

func (c *Conn) Ping(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

func New(ctx context.Context, databaseURL string) (*Conn, error) {
	return NewWithOptions(ctx, databaseURL, Options{})
}

func NewWithOptions(ctx context.Context, databaseURL string, options Options) (*Conn, error) {
	poolConfig, err := configuredPool(databaseURL, options)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
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

func configuredPool(databaseURL string, options Options) (*pgxpool.Config, error) {
	if options.MaxOpenConns < 0 || options.MinIdleConns < 0 {
		return nil, fmt.Errorf("database connection limits must not be negative")
	}
	if options.MaxOpenConns > 0 && options.MinIdleConns > options.MaxOpenConns {
		return nil, fmt.Errorf("database minimum idle connections (%d) exceeds max open connections (%d)", options.MinIdleConns, options.MaxOpenConns)
	}

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	if options.MaxOpenConns > 0 {
		poolConfig.MaxConns = int32(options.MaxOpenConns)
	}
	if options.MinIdleConns > 0 {
		poolConfig.MinIdleConns = int32(options.MinIdleConns)
	}
	if options.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = options.MaxConnLifetime
	}
	if options.ConnectTimeout > 0 {
		poolConfig.ConnConfig.ConnectTimeout = options.ConnectTimeout
	}
	if options.LogQueries {
		poolConfig.ConnConfig.Tracer = dbQueryTracer{}
	}
	if options.QueryTimeout > 0 {
		previousAfterConnect := poolConfig.AfterConnect
		poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			if previousAfterConnect != nil {
				if err := previousAfterConnect(ctx, conn); err != nil {
					return err
				}
			}
			milliseconds := strconv.FormatInt(options.QueryTimeout.Milliseconds(), 10)
			_, err := conn.Exec(ctx, "SELECT set_config('statement_timeout', $1, false)", milliseconds)
			return err
		}
	}
	return poolConfig, nil
}

type queryTraceStartKey struct{}

type dbQueryTracer struct{}

func (dbQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	logrus.WithField("sql", data.SQL).Debug("Database query started")
	return context.WithValue(ctx, queryTraceStartKey{}, time.Now())
}

func (dbQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	entry := logrus.WithField("commandTag", data.CommandTag.String())
	if started, ok := ctx.Value(queryTraceStartKey{}).(time.Time); ok {
		entry = entry.WithField("durationMs", time.Since(started).Milliseconds())
	}
	if data.Err != nil {
		entry.WithError(data.Err).Warn("Database query failed")
		return
	}
	entry.Debug("Database query completed")
}
