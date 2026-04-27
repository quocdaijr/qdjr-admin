package categories

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

type adminFixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	ctx  context.Context
	// Track created category IDs for cleanup.
	created []uuid.UUID
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

	f := &adminFixture{t: t, pool: pool, ctx: ctx}
	t.Cleanup(func() {
		bg := context.Background()
		for _, id := range f.created {
			_, _ = pool.Exec(bg, `delete from public.categories where id = $1`, id)
		}
	})
	return f
}

func (f *adminFixture) track(id uuid.UUID) { f.created = append(f.created, id) }

func TestAdminRepo_Categories_CreateAndGet(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	desc := "an example"
	in := CreateInput{
		Slug:        "cat-create-" + uuid.New().String()[:8],
		Name:        "Create Test",
		Description: &desc,
	}
	out, err := repo.Create(f.ctx, in)
	require.NoError(t, err)
	f.track(out.ID)
	assert.Equal(t, in.Slug, out.Slug)
	assert.Equal(t, in.Name, out.Name)
	require.NotNil(t, out.Description)
	assert.Equal(t, desc, *out.Description)

	got, err := repo.GetByID(f.ctx, out.ID)
	require.NoError(t, err)
	assert.Equal(t, out.ID, got.ID)
}

func TestAdminRepo_Categories_GetNotFound(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	_, err := repo.GetByID(f.ctx, uuid.New())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestAdminRepo_Categories_CreateSlugConflict(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	slug := "cat-dup-" + uuid.New().String()[:8]
	c1, err := repo.Create(f.ctx, CreateInput{Slug: slug, Name: "A"})
	require.NoError(t, err)
	f.track(c1.ID)

	_, err = repo.Create(f.ctx, CreateInput{Slug: slug, Name: "B"})
	assert.ErrorIs(t, err, ErrSlugConflict)
}

func TestAdminRepo_Categories_UpdatePartial(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	c, err := repo.Create(f.ctx, CreateInput{
		Slug: "cat-upd-" + uuid.New().String()[:8],
		Name: "Original",
	})
	require.NoError(t, err)
	f.track(c.ID)

	newName := "Updated"
	got, err := repo.Update(f.ctx, c.ID, UpdateInput{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Name)
	assert.Equal(t, c.Slug, got.Slug, "slug untouched")
}

func TestAdminRepo_Categories_UpdateSlugConflict(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	a, err := repo.Create(f.ctx, CreateInput{
		Slug: "cat-a-" + uuid.New().String()[:8], Name: "A",
	})
	require.NoError(t, err)
	f.track(a.ID)

	b, err := repo.Create(f.ctx, CreateInput{
		Slug: "cat-b-" + uuid.New().String()[:8], Name: "B",
	})
	require.NoError(t, err)
	f.track(b.ID)

	collide := a.Slug
	_, err = repo.Update(f.ctx, b.ID, UpdateInput{Slug: &collide})
	assert.ErrorIs(t, err, ErrSlugConflict)
}

func TestAdminRepo_Categories_UpdateNotFound(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	name := "Ghost"
	_, err := repo.Update(f.ctx, uuid.New(), UpdateInput{Name: &name})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestAdminRepo_Categories_Delete(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	c, err := repo.Create(f.ctx, CreateInput{
		Slug: "cat-del-" + uuid.New().String()[:8], Name: "Doomed",
	})
	require.NoError(t, err)
	// Don't track: we delete it here.

	require.NoError(t, repo.Delete(f.ctx, c.ID))
	_, err = repo.GetByID(f.ctx, c.ID)
	assert.ErrorIs(t, err, ErrNotFound)

	assert.ErrorIs(t, repo.Delete(f.ctx, uuid.New()), ErrNotFound)
}

func TestAdminRepo_Categories_DeleteBlockedByReferences(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	cat, err := repo.Create(f.ctx, CreateInput{
		Slug: "cat-ref-" + uuid.New().String()[:8], Name: "Referenced",
	})
	require.NoError(t, err)
	f.track(cat.ID)

	// Create a post referencing this category.
	authorID := uuid.New()
	_, err = f.pool.Exec(f.ctx,
		`insert into auth.users (id, email) values ($1, $2)`,
		authorID, fmt.Sprintf("cat-test-%s@example.com", authorID))
	require.NoError(t, err)

	postID := uuid.New()
	_, err = f.pool.Exec(f.ctx, `
        insert into public.posts (id, slug, title, content, status, created_by)
        values ($1, $2, 'Title', 'body', 'draft'::public.post_status, $3)`,
		postID, "cat-ref-post-"+uuid.New().String()[:8], authorID)
	require.NoError(t, err)
	_, err = f.pool.Exec(f.ctx,
		`insert into public.post_categories (post_id, category_id) values ($1, $2)`,
		postID, cat.ID)
	require.NoError(t, err)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = f.pool.Exec(bg, `delete from public.posts where id = $1`, postID)
		_, _ = f.pool.Exec(bg, `delete from auth.users where id = $1`, authorID)
	})

	err = repo.Delete(f.ctx, cat.ID)
	assert.ErrorIs(t, err, ErrInUse)
	assert.Contains(t, err.Error(), "referenced by 1 posts")
}

func TestAdminRepo_Categories_List(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	uniq := uuid.New().String()[:8]
	for i, n := range []string{"Apple", "Banana", "Cherry"} {
		c, err := repo.Create(f.ctx, CreateInput{
			Slug: fmt.Sprintf("cat-l%d-%s", i, uniq),
			Name: n + " " + uniq,
		})
		require.NoError(t, err)
		f.track(c.ID)
	}

	out, total, err := repo.List(f.ctx, AdminListFilter{Page: 1, PerPage: 50, Q: uniq})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	require.Len(t, out, 3)

	out2, total2, err := repo.List(f.ctx, AdminListFilter{Page: 1, PerPage: 50, Q: "Banana " + uniq})
	require.NoError(t, err)
	assert.Equal(t, 1, total2)
	require.Len(t, out2, 1)
	assert.Contains(t, out2[0].Name, "Banana")
}
