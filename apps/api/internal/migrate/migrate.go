// Package migrate applies the embedded SQL migrations (apps/api/migrations)
// with goose. cmd/api calls Up at startup when RUN_MIGRATIONS=true; cmd/migrate
// exposes a few commands for local use. Forward-only: nothing here can roll a
// database back.
package migrate

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/quocdaijr/qdjr-admin/apps/api/migrations"
)

// Commands is the allowlist cmd/migrate accepts. down/reset/redo are left out
// on purpose: rollback in this repo is a new migration, never a reversal.
var Commands = map[string]bool{"up": true, "up-by-one": true, "up-to": true, "status": true, "version": true}

// Run executes one goose command against dsn using the embedded migrations.
func Run(ctx context.Context, dsn, command string, args ...string) error {
	if !Commands[command] {
		return fmt.Errorf("unsupported command %q (use up, up-by-one, up-to N, status, version)", command)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.RunContext(ctx, command, db, ".", args...)
}

// Up applies every pending migration.
func Up(ctx context.Context, dsn string) error {
	return Run(ctx, dsn, "up")
}
