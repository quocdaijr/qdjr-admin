// apps/api/cmd/migrate/main.go
//
// Manual migration runner:
//
//	go run ./cmd/migrate            # up
//	go run ./cmd/migrate status
//	go run ./cmd/migrate up-to 7
//
// Reads DATABASE_URL like the API. In production the API applies migrations
// itself at startup (RUN_MIGRATIONS=true), so this is for local databases and
// emergencies only.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/migrate"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"up"}
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "migrate: DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := migrate.Run(ctx, dsn, args[0], args[1:]...); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}
