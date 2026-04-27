package sitesettings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads the singleton site_settings row from Postgres.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// GetPublic returns only the public-safe columns. contact_email is
// deliberately not selected here — see Public type docs.
func (r *Repository) GetPublic(ctx context.Context) (Public, error) {
	const q = `
        SELECT site_title, site_description, footer_text, social_links
        FROM public.site_settings
        WHERE id = 1`

	var (
		out         Public
		socialBytes []byte
	)
	err := r.pool.QueryRow(ctx, q).Scan(
		&out.SiteTitle, &out.SiteDescription, &out.FooterText, &socialBytes,
	)
	if err != nil {
		return Public{}, fmt.Errorf("site_settings get: %w", err)
	}

	out.SocialLinks = map[string]string{}
	if len(socialBytes) > 0 {
		if err := json.Unmarshal(socialBytes, &out.SocialLinks); err != nil {
			return Public{}, fmt.Errorf("site_settings decode social_links: %w", err)
		}
	}
	return out, nil
}
