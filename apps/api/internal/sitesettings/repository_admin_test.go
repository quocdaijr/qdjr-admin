package sitesettings

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminFixture extends the public fixture with an auth user for updated_by.
type adminFixture struct {
	*fixture
	authorID uuid.UUID
}

func newAdminFixture(t *testing.T) *adminFixture {
	t.Helper()
	f := newFixture(t)

	authorID := uuid.New()
	_, err := f.pool.Exec(f.ctx,
		`insert into auth.users (id, email) values ($1, $2)`,
		authorID, fmt.Sprintf("settings-test-%s@example.com", authorID),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = f.pool.Exec(bg, `update public.site_settings set updated_by = null where id = 1`)
		_, _ = f.pool.Exec(bg, `delete from auth.users where id = $1`, authorID)
	})
	return &adminFixture{fixture: f, authorID: authorID}
}

func TestAdminRepository_Get_IncludesContactEmail(t *testing.T) {
	f := newAdminFixture(t)
	_, err := f.pool.Exec(f.ctx,
		`update public.site_settings set contact_email = 'admin@example.com' where id = 1`)
	require.NoError(t, err)

	repo := NewAdminRepository(f.pool)
	got, err := repo.Get(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got.ContactEmail)
	assert.Equal(t, "admin@example.com", *got.ContactEmail)
}

func TestAdminRepository_Update_PartialFields(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	title := "Updated Title"
	contact := "owner@example.com"
	got, err := repo.Update(f.ctx, UpdateInput{
		SiteTitle:    &title,
		ContactEmail: &contact,
		UpdatedBy:    f.authorID,
	})
	require.NoError(t, err)
	assert.Equal(t, title, got.SiteTitle)
	require.NotNil(t, got.ContactEmail)
	assert.Equal(t, contact, *got.ContactEmail)

	var updatedBy *uuid.UUID
	err = f.pool.QueryRow(f.ctx,
		`select updated_by from public.site_settings where id = 1`).Scan(&updatedBy)
	require.NoError(t, err)
	require.NotNil(t, updatedBy)
	assert.Equal(t, f.authorID, *updatedBy)
}

func TestAdminRepository_Update_SocialLinksReplace(t *testing.T) {
	f := newAdminFixture(t)
	repo := NewAdminRepository(f.pool)

	links := map[string]string{"github": "qdjr"}
	got, err := repo.Update(f.ctx, UpdateInput{
		SocialLinks: &links,
		UpdatedBy:   f.authorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "qdjr", got.SocialLinks["github"])

	empty := map[string]string{}
	got, err = repo.Update(f.ctx, UpdateInput{
		SocialLinks: &empty,
		UpdatedBy:   f.authorID,
	})
	require.NoError(t, err)
	assert.Empty(t, got.SocialLinks)
}
