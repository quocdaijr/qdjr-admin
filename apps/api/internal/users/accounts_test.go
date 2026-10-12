package users

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
)

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%s@Example.COM", prefix, uuid.NewString()[:8])
}

func TestPGAccounts_CreateFindDelete(t *testing.T) {
	f := newAdminFixture(t)
	acc := NewPGAccounts(f.pool)
	email := uniqueEmail("acc")

	id, err := acc.Create(f.ctx, "  "+email+" ", "a-long-password")
	require.NoError(t, err)
	t.Cleanup(func() { _ = acc.Delete(f.ctx, id) })

	cred, err := acc.FindByEmail(f.ctx, email) // lookup is case-insensitive
	require.NoError(t, err)
	assert.Equal(t, id, cred.ID)
	assert.Equal(t, normalizeEmail(email), cred.Email)
	assert.True(t, auth.CheckPassword(cred.PasswordHash, "a-long-password"))

	_, err = acc.Create(f.ctx, email, "another-long-password")
	assert.ErrorIs(t, err, ErrEmailTaken)

	require.NoError(t, acc.Delete(f.ctx, id))
	_, err = acc.FindByID(f.ctx, id)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, acc.Delete(f.ctx, id), "deleting twice is a no-op")
}

func TestPGAccounts_CreateRejectsWeakPasswordAndBadEmail(t *testing.T) {
	f := newAdminFixture(t)
	acc := NewPGAccounts(f.pool)
	_, err := acc.Create(f.ctx, uniqueEmail("weak"), "short")
	assert.ErrorIs(t, err, auth.ErrWeakPassword)
	_, err = acc.Create(f.ctx, "not-an-email", "a-long-password")
	assert.ErrorIs(t, err, ErrInvalidEmail)
}

func TestPGAccounts_SetPasswordAndTouch(t *testing.T) {
	f := newAdminFixture(t)
	acc := NewPGAccounts(f.pool)
	id, err := acc.Create(f.ctx, uniqueEmail("pw"), "first-password")
	require.NoError(t, err)
	t.Cleanup(func() { _ = acc.Delete(f.ctx, id) })

	hash, err := auth.HashPassword("second-password")
	require.NoError(t, err)
	require.NoError(t, acc.SetPassword(f.ctx, id, hash))
	require.NoError(t, acc.TouchSignIn(f.ctx, id))

	cred, err := acc.FindByID(f.ctx, id)
	require.NoError(t, err)
	assert.True(t, auth.CheckPassword(cred.PasswordHash, "second-password"))
	var touched bool
	require.NoError(t, f.pool.QueryRow(f.ctx, `select last_sign_in_at is not null from public.users where id=$1`, id).Scan(&touched))
	assert.True(t, touched)
}

func TestSeedAdmin_IsIdempotent(t *testing.T) {
	f := newAdminFixture(t)
	acc := NewPGAccounts(f.pool)
	repo := NewAdminRepository(f.pool)
	email := uniqueEmail("seed")

	created, err := SeedAdmin(f.ctx, acc, repo, email, "seed-password-1")
	require.NoError(t, err)
	assert.True(t, created)
	cred, err := acc.FindByEmail(f.ctx, email)
	require.NoError(t, err)
	t.Cleanup(func() { _ = acc.Delete(f.ctx, cred.ID) })

	u, err := repo.Get(f.ctx, cred.ID)
	require.NoError(t, err)
	require.NotNil(t, u.Role)
	assert.Equal(t, "super_admin", *u.Role)

	created, err = SeedAdmin(f.ctx, acc, repo, email, "a-different-password")
	require.NoError(t, err)
	assert.False(t, created, "second run leaves the existing account alone")
	cred2, err := acc.FindByEmail(f.ctx, email)
	require.NoError(t, err)
	assert.True(t, auth.CheckPassword(cred2.PasswordHash, "seed-password-1"), "password is not overwritten")

	created, err = SeedAdmin(f.ctx, acc, repo, "", "")
	require.NoError(t, err)
	assert.False(t, created, "empty config is a no-op")
}
