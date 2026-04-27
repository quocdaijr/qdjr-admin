package tags

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

// testFixture seeds two tags with unique slugs/ids so the test can run
// alongside any other rows already in the table.
type testFixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	ctx  context.Context

	idA, idB     uuid.UUID
	slugA, slugB string
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

	idA, idB := uuid.New(), uuid.New()
	slugA := "tag-a-" + uuid.New().String()[:8]
	slugB := "tag-b-" + uuid.New().String()[:8]

	descA := "alpha description"
	_, err = pool.Exec(ctx,
		`insert into public.tags (id, slug, name, description) values ($1, $2, $3, $4)`,
		idA, slugA, "Alpha", descA,
	)
	require.NoError(t, err)
	// B has nil description to assert pointer behaviour.
	_, err = pool.Exec(ctx,
		`insert into public.tags (id, slug, name) values ($1, $2, $3)`,
		idB, slugB, "Bravo",
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `delete from public.tags where id = any($1)`,
			[]uuid.UUID{idA, idB})
	})

	return &testFixture{
		t: t, pool: pool, ctx: ctx,
		idA: idA, idB: idB, slugA: slugA, slugB: slugB,
	}
}

func TestRepository_List(t *testing.T) {
	f := newFixture(t)
	repo := NewRepository(f.pool)

	out, err := repo.List(f.ctx)
	require.NoError(t, err)

	idxA, idxB := -1, -1
	for i, tag := range out {
		switch tag.Slug {
		case f.slugA:
			idxA = i
			require.NotNil(t, tag.Description)
			assert.Equal(t, "alpha description", *tag.Description)
			assert.Equal(t, "Alpha", tag.Name)
		case f.slugB:
			idxB = i
			assert.Nil(t, tag.Description)
			assert.Equal(t, "Bravo", tag.Name)
		}
	}
	require.NotEqual(t, -1, idxA, "slugA should be in list")
	require.NotEqual(t, -1, idxB, "slugB should be in list")
	assert.Less(t, idxA, idxB, "Alpha should come before Bravo (ordered by name)")
}

func TestRepository_GetBySlug(t *testing.T) {
	f := newFixture(t)
	repo := NewRepository(f.pool)

	t.Run("existing slug", func(t *testing.T) {
		got, err := repo.GetBySlug(f.ctx, f.slugA)
		require.NoError(t, err)
		assert.Equal(t, f.idA, got.ID)
		assert.Equal(t, f.slugA, got.Slug)
		assert.Equal(t, "Alpha", got.Name)
		require.NotNil(t, got.Description)
		assert.Equal(t, "alpha description", *got.Description)
	})

	t.Run("nil description", func(t *testing.T) {
		got, err := repo.GetBySlug(f.ctx, f.slugB)
		require.NoError(t, err)
		assert.Nil(t, got.Description)
	})

	t.Run("missing slug returns ErrNotFound", func(t *testing.T) {
		_, err := repo.GetBySlug(f.ctx, "definitely-does-not-exist-"+uuid.New().String())
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
