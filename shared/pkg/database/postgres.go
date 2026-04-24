// Package database provides a shared PostgreSQL connection pool factory
// backed by pgx/v5. Call Connect at service startup and defer pool.Close().
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds the options passed to Connect.
type Config struct {
	DSN         string
	MaxConns    int32
	MinConns    int32
	MaxConnLife time.Duration
	MaxConnIdle time.Duration
}

// DefaultConfig returns sensible defaults for a single-service pool.
func DefaultConfig(dsn string) Config {
	return Config{
		DSN:         dsn,
		MaxConns:    25,
		MinConns:    2,
		MaxConnLife: 30 * time.Minute,
		MaxConnIdle: 5 * time.Minute,
	}
}

// Connect creates a pgxpool.Pool, pings the database, and returns it.
// The caller is responsible for calling pool.Close() on shutdown.
func Connect(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("database: parse config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLife
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdle

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	return pool, nil
}
