package profile

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

// fixture connects to TEST_DATABASE_URL and snapshots the singleton profile
// row before each test so mutations can be reverted in cleanup.
type fixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	ctx  context.Context

	authorID uuid.UUID
	mediaID  uuid.UUID
}

func newFixture(t *testing.T) *fixture {
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

	// Snapshot current row so we can restore it after the test.
	var (
		fullName, bio, tagline, location, email *string
		avatarID                                *uuid.UUID
		socialLinks                             []byte
	)
	err = pool.QueryRow(ctx,
		`select full_name, bio, avatar_id, tagline, social_links, location, email
		   from public.profile where id = 1`,
	).Scan(&fullName, &bio, &avatarID, &tagline, &socialLinks, &location, &email)
	require.NoError(t, err)

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg,
			`update public.profile set full_name=$1, bio=$2, avatar_id=$3,
			   tagline=$4, social_links=$5::jsonb, location=$6, email=$7
			 where id = 1`,
			fullName, bio, avatarID, tagline, socialLinks, location, email,
		)
	})

	authorID := uuid.New()
	_, err = pool.Exec(ctx,
		`insert into auth.users (id, email) values ($1, $2)`,
		authorID, fmt.Sprintf("profile-test-%s@example.com", authorID),
	)
	require.NoError(t, err)

	mediaID := uuid.New()
	_, err = pool.Exec(ctx,
		`insert into public.media (id, filename, storage_path, mime_type, size, alt_text, uploaded_by)
		 values ($1, 'avatar.jpg', $2, 'image/jpeg', 1234, 'avatar', $3)`,
		mediaID, fmt.Sprintf("avatars/%s.jpg", mediaID), authorID,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		bg := context.Background()
		// Detach avatar from profile before deleting media (FK is SET NULL,
		// but explicit decoupling makes ordering obvious).
		_, _ = pool.Exec(bg, `update public.profile set avatar_id = null where id = 1`)
		_, _ = pool.Exec(bg, `delete from public.media where id = $1`, mediaID)
		_, _ = pool.Exec(bg, `delete from auth.users where id = $1`, authorID)
	})

	return &fixture{t: t, pool: pool, ctx: ctx, authorID: authorID, mediaID: mediaID}
}

func TestRepository_Get(t *testing.T) {
	f := newFixture(t)

	_, err := f.pool.Exec(f.ctx,
		`update public.profile
		    set full_name = 'Quoc Dai',
		        bio = 'engineer',
		        avatar_id = $1,
		        tagline = 'shipping',
		        social_links = '{"github":"quocdaijr"}'::jsonb,
		        location = 'Hanoi',
		        email = 'me@example.com'
		  where id = 1`, f.mediaID)
	require.NoError(t, err)

	repo := NewRepository(f.pool, "https://example.test/storage/")
	got, err := repo.Get(f.ctx)
	require.NoError(t, err)

	assert.Equal(t, int16(1), got.ID)
	require.NotNil(t, got.FullName)
	assert.Equal(t, "Quoc Dai", *got.FullName)
	require.NotNil(t, got.Bio)
	assert.Equal(t, "engineer", *got.Bio)
	require.NotNil(t, got.AvatarURL)
	assert.Equal(t, fmt.Sprintf("https://example.test/storage/avatars/%s.jpg", f.mediaID), *got.AvatarURL)
	require.NotNil(t, got.Tagline)
	assert.Equal(t, "shipping", *got.Tagline)
	assert.Equal(t, "quocdaijr", got.SocialLinks["github"])
	require.NotNil(t, got.Location)
	assert.Equal(t, "Hanoi", *got.Location)
	require.NotNil(t, got.Email)
	assert.Equal(t, "me@example.com", *got.Email)
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestRepository_Get_NoAvatar(t *testing.T) {
	f := newFixture(t)
	_, err := f.pool.Exec(f.ctx,
		`update public.profile set avatar_id = null, full_name = 'Anon' where id = 1`)
	require.NoError(t, err)

	repo := NewRepository(f.pool, "https://example.test/storage/")
	got, err := repo.Get(f.ctx)
	require.NoError(t, err)
	assert.Nil(t, got.AvatarURL)
	require.NotNil(t, got.FullName)
	assert.Equal(t, "Anon", *got.FullName)
}
