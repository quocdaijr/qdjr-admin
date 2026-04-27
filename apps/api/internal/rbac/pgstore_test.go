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
	// Use a per-run unique email so reruns don't collide on users_email_partial_key.
	uid := uuid.New()
	email := "store-test+" + uid.String() + "@example.com"
	_, err = pool.Exec(ctx,
		`insert into auth.users (id, email, created_at) values ($1, $2, now())`,
		uid, email)
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
