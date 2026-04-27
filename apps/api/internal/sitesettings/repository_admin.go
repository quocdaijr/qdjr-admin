package sitesettings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminRepository provides admin read/update for the singleton site_settings row.
type AdminRepository struct {
	pool *pgxpool.Pool
}

// NewAdminRepository constructs an AdminRepository.
func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{pool: pool}
}

// Get returns the singleton site_settings row including contact_email.
func (r *AdminRepository) Get(ctx context.Context) (Admin, error) {
	const q = `
        SELECT id, site_title, site_description, footer_text, contact_email,
               social_links, updated_at
        FROM public.site_settings
        WHERE id = 1`
	var (
		out         Admin
		socialBytes []byte
	)
	err := r.pool.QueryRow(ctx, q).Scan(
		&out.ID, &out.SiteTitle, &out.SiteDescription, &out.FooterText,
		&out.ContactEmail, &socialBytes, &out.UpdatedAt,
	)
	if err != nil {
		return Admin{}, fmt.Errorf("admin site_settings get: %w", err)
	}
	out.SocialLinks = map[string]string{}
	if len(socialBytes) > 0 {
		if err := json.Unmarshal(socialBytes, &out.SocialLinks); err != nil {
			return Admin{}, fmt.Errorf("admin site_settings decode social_links: %w", err)
		}
	}
	return out, nil
}

// Update applies a partial update to the singleton row. Always sets
// updated_by + updated_at.
func (r *AdminRepository) Update(ctx context.Context, in UpdateInput) (Admin, error) {
	sets := make([]string, 0, 8)
	args := make([]any, 0, 8)
	nextArg := 1
	add := func(col, cast string, v any) {
		if cast != "" {
			sets = append(sets, fmt.Sprintf("%s = $%d::%s", col, nextArg, cast))
		} else {
			sets = append(sets, fmt.Sprintf("%s = $%d", col, nextArg))
		}
		args = append(args, v)
		nextArg++
	}
	if in.SiteTitle != nil {
		add("site_title", "", *in.SiteTitle)
	}
	if in.SiteDescription != nil {
		add("site_description", "", *in.SiteDescription)
	}
	if in.FooterText != nil {
		add("footer_text", "", *in.FooterText)
	}
	if in.ContactEmail != nil {
		add("contact_email", "", *in.ContactEmail)
	}
	if in.SocialLinks != nil {
		raw, err := json.Marshal(*in.SocialLinks)
		if err != nil {
			return Admin{}, fmt.Errorf("admin site_settings encode social_links: %w", err)
		}
		add("social_links", "jsonb", string(raw))
	}
	add("updated_by", "", in.UpdatedBy)
	sets = append(sets, "updated_at = now()")

	stmt := fmt.Sprintf(`UPDATE public.site_settings SET %s WHERE id = 1`, strings.Join(sets, ", "))
	if _, err := r.pool.Exec(ctx, stmt, args...); err != nil {
		return Admin{}, fmt.Errorf("admin site_settings update: %w", err)
	}
	return r.Get(ctx)
}
