// Package migrations embeds the schema migrations so the API binary (and
// cmd/migrate) carry them. goose applies them in numeric order; see
// internal/migrate.
package migrations

import "embed"

// FS holds every migration file, applied in numeric order by internal/migrate.
//
//go:embed *.sql
var FS embed.FS
