package contact

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedMessage inserts a contact message and returns its id. Cleans up via t.Cleanup.
func seedMessage(t *testing.T, repo *Repository, in CreateInput) uuid.UUID {
	t.Helper()
	out, err := repo.Create(context.Background(), in)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.pool.Exec(context.Background(),
			`delete from public.contact_messages where id = $1`, out.ID)
	})
	return out.ID
}

func TestAdminRepository_List_FilterAndPage(t *testing.T) {
	pool, ctx := newPool(t)
	pub := NewRepository(pool)
	admin := NewAdminRepository(pool)

	id1 := seedMessage(t, pub, CreateInput{Name: "A", Email: "a@example.com", Body: "1"})
	id2 := seedMessage(t, pub, CreateInput{Name: "B", Email: "b@example.com", Body: "2"})

	// Mark id2 as 'spam' so we can verify status filter.
	_, err := pool.Exec(ctx,
		`update public.contact_messages set status = 'spam' where id = $1`, id2)
	require.NoError(t, err)

	// No filter → both visible (at minimum).
	got, total, err := admin.List(ctx, AdminListFilter{Page: 1, PerPage: 100})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 2)
	require.NotEmpty(t, got)

	// Status filter.
	gotSpam, _, err := admin.List(ctx, AdminListFilter{Page: 1, PerPage: 100, Status: "spam"})
	require.NoError(t, err)
	foundSpam := false
	for _, m := range gotSpam {
		assert.Equal(t, "spam", m.Status)
		if m.ID == id2 {
			foundSpam = true
		}
	}
	assert.True(t, foundSpam)

	// Status="new" filter must include id1 and exclude id2.
	gotNew, _, err := admin.List(ctx, AdminListFilter{Page: 1, PerPage: 100, Status: "new"})
	require.NoError(t, err)
	hasID1, hasID2 := false, false
	for _, m := range gotNew {
		if m.ID == id1 {
			hasID1 = true
		}
		if m.ID == id2 {
			hasID2 = true
		}
	}
	assert.True(t, hasID1, "id1 should appear under status=new")
	assert.False(t, hasID2, "id2 should be filtered out")
}

func TestAdminRepository_UpdateStatus(t *testing.T) {
	pool, ctx := newPool(t)
	pub := NewRepository(pool)
	admin := NewAdminRepository(pool)

	id := seedMessage(t, pub, CreateInput{Name: "A", Email: "a@example.com", Body: "1"})

	got, err := admin.UpdateStatus(ctx, id, "replied")
	require.NoError(t, err)
	assert.Equal(t, "replied", got.Status)

	var st string
	err = pool.QueryRow(ctx,
		`select status::text from public.contact_messages where id = $1`, id).Scan(&st)
	require.NoError(t, err)
	assert.Equal(t, "replied", st)
}

func TestAdminRepository_UpdateStatus_NotFound(t *testing.T) {
	pool, ctx := newPool(t)
	admin := NewAdminRepository(pool)
	_, err := admin.UpdateStatus(ctx, uuid.New(), "read")
	assert.ErrorIs(t, err, ErrNotFound)
}
