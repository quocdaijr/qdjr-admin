// Package db opens the pgx connection pool.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// First-connect retry budget: 1+2+4+8 s of waits covers a Neon cold start.
const (
	connectAttempts = 5
	connectBackoff  = time.Second
)

// NewPool opens a pgx connection pool and verifies connectivity with a Ping
// (retried, see Retry). Sized for one small web instance; Neon's pooled
// endpoint (PgBouncer) multiplexes these onto the database.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := Retry(ctx, connectAttempts, connectBackoff, pool.Ping); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
