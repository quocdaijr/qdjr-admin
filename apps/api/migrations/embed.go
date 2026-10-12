// Package migrations embeds the schema migrations so the API binary (and
// cmd/migrate) carry them. goose applies them in numeric order; see
// internal/migrate.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
