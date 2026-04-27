package users

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSupabaseClient(t *testing.T) *SupabaseAdminClient {
	t.Helper()
	baseURL := os.Getenv("TEST_SUPABASE_URL")
	key := os.Getenv("TEST_SUPABASE_SERVICE_ROLE_KEY")
	if baseURL == "" || key == "" {
		t.Skip("set TEST_SUPABASE_URL and TEST_SUPABASE_SERVICE_ROLE_KEY")
	}
	return NewSupabaseAdminClient(baseURL, key)
}

func freshEmail() string {
	return fmt.Sprintf("supabase-admin-test-%s@example.com", uuid.New())
}

func TestSupabaseAdmin_EnsureUserCreatesThenReuses(t *testing.T) {
	c := newSupabaseClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	email := freshEmail()
	id1, err := c.EnsureUser(ctx, email, "P@ssw0rd-123!")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id1)
	t.Cleanup(func() {
		_ = c.DeleteUser(context.Background(), id1)
	})

	// Reuse: same email, empty password — should return same id (no create).
	id2, err := c.EnsureUser(ctx, email, "")
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "second EnsureUser should reuse existing id")
}

func TestSupabaseAdmin_DeleteUserIdempotent(t *testing.T) {
	c := newSupabaseClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	email := freshEmail()
	id, err := c.EnsureUser(ctx, email, "P@ssw0rd-123!")
	require.NoError(t, err)

	require.NoError(t, c.DeleteUser(ctx, id))
	// Second delete should not error (idempotent).
	require.NoError(t, c.DeleteUser(ctx, id))
}

func TestSupabaseAdmin_EnsureUserMissingPasswordOnNewErrors(t *testing.T) {
	c := newSupabaseClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	email := freshEmail()
	_, err := c.EnsureUser(ctx, email, "")
	require.Error(t, err, "creating a brand-new user without a password should fail")
}
