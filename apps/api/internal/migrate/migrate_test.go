package migrate

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_RefusesDestructiveCommands(t *testing.T) {
	for _, cmd := range []string{"down", "down-to", "reset", "redo", "create", "fix", ""} {
		err := Run(context.Background(), "postgres://unused", cmd)
		require.Error(t, err, cmd)
		assert.Contains(t, err.Error(), "unsupported command", cmd)
	}
}

func TestUp_IsIdempotent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	ctx := context.Background()
	require.NoError(t, Up(ctx, dsn))
	require.NoError(t, Up(ctx, dsn)) // second run: nothing to apply, no error
	require.NoError(t, Run(ctx, dsn, "status"))
}
