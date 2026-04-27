package rbac

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPGStore_RoleForUser(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// Insert a fake auth.users row; in real life Supabase Auth owns this.
	uid := uuid.New()
	_, err = pool.Exec(ctx,
		`insert into auth.users (id, email) values ($1, $2)`,
		uid, "store-test@example.com")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `delete from auth.users where id = $1`, uid) })

	_, err = pool.Exec(ctx,
		`insert into public.user_roles (user_id, role_id)
         select $1, id from public.roles where name = 'editor'`,
		uid)
	require.NoError(t, err)

	s := NewPGStore(pool)
	role, err := s.RoleForUser(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, "editor", role)

	_, err = s.RoleForUser(ctx, uuid.New())
	assert.ErrorIs(t, err, ErrNoRole)
}
