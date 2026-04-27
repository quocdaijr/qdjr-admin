package posts

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

// testFixture seeds a small published-post graph (post + category + tag +
// thumbnail) and returns helpers to build extra posts. All inserted rows are
// scoped to a unique auth.users row so t.Cleanup can wipe by author + media id
// without disturbing other tests.
type testFixture struct {
	t        *testing.T
	pool     *pgxpool.Pool
	ctx      context.Context
	authorID uuid.UUID
	mediaID  uuid.UUID
	catID    uuid.UUID
	catSlug  string
	tagID    uuid.UUID
	tagSlug  string
}

func newFixture(t *testing.T) *testFixture {
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

	authorID := uuid.New()
	_, err = pool.Exec(ctx,
		`insert into auth.users (id, email) values ($1, $2)`,
		authorID, fmt.Sprintf("posts-test-%s@example.com", authorID),
	)
	require.NoError(t, err)

	mediaID := uuid.New()
	_, err = pool.Exec(ctx,
		`insert into public.media (id, filename, storage_path, mime_type, size, alt_text, uploaded_by)
         values ($1, 'thumb.jpg', $2, 'image/jpeg', 1234, 'a thumbnail', $3)`,
		mediaID, fmt.Sprintf("media/%s.jpg", mediaID), authorID,
	)
	require.NoError(t, err)

	// Use a unique slug per run to dodge the unique constraint.
	catSlug := "cat-" + uuid.New().String()[:8]
	catID := uuid.New()
	_, err = pool.Exec(ctx,
		`insert into public.categories (id, slug, name) values ($1, $2, 'Test Cat')`,
		catID, catSlug,
	)
	require.NoError(t, err)

	tagSlug := "tag-" + uuid.New().String()[:8]
	tagID := uuid.New()
	_, err = pool.Exec(ctx,
		`insert into public.tags (id, slug, name) values ($1, $2, 'Test Tag')`,
		tagID, tagSlug,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		bg := context.Background()
		// Wipe in dependency order. Posts cascade their pivots; categories
		// and tags cascade pivots from their side. Media is referenced by
		// posts.thumbnail_id ON DELETE SET NULL, so deleting posts first is
		// fine. The author cascades back into auth.users children.
		_, _ = pool.Exec(bg, `delete from public.posts where created_by = $1`, authorID)
		_, _ = pool.Exec(bg, `delete from public.media where id = $1`, mediaID)
		_, _ = pool.Exec(bg, `delete from public.categories where id = $1`, catID)
		_, _ = pool.Exec(bg, `delete from public.tags where id = $1`, tagID)
		_, _ = pool.Exec(bg, `delete from auth.users where id = $1`, authorID)
	})

	return &testFixture{
		t: t, pool: pool, ctx: ctx,
		authorID: authorID, mediaID: mediaID,
		catID: catID, catSlug: catSlug,
		tagID: tagID, tagSlug: tagSlug,
	}
}

// insertPost creates a post with optional category/tag/thumbnail links.
func (f *testFixture) insertPost(slug, title, status string, publishedAt *time.Time, withCat, withTag, withThumb bool) uuid.UUID {
	f.t.Helper()
	id := uuid.New()

	var thumb any
	if withThumb {
		thumb = f.mediaID
	}

	_, err := f.pool.Exec(f.ctx, `
        insert into public.posts (id, slug, title, content, status, thumbnail_id, published_at, created_by)
        values ($1, $2, $3, 'body', $4::public.post_status, $5, $6, $7)
    `, id, slug, title, status, thumb, publishedAt, f.authorID)
	require.NoError(f.t, err)

	if withCat {
		_, err = f.pool.Exec(f.ctx,
			`insert into public.post_categories (post_id, category_id) values ($1, $2)`,
			id, f.catID)
		require.NoError(f.t, err)
	}
	if withTag {
		_, err = f.pool.Exec(f.ctx,
			`insert into public.post_tags (post_id, tag_id) values ($1, $2)`,
			id, f.tagID)
		require.NoError(f.t, err)
	}
	return id
}

func TestRepository_ListPublished(t *testing.T) {
	f := newFixture(t)

	now := time.Now().UTC()
	earlier := now.Add(-1 * time.Hour)

	// Three posts: published+links, published no-links, draft (excluded).
	slugA := "post-a-" + uuid.New().String()[:8]
	slugB := "post-b-" + uuid.New().String()[:8]
	slugDraft := "post-draft-" + uuid.New().String()[:8]

	f.insertPost(slugA, "Alpha About Foo", "published", &now, true, true, true)
	f.insertPost(slugB, "Bravo", "published", &earlier, false, false, false)
	f.insertPost(slugDraft, "Delta Draft", "draft", nil, true, true, true)

	repo := NewRepository(f.pool, "https://example.test/storage/")

	t.Run("returns only published, newest first", func(t *testing.T) {
		out, total, err := repo.ListPublished(f.ctx, ListFilter{
			Page: 1, PerPage: 50, CategorySlug: f.catSlug,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, total, "only one published post matches the category filter")
		require.Len(t, out, 1)
		assert.Equal(t, slugA, out[0].Slug)
		require.NotNil(t, out[0].Thumbnail)
		assert.Equal(t, "https://example.test/storage/media/"+f.mediaID.String()+".jpg", out[0].Thumbnail.URL)
		assert.Equal(t, "a thumbnail", out[0].Thumbnail.Alt)
		require.NotNil(t, out[0].Category)
		assert.Equal(t, f.catSlug, out[0].Category.Slug)
		require.Len(t, out[0].Tags, 1)
		assert.Equal(t, f.tagSlug, out[0].Tags[0].Slug)
		assert.Nil(t, out[0].Location)
	})

	t.Run("no filters → all published, ordered by published_at desc", func(t *testing.T) {
		out, total, err := repo.ListPublished(f.ctx, ListFilter{Page: 1, PerPage: 50})
		require.NoError(t, err)
		// Total may be >=2 if other tests left rows, but our two slugs must be
		// present in order (A before B).
		assert.GreaterOrEqual(t, total, 2)
		var idxA, idxB = -1, -1
		for i, p := range out {
			switch p.Slug {
			case slugA:
				idxA = i
			case slugB:
				idxB = i
			case slugDraft:
				t.Fatalf("draft post leaked into list response")
			}
		}
		require.NotEqual(t, -1, idxA)
		require.NotEqual(t, -1, idxB)
		assert.Less(t, idxA, idxB, "newer post (slugA) should come first")
	})

	t.Run("q filter matches title (ILIKE)", func(t *testing.T) {
		out, total, err := repo.ListPublished(f.ctx, ListFilter{Page: 1, PerPage: 50, Q: "alpha"})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, out, 1)
		assert.Equal(t, slugA, out[0].Slug)
	})

	t.Run("tag filter", func(t *testing.T) {
		out, total, err := repo.ListPublished(f.ctx, ListFilter{Page: 1, PerPage: 50, TagSlug: f.tagSlug})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, out, 1)
		assert.Equal(t, slugA, out[0].Slug)
	})
}

func TestRepository_GetPublishedBySlug(t *testing.T) {
	f := newFixture(t)
	now := time.Now().UTC()

	slug := "single-" + uuid.New().String()[:8]
	f.insertPost(slug, "Single", "published", &now, true, true, true)

	draftSlug := "single-draft-" + uuid.New().String()[:8]
	f.insertPost(draftSlug, "Draft", "draft", nil, false, false, false)

	repo := NewRepository(f.pool, "")

	t.Run("returns published post with full graph", func(t *testing.T) {
		got, err := repo.GetPublishedBySlug(f.ctx, slug)
		require.NoError(t, err)
		assert.Equal(t, slug, got.Slug)
		require.NotNil(t, got.Thumbnail)
		// urlPrefix empty → raw storage_path
		assert.Equal(t, fmt.Sprintf("media/%s.jpg", f.mediaID), got.Thumbnail.URL)
		require.NotNil(t, got.Category)
		assert.Len(t, got.Tags, 1)
		require.NotNil(t, got.PublishedAt)
	})

	t.Run("draft slug returns ErrNotFound", func(t *testing.T) {
		_, err := repo.GetPublishedBySlug(f.ctx, draftSlug)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("missing slug returns ErrNotFound", func(t *testing.T) {
		_, err := repo.GetPublishedBySlug(f.ctx, "definitely-does-not-exist-"+uuid.New().String())
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
