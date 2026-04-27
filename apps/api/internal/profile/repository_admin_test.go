package profile

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminRepository_Get_IncludesEmail(t *testing.T) {
	f := newFixture(t)

	_, err := f.pool.Exec(f.ctx,
		`update public.profile set email = 'admin@example.com', full_name = 'Admin' where id = 1`)
	require.NoError(t, err)

	repo := NewAdminRepository(f.pool, "https://example.test/storage/")
	got, err := repo.Get(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got.Email)
	assert.Equal(t, "admin@example.com", *got.Email)
}

func TestAdminRepository_Update_PartialFields(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool, "https://example.test/storage/")

	name := "Quoc Dai Updated"
	tagline := "shipping things"
	got, err := repo.Update(f.ctx, UpdateInput{
		FullName:  &name,
		Tagline:   &tagline,
		UpdatedBy: f.authorID,
	})
	require.NoError(t, err)
	require.NotNil(t, got.FullName)
	assert.Equal(t, name, *got.FullName)
	require.NotNil(t, got.Tagline)
	assert.Equal(t, tagline, *got.Tagline)

	// updated_by persisted.
	var updatedBy *uuid.UUID
	err = f.pool.QueryRow(f.ctx,
		`select updated_by from public.profile where id = 1`).Scan(&updatedBy)
	require.NoError(t, err)
	require.NotNil(t, updatedBy)
	assert.Equal(t, f.authorID, *updatedBy)
}

func TestAdminRepository_Update_AvatarSetAndClear(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool, "https://example.test/storage/")

	// Set avatar.
	avatar := f.mediaID
	avatarPtr := &avatar
	got, err := repo.Update(f.ctx, UpdateInput{
		AvatarID:  &avatarPtr,
		UpdatedBy: f.authorID,
	})
	require.NoError(t, err)
	require.NotNil(t, got.AvatarURL)
	assert.Equal(t, fmt.Sprintf("https://example.test/storage/avatars/%s.jpg", f.mediaID), *got.AvatarURL)

	// Clear avatar (explicit null = pointer-to-pointer where inner is nil).
	var nilUUID *uuid.UUID
	got, err = repo.Update(f.ctx, UpdateInput{
		AvatarID:  &nilUUID,
		UpdatedBy: f.authorID,
	})
	require.NoError(t, err)
	assert.Nil(t, got.AvatarURL)
}

func TestAdminRepository_Update_SocialLinksReplace(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool, "")

	links := map[string]string{"github": "quocdaijr", "x": "qdjr"}
	got, err := repo.Update(f.ctx, UpdateInput{
		SocialLinks: &links,
		UpdatedBy:   f.authorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "quocdaijr", got.SocialLinks["github"])
	assert.Equal(t, "qdjr", got.SocialLinks["x"])

	// Empty map clears.
	empty := map[string]string{}
	got, err = repo.Update(f.ctx, UpdateInput{
		SocialLinks: &empty,
		UpdatedBy:   f.authorID,
	})
	require.NoError(t, err)
	assert.Empty(t, got.SocialLinks)
}
