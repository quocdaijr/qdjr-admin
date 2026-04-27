package sitesettings

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture connects to TEST_DATABASE_URL and snapshots the singleton row so
// the test can mutate it freely and revert in cleanup.
type fixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	ctx  context.Context
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

	var (
		title                                  string
		description, footer, contactEmail      *string
		socialLinks                            []byte
	)
	err = pool.QueryRow(ctx,
		`select site_title, site_description, footer_text, contact_email, social_links
		   from public.site_settings where id = 1`,
	).Scan(&title, &description, &footer, &contactEmail, &socialLinks)
	require.NoError(t, err)

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg,
			`update public.site_settings
			    set site_title = $1, site_description = $2, footer_text = $3,
			        contact_email = $4, social_links = $5::jsonb
			  where id = 1`,
			title, description, footer, contactEmail, socialLinks,
		)
	})

	return &fixture{t: t, pool: pool, ctx: ctx}
}

func TestRepository_GetPublic(t *testing.T) {
	f := newFixture(t)

	_, err := f.pool.Exec(f.ctx,
		`update public.site_settings
		    set site_title = 'qdjr.me',
		        site_description = 'personal site',
		        footer_text = '(c) 2026',
		        contact_email = 'admin@example.com',
		        social_links = '{"github":"quocdaijr"}'::jsonb
		  where id = 1`)
	require.NoError(t, err)

	repo := NewRepository(f.pool)
	got, err := repo.GetPublic(f.ctx)
	require.NoError(t, err)

	assert.Equal(t, "qdjr.me", got.SiteTitle)
	require.NotNil(t, got.SiteDescription)
	assert.Equal(t, "personal site", *got.SiteDescription)
	require.NotNil(t, got.FooterText)
	assert.Equal(t, "(c) 2026", *got.FooterText)
	assert.Equal(t, "quocdaijr", got.SocialLinks["github"])
}
