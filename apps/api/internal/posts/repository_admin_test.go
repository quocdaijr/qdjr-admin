package posts

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminRepo_CreateAndGet(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool)

	excerpt := "summary"
	in := CreateInput{
		Title:       "Hello Admin",
		Slug:        "hello-admin-" + uuid.New().String()[:8],
		Excerpt:     &excerpt,
		Content:     "body",
		Status:      "draft",
		CategoryIDs: []uuid.UUID{f.catID},
		TagIDs:      []uuid.UUID{f.tagID},
		CreatedBy:   f.authorID,
	}
	out, err := repo.Create(f.ctx, in)
	require.NoError(t, err)
	assert.Equal(t, in.Slug, out.Slug)
	assert.Equal(t, "draft", out.Status)
	require.NotNil(t, out.CreatedBy)
	assert.Equal(t, f.authorID, *out.CreatedBy)
	require.Len(t, out.Categories, 1)
	require.Len(t, out.Tags, 1)
	assert.Equal(t, f.catSlug, out.Categories[0].Slug)
	assert.Equal(t, f.tagSlug, out.Tags[0].Slug)
	assert.Nil(t, out.PublishedAt)

	got, err := repo.GetByID(f.ctx, out.ID)
	require.NoError(t, err)
	assert.Equal(t, out.ID, got.ID)
}

func TestAdminRepo_CreatePublishedSetsPublishedAt(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool)

	in := CreateInput{
		Title:     "Published",
		Slug:      "published-" + uuid.New().String()[:8],
		Content:   "body",
		Status:    "published",
		CreatedBy: f.authorID,
	}
	out, err := repo.Create(f.ctx, in)
	require.NoError(t, err)
	assert.Equal(t, "published", out.Status)
	require.NotNil(t, out.PublishedAt)
}

func TestAdminRepo_CreateSlugConflict(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool)

	slug := "dup-" + uuid.New().String()[:8]
	_, err := repo.Create(f.ctx, CreateInput{
		Title: "A", Slug: slug, Content: "b", Status: "draft", CreatedBy: f.authorID,
	})
	require.NoError(t, err)

	_, err = repo.Create(f.ctx, CreateInput{
		Title: "B", Slug: slug, Content: "b", Status: "draft", CreatedBy: f.authorID,
	})
	assert.ErrorIs(t, err, ErrSlugConflict)
}

func TestAdminRepo_UpdatePartialAndPivots(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool)

	created, err := repo.Create(f.ctx, CreateInput{
		Title:       "Initial",
		Slug:        "initial-" + uuid.New().String()[:8],
		Content:     "body",
		Status:      "draft",
		CategoryIDs: []uuid.UUID{f.catID},
		TagIDs:      []uuid.UUID{f.tagID},
		CreatedBy:   f.authorID,
	})
	require.NoError(t, err)

	newTitle := "Updated"
	emptyTags := []uuid.UUID{}
	updated, err := repo.Update(f.ctx, created.ID, UpdateInput{
		Title:     &newTitle,
		TagIDs:    &emptyTags, // explicit empty → wipe tag pivots
		UpdatedBy: f.authorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Title)
	assert.Len(t, updated.Tags, 0, "tags should be wiped")
	assert.Len(t, updated.Categories, 1, "categories left untouched (CategoryIDs nil)")
}

func TestAdminRepo_UpdatePublishTransition(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool)

	created, err := repo.Create(f.ctx, CreateInput{
		Title:     "Going Live",
		Slug:      "live-" + uuid.New().String()[:8],
		Content:   "body",
		Status:    "draft",
		CreatedBy: f.authorID,
	})
	require.NoError(t, err)
	assert.Nil(t, created.PublishedAt)

	pub := "published"
	updated, err := repo.Update(f.ctx, created.ID, UpdateInput{
		Status:    &pub,
		UpdatedBy: f.authorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "published", updated.Status)
	require.NotNil(t, updated.PublishedAt, "published_at must be set on draft→published transition")
	firstPublished := *updated.PublishedAt

	// Re-publish (no transition since already published) should preserve the
	// timestamp.
	updated2, err := repo.Update(f.ctx, created.ID, UpdateInput{
		Status:    &pub,
		UpdatedBy: f.authorID,
	})
	require.NoError(t, err)
	require.NotNil(t, updated2.PublishedAt)
	assert.Equal(t, firstPublished.Unix(), updated2.PublishedAt.Unix())
}

func TestAdminRepo_Delete(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool)

	created, err := repo.Create(f.ctx, CreateInput{
		Title: "Doomed", Slug: "doomed-" + uuid.New().String()[:8],
		Content: "x", Status: "draft", CreatedBy: f.authorID,
	})
	require.NoError(t, err)

	require.NoError(t, repo.Delete(f.ctx, created.ID))
	_, err = repo.GetByID(f.ctx, created.ID)
	assert.ErrorIs(t, err, ErrNotFound)

	// Idempotency / missing → ErrNotFound.
	assert.ErrorIs(t, repo.Delete(f.ctx, uuid.New()), ErrNotFound)
}

func TestAdminRepo_SetPublishedIdempotent(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool)

	created, err := repo.Create(f.ctx, CreateInput{
		Title: "Toggle", Slug: "toggle-" + uuid.New().String()[:8],
		Content: "x", Status: "draft", CreatedBy: f.authorID,
	})
	require.NoError(t, err)

	pub, err := repo.SetPublished(f.ctx, created.ID, true, f.authorID)
	require.NoError(t, err)
	assert.Equal(t, "published", pub.Status)
	require.NotNil(t, pub.PublishedAt)
	first := *pub.PublishedAt

	// Re-publish: keeps original timestamp.
	pub2, err := repo.SetPublished(f.ctx, created.ID, true, f.authorID)
	require.NoError(t, err)
	require.NotNil(t, pub2.PublishedAt)
	assert.Equal(t, first.Unix(), pub2.PublishedAt.Unix())

	// Unpublish: clears.
	unp, err := repo.SetPublished(f.ctx, created.ID, false, f.authorID)
	require.NoError(t, err)
	assert.Equal(t, "draft", unp.Status)
	assert.Nil(t, unp.PublishedAt)
}

func TestAdminRepo_ListFilters(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool)

	otherAuthor := uuid.New()
	_, err := f.pool.Exec(f.ctx,
		`insert into auth.users (id, email) values ($1, $2)`,
		otherAuthor, "other-"+otherAuthor.String()+"@example.com")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.pool.Exec(f.ctx, `delete from public.posts where created_by = $1`, otherAuthor)
		_, _ = f.pool.Exec(f.ctx, `delete from auth.users where id = $1`, otherAuthor)
	})

	// Author A: 1 draft, 1 published.
	_, err = repo.Create(f.ctx, CreateInput{
		Title: "A draft", Slug: "a-draft-" + uuid.New().String()[:8],
		Content: "x", Status: "draft", CreatedBy: f.authorID,
	})
	require.NoError(t, err)
	_, err = repo.Create(f.ctx, CreateInput{
		Title: "A published", Slug: "a-pub-" + uuid.New().String()[:8],
		Content: "x", Status: "published", CreatedBy: f.authorID,
	})
	require.NoError(t, err)
	// Author B: 1 published.
	_, err = repo.Create(f.ctx, CreateInput{
		Title: "B published", Slug: "b-pub-" + uuid.New().String()[:8],
		Content: "x", Status: "published", CreatedBy: otherAuthor,
	})
	require.NoError(t, err)

	t.Run("scoped to author A", func(t *testing.T) {
		uid := f.authorID
		out, total, err := repo.List(f.ctx, AdminListFilter{
			Page: 1, PerPage: 50, CreatedBy: &uid,
		})
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, out, 2)
		for _, p := range out {
			require.NotNil(t, p.CreatedBy)
			assert.Equal(t, f.authorID, *p.CreatedBy)
		}
	})

	t.Run("status=draft scoped", func(t *testing.T) {
		uid := f.authorID
		out, total, err := repo.List(f.ctx, AdminListFilter{
			Page: 1, PerPage: 50, Status: "draft", CreatedBy: &uid,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, out, 1)
		assert.Equal(t, "draft", out[0].Status)
	})

	t.Run("q filter ILIKE", func(t *testing.T) {
		uid := f.authorID
		out, _, err := repo.List(f.ctx, AdminListFilter{
			Page: 1, PerPage: 50, Q: "published", CreatedBy: &uid,
		})
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Contains(t, out[0].Title, "published")
	})
}
