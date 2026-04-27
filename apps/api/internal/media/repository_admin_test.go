package media

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

type mediaFixture struct {
	t        *testing.T
	pool     *pgxpool.Pool
	ctx      context.Context
	authorID uuid.UUID
}

func newMediaFixture(t *testing.T) *mediaFixture {
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
		authorID, fmt.Sprintf("media-test-%s@example.com", authorID))
	require.NoError(t, err)

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `delete from public.media where uploaded_by = $1`, authorID)
		_, _ = pool.Exec(bg, `delete from auth.users where id = $1`, authorID)
	})
	return &mediaFixture{t: t, pool: pool, ctx: ctx, authorID: authorID}
}

func TestAdminRepository_CreateAndGet(t *testing.T) {
	f := newMediaFixture(t)
	repo := NewAdminRepository(f.pool, "https://stor.example/storage/v1/object/public/")

	alt := "an image"
	w, h := 800, 600
	in := CreateInput{
		Filename:    "thumb.png",
		StoragePath: "media/" + uuid.New().String() + ".png",
		MimeType:    "image/png",
		Size:        4096,
		Width:       &w,
		Height:      &h,
		AltText:     &alt,
		UploadedBy:  f.authorID,
	}
	out, err := repo.Create(f.ctx, in)
	require.NoError(t, err)
	assert.Equal(t, in.Filename, out.Filename)
	assert.Equal(t, in.StoragePath, out.StoragePath)
	assert.Equal(t, in.MimeType, out.MimeType)
	assert.Equal(t, in.Size, out.Size)
	require.NotNil(t, out.UploadedBy)
	assert.Equal(t, f.authorID, *out.UploadedBy)
	assert.Equal(t, "https://stor.example/storage/v1/object/public/"+in.StoragePath, out.URL)

	got, err := repo.Get(f.ctx, out.ID)
	require.NoError(t, err)
	assert.Equal(t, out.ID, got.ID)
	assert.Equal(t, out.URL, got.URL)
}

func TestAdminRepository_Get_NotFound(t *testing.T) {
	f := newMediaFixture(t)
	repo := NewAdminRepository(f.pool, "")
	_, err := repo.Get(f.ctx, uuid.New())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestAdminRepository_List_Paginated(t *testing.T) {
	f := newMediaFixture(t)
	repo := NewAdminRepository(f.pool, "")

	// Seed three rows scoped to authorID.
	for i := 0; i < 3; i++ {
		_, err := repo.Create(f.ctx, CreateInput{
			Filename:    fmt.Sprintf("f%d.png", i),
			StoragePath: "media/" + uuid.New().String() + ".png",
			MimeType:    "image/png",
			Size:        1024,
			UploadedBy:  f.authorID,
		})
		require.NoError(t, err)
	}

	got, total, err := repo.List(f.ctx, AdminListFilter{Page: 1, PerPage: 100})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 3)
	require.NotEmpty(t, got)
}

func TestAdminRepository_Delete(t *testing.T) {
	f := newMediaFixture(t)
	repo := NewAdminRepository(f.pool, "")

	out, err := repo.Create(f.ctx, CreateInput{
		Filename:    "del.png",
		StoragePath: "media/" + uuid.New().String() + ".png",
		MimeType:    "image/png",
		Size:        100,
		UploadedBy:  f.authorID,
	})
	require.NoError(t, err)

	require.NoError(t, repo.Delete(f.ctx, out.ID))
	_, err = repo.Get(f.ctx, out.ID)
	assert.ErrorIs(t, err, ErrNotFound)

	// Idempotent failure when re-deleting.
	err = repo.Delete(f.ctx, out.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}
