package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Skipped unless TEST_DATABASE_URL points at a running Postgres (use the local Supabase stack).
func TestNewPool_PingsDatabase(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	var one int
	err = pool.QueryRow(ctx, "select 1").Scan(&one)
	require.NoError(t, err)
	assert.Equal(t, 1, one)
}
