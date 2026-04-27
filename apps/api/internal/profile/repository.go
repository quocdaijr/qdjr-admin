package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads the singleton profile row from Postgres.
//
// urlPrefix is prepended to media.storage_path to produce a public avatar URL.
// When empty the raw storage_path is returned (test convenience).
type Repository struct {
	pool      *pgxpool.Pool
	urlPrefix string
}

// NewRepository constructs a Repository. Pass the Supabase storage public-URL
// prefix as urlPrefix so the API returns directly-loadable avatar URLs.
func NewRepository(pool *pgxpool.Pool, urlPrefix string) *Repository {
	return &Repository{pool: pool, urlPrefix: urlPrefix}
}

// Get returns the singleton profile (id=1). The row is seeded by
// supabase/seed.sql, so this should always succeed in a properly-initialised
// database.
func (r *Repository) Get(ctx context.Context) (Profile, error) {
	const q = `
        SELECT p.id, p.full_name, p.bio, m.storage_path, p.tagline,
               p.social_links, p.location, p.email, p.updated_at
        FROM public.profile p
        LEFT JOIN public.media m ON m.id = p.avatar_id
        WHERE p.id = 1`

	var (
		out         Profile
		avatarPath  *string
		socialBytes []byte
	)
	err := r.pool.QueryRow(ctx, q).Scan(
		&out.ID, &out.FullName, &out.Bio, &avatarPath, &out.Tagline,
		&socialBytes, &out.Location, &out.Email, &out.UpdatedAt,
	)
	if err != nil {
		return Profile{}, fmt.Errorf("profile get: %w", err)
	}

	if avatarPath != nil {
		url := r.avatarURL(*avatarPath)
		out.AvatarURL = &url
	}

	out.SocialLinks = map[string]string{}
	if len(socialBytes) > 0 {
		if err := json.Unmarshal(socialBytes, &out.SocialLinks); err != nil {
			return Profile{}, fmt.Errorf("profile decode social_links: %w", err)
		}
	}
	return out, nil
}

func (r *Repository) avatarURL(storagePath string) string {
	if r.urlPrefix == "" {
		return storagePath
	}
	if strings.HasSuffix(r.urlPrefix, "/") {
		return r.urlPrefix + storagePath
	}
	return r.urlPrefix + "/" + storagePath
}
