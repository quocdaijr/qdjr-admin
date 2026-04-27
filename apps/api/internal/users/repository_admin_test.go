package users

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminFixture connects to TEST_DATABASE_URL and provides per-test cleanup
// helpers for inserted auth.users / user_roles rows.
type adminFixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	ctx  context.Context
}

func newAdminFixture(t *testing.T) *adminFixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return &adminFixture{t: t, pool: pool, ctx: ctx}
}

// insertUser inserts a fresh row in auth.users with random uuid + email and
// registers cleanup that removes the user_roles + auth.users rows.
func (f *adminFixture) insertUser() uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	email := fmt.Sprintf("users-admin-test-%s@example.com", id)
	_, err := f.pool.Exec(f.ctx,
		`insert into auth.users (id, email, created_at) values ($1, $2, now())`, id, email)
	require.NoError(f.t, err)
	f.t.Cleanup(func() {
		bg := context.Background()
		_, _ = f.pool.Exec(bg, `delete from public.user_roles where user_id = $1`, id)
		_, _ = f.pool.Exec(bg, `delete from auth.users where id = $1`, id)
	})
	return id
}

func TestAdminRepo_ListIncludesUserWithoutRole(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	uid := f.insertUser()
	users, _, err := repo.List(f.ctx, ListFilter{Page: 1, PerPage: 100})
	require.NoError(t, err)

	var found *User
	for i := range users {
		if users[i].ID == uid {
			found = &users[i]
			break
		}
	}
	require.NotNil(t, found, "freshly inserted user should appear in list")
	assert.Nil(t, found.Role, "no role assigned yet")
	assert.Nil(t, found.AssignedAt)
}

func TestAdminRepo_SetRoleUpsertsAndGet(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)
	uid := f.insertUser()

	require.NoError(t, repo.SetRole(f.ctx, uid, "editor"))
	got, err := repo.Get(f.ctx, uid)
	require.NoError(t, err)
	require.NotNil(t, got.Role)
	assert.Equal(t, "editor", *got.Role)

	require.NoError(t, repo.SetRole(f.ctx, uid, "author"))
	got, err = repo.Get(f.ctx, uid)
	require.NoError(t, err)
	require.NotNil(t, got.Role)
	assert.Equal(t, "author", *got.Role)
}

func TestAdminRepo_SetRoleUnknownRoleErrors(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)
	uid := f.insertUser()

	err := repo.SetRole(f.ctx, uid, "nonsense")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonsense")
}

func TestAdminRepo_DeleteRoleClearsAssignment(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)
	uid := f.insertUser()

	require.NoError(t, repo.SetRole(f.ctx, uid, "editor"))
	require.NoError(t, repo.DeleteRole(f.ctx, uid))

	got, err := repo.Get(f.ctx, uid)
	require.NoError(t, err)
	assert.Nil(t, got.Role)
	assert.Nil(t, got.AssignedAt)
}

func TestAdminRepo_IsLastSuperAdmin(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	// Capture current super_admin user ids so we can restore after.
	rows, err := f.pool.Query(f.ctx, `
        select ur.user_id
          from public.user_roles ur
          join public.roles r on r.id = ur.role_id
         where r.name = 'super_admin'`)
	require.NoError(t, err)
	preExisting := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		preExisting = append(preExisting, id)
	}
	rows.Close()

	// Temporarily delete pre-existing super_admin rows so the test is
	// deterministic regardless of seed state, then restore.
	for _, id := range preExisting {
		_, err = f.pool.Exec(f.ctx, `delete from public.user_roles where user_id = $1`, id)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		for _, id := range preExisting {
			_, _ = f.pool.Exec(bg, `
                insert into public.user_roles (user_id, role_id)
                  select $1, id from public.roles where name = 'super_admin'
                  on conflict (user_id) do update
                    set role_id = excluded.role_id, assigned_at = now()`, id)
		}
	})

	uid1 := f.insertUser()
	uid2 := f.insertUser()
	uidEditor := f.insertUser()
	uidNoRole := f.insertUser()

	// Case: target is the only super_admin → true.
	require.NoError(t, repo.SetRole(f.ctx, uid1, "super_admin"))
	last, err := repo.IsLastSuperAdmin(f.ctx, uid1)
	require.NoError(t, err)
	assert.True(t, last, "single super_admin should be the last one")

	// Case: target is super_admin but two exist → false.
	require.NoError(t, repo.SetRole(f.ctx, uid2, "super_admin"))
	last, err = repo.IsLastSuperAdmin(f.ctx, uid1)
	require.NoError(t, err)
	assert.False(t, last, "with 2 super_admins the target is not last")

	// Case: target is editor → false.
	require.NoError(t, repo.SetRole(f.ctx, uidEditor, "editor"))
	last, err = repo.IsLastSuperAdmin(f.ctx, uidEditor)
	require.NoError(t, err)
	assert.False(t, last)

	// Case: target has no role → false.
	last, err = repo.IsLastSuperAdmin(f.ctx, uidNoRole)
	require.NoError(t, err)
	assert.False(t, last)
}
